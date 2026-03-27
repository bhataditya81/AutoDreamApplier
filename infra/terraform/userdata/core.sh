#!/bin/bash
# =============================================================================
# AutoDreamApplier — Core EC2 Bootstrap Script
# Runs once at first boot (user_data). Idempotent: re-run safe.
#
# This script:
#   1. Updates the system and installs dependencies
#   2. Mounts and formats the gp3 data EBS volume
#   3. Installs Docker CE + Docker Compose v2
#   4. Installs golang-migrate
#   5. Installs CloudWatch agent
#   6. Authenticates with ECR
#   7. Writes docker-compose.core.yml
#   8. Registers and starts the autodream-core systemd service
#   9. Runs database migrations
# =============================================================================

set -euo pipefail
exec > >(tee /var/log/autodream-bootstrap.log | logger -t autodream-bootstrap) 2>&1

echo "=== AutoDreamApplier bootstrap starting at $(date) ==="

# ── Variables (injected by Terraform templatefile) ────────────────────────────
AWS_REGION="${aws_region}"
APP_NAME="${app_name}"
ENVIRONMENT="${environment}"
SSM_PREFIX="${ssm_prefix}"
ECR_BASE="${ecr_base}"
API_GATEWAY_IMAGE="${api_gateway_image}"
DATA_DEVICE="${data_device}"
DATA_MOUNT="${data_mount}"
APP_DIR="/opt/autodream"
COMPOSE_FILE="$APP_DIR/docker-compose.core.yml"
MIGRATE_VERSION="4.17.1"
CLOUDWATCH_AGENT_VERSION="latest"

# ── 1. System update ──────────────────────────────────────────────────────────
echo "--- Updating system packages ---"
dnf update -y --quiet
dnf install -y --quiet \
    curl wget git unzip jq \
    python3 python3-pip \
    htop tmux vim

# ── 2. Mount and format the data EBS volume ───────────────────────────────────
echo "--- Setting up data volume $DATA_DEVICE → $DATA_MOUNT ---"
mkdir -p "$DATA_MOUNT"

# Only format if the device has no filesystem yet
if ! blkid "$DATA_DEVICE" > /dev/null 2>&1; then
    echo "Formatting $DATA_DEVICE as ext4..."
    mkfs.ext4 -L autodream-data "$DATA_DEVICE"
fi

# Mount persistently
if ! grep -q "$DATA_MOUNT" /etc/fstab; then
    echo "LABEL=autodream-data  $DATA_MOUNT  ext4  defaults,nofail  0  2" >> /etc/fstab
fi
mount -a

# Create subdirectories on the data volume
mkdir -p \
    "$DATA_MOUNT/postgres" \
    "$DATA_MOUNT/redis" \
    "$DATA_MOUNT/migrations"

# Set permissions so Docker containers can write to these directories
chmod 755 "$DATA_MOUNT/postgres" "$DATA_MOUNT/redis"

# ── 3. Install Docker CE ──────────────────────────────────────────────────────
echo "--- Installing Docker CE ---"
if ! command -v docker > /dev/null 2>&1; then
    dnf install -y --quiet docker
    systemctl enable docker
    systemctl start docker
    usermod -aG docker ec2-user
fi

# ── 4. Install Docker Compose v2 ──────────────────────────────────────────────
echo "--- Installing Docker Compose v2 ---"
DOCKER_CONFIG=/usr/local/lib/docker
mkdir -p "$DOCKER_CONFIG/cli-plugins"
if [ ! -f "$DOCKER_CONFIG/cli-plugins/docker-compose" ]; then
    COMPOSE_URL="https://github.com/docker/compose/releases/latest/download/docker-compose-linux-x86_64"
    curl -sSL "$COMPOSE_URL" -o "$DOCKER_CONFIG/cli-plugins/docker-compose"
    chmod +x "$DOCKER_CONFIG/cli-plugins/docker-compose"
fi
# Also symlink to /usr/local/bin for convenience
ln -sf "$DOCKER_CONFIG/cli-plugins/docker-compose" /usr/local/bin/docker-compose || true

# ── 5. Install golang-migrate ─────────────────────────────────────────────────
echo "--- Installing golang-migrate v$MIGRATE_VERSION ---"
if ! command -v migrate > /dev/null 2>&1; then
    MIGRATE_URL="https://github.com/golang-migrate/migrate/releases/download/v$MIGRATE_VERSION/migrate.linux-amd64.tar.gz"
    curl -sSL "$MIGRATE_URL" -o /tmp/migrate.tar.gz
    tar -xzf /tmp/migrate.tar.gz -C /usr/local/bin migrate
    chmod +x /usr/local/bin/migrate
    rm /tmp/migrate.tar.gz
fi

# ── 6. Install CloudWatch Agent ───────────────────────────────────────────────
echo "--- Installing CloudWatch Agent ---"
if ! command -v amazon-cloudwatch-agent-ctl > /dev/null 2>&1; then
    dnf install -y --quiet amazon-cloudwatch-agent
fi

# Write CloudWatch agent config for memory + disk metrics
cat > /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json << 'CWCONFIG'
{
  "metrics": {
    "append_dimensions": {
      "InstanceId": "$${aws:InstanceId}"
    },
    "metrics_collected": {
      "mem": {
        "measurement": ["mem_used_percent"],
        "metrics_collection_interval": 60
      },
      "disk": {
        "measurement": ["disk_used_percent"],
        "metrics_collection_interval": 60,
        "resources": ["/", "/opt/autodream/data"]
      }
    }
  },
  "logs": {
    "logs_collected": {
      "files": {
        "collect_list": [
          {
            "file_path": "/var/log/autodream-bootstrap.log",
            "log_group_name": "/ec2/autodreamapplier-production/core",
            "log_stream_name": "bootstrap-{instance_id}"
          }
        ]
      }
    }
  }
}
CWCONFIG

/opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
    -a fetch-config -m ec2 -s \
    -c file:/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json || true

# ── 7. Create app directory ───────────────────────────────────────────────────
echo "--- Creating app directory $APP_DIR ---"
mkdir -p "$APP_DIR"
mkdir -p "$APP_DIR/migrations"

# ── 8. Fetch secrets from SSM ─────────────────────────────────────────────────
echo "--- Fetching secrets from SSM ---"

get_ssm() {
    aws ssm get-parameter \
        --name "$SSM_PREFIX/$1" \
        --with-decryption \
        --query "Parameter.Value" \
        --output text \
        --region "$AWS_REGION" 2>/dev/null || echo ""
}

DB_PASSWORD="$(get_ssm db_password)"
JWT_SECRET="$(get_ssm jwt_secret)"
REDIS_URL_VALUE="redis://127.0.0.1:6379/0"  # local Redis on same instance

# ── 9. Authenticate with ECR ──────────────────────────────────────────────────
echo "--- Authenticating with ECR ---"
aws ecr get-login-password --region "$AWS_REGION" | \
    docker login --username AWS --password-stdin "$ECR_BASE"

# ── 10. Write docker-compose.core.yml ─────────────────────────────────────────
echo "--- Writing $COMPOSE_FILE ---"
cat > "$COMPOSE_FILE" << COMPOSE
services:

  # ─── PostgreSQL 16 with pgvector ─────────────────────────────────────────
  postgres:
    image: pgvector/pgvector:pg16
    container_name: autodream-postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: autodream
      POSTGRES_PASSWORD: "$DB_PASSWORD"
      POSTGRES_DB: autodreamapplier
      PGDATA: /var/lib/postgresql/data/pgdata
    volumes:
      - $DATA_MOUNT/postgres:/var/lib/postgresql/data
    ports:
      - "127.0.0.1:5432:5432"   # bind to loopback only — ECS reaches via private IP
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U autodream -d autodreamapplier"]
      interval: 10s
      timeout: 5s
      retries: 5
    logging:
      driver: awslogs
      options:
        awslogs-group: /ec2/${app_name}-${environment}/core
        awslogs-region: $AWS_REGION
        awslogs-stream: postgres

  # ─── Redis 7 (Asynq queue backend) ───────────────────────────────────────
  redis:
    image: redis:7-alpine
    container_name: autodream-redis
    restart: unless-stopped
    command: >
      redis-server
      --save 60 1
      --save 300 10
      --appendonly yes
      --appendfsync everysec
      --maxmemory 256mb
      --maxmemory-policy allkeys-lru
    volumes:
      - $DATA_MOUNT/redis:/data
    ports:
      - "0.0.0.0:6379:6379"    # ECS tasks in private subnet connect here
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5
    logging:
      driver: awslogs
      options:
        awslogs-group: /ec2/${app_name}-${environment}/core
        awslogs-region: $AWS_REGION
        awslogs-stream: redis

  # ─── api-gateway Go binary ────────────────────────────────────────────────
  api-gateway:
    image: $API_GATEWAY_IMAGE
    container_name: autodream-api-gateway
    restart: unless-stopped
    environment:
      - PORT=8080
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_USER=autodream
      - DB_PASSWORD=$DB_PASSWORD
      - DB_NAME=autodreamapplier
      - DB_SSLMODE=disable
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - JWT_SECRET=$JWT_SECRET
      - AWS_REGION=$AWS_REGION
      - S3_BUCKET=$(aws ssm get-parameter --name "$SSM_PREFIX/s3_bucket" --query "Parameter.Value" --output text --region "$AWS_REGION" 2>/dev/null || echo "")
      - AI_SERVICE_URL=http://ai-service:8081
      - ENVIRONMENT=production
      - LOG_FORMAT=json
    ports:
      - "0.0.0.0:8080:8080"
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/health"]
      interval: 15s
      timeout: 5s
      retries: 3
      start_period: 15s
    logging:
      driver: awslogs
      options:
        awslogs-group: /ec2/${app_name}-${environment}/api-gateway
        awslogs-region: $AWS_REGION
        awslogs-stream: api-gateway

COMPOSE

# ── 11. Write systemd service ─────────────────────────────────────────────────
echo "--- Writing systemd service ---"
cat > /etc/systemd/system/autodream-core.service << SYSTEMD
[Unit]
Description=AutoDreamApplier Core Services (PostgreSQL, Redis, api-gateway)
After=docker.service network-online.target
Requires=docker.service
Wants=network-online.target

[Service]
Type=simple
Restart=on-failure
RestartSec=30s
WorkingDirectory=$APP_DIR
ExecStartPre=/bin/bash -c 'aws ecr get-login-password --region $AWS_REGION | docker login --username AWS --password-stdin $ECR_BASE'
ExecStart=/usr/local/bin/docker-compose -f $COMPOSE_FILE up
ExecStop=/usr/local/bin/docker-compose -f $COMPOSE_FILE down
TimeoutStartSec=120
TimeoutStopSec=60

[Install]
WantedBy=multi-user.target
SYSTEMD

systemctl daemon-reload
systemctl enable autodream-core.service
systemctl start autodream-core.service

# ── 12. Wait for PostgreSQL then run migrations ───────────────────────────────
echo "--- Waiting for PostgreSQL to be ready ---"
MAX_WAIT=120
ELAPSED=0
until docker exec autodream-postgres pg_isready -U autodream -d autodreamapplier 2>/dev/null; do
    if [ $ELAPSED -ge $MAX_WAIT ]; then
        echo "ERROR: PostgreSQL did not become ready within $MAX_WAIT seconds"
        exit 1
    fi
    echo "Waiting for PostgreSQL... ($ELAPSED s)"
    sleep 5
    ELAPSED=$((ELAPSED + 5))
done

echo "--- Running database migrations ---"
DB_PASSWORD_SAFE=$(python3 -c "import urllib.parse, sys; print(urllib.parse.quote(sys.argv[1]))" "$DB_PASSWORD")
DATABASE_URL="postgres://autodream:$DB_PASSWORD_SAFE@127.0.0.1:5432/autodreamapplier?sslmode=disable"

# Migrations are baked into the Docker image or copied separately.
# The bootstrap.sh script copies them after first apply.
if ls "$APP_DIR/migrations"/*.sql > /dev/null 2>&1; then
    migrate -path "$APP_DIR/migrations" -database "$DATABASE_URL" up
    echo "Migrations applied successfully"
else
    echo "WARN: No migrations found in $APP_DIR/migrations. Run bootstrap.sh migrate after deploy."
fi

echo "=== AutoDreamApplier bootstrap complete at $(date) ==="
