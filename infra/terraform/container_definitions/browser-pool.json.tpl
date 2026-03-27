[
  {
    "name": "browser-pool",
    "image": "${image_url}",
    "essential": true,
    "portMappings": [
      {
        "containerPort": ${port},
        "hostPort": ${port},
        "protocol": "tcp"
      }
    ],
    "environment": [
      { "name": "BROWSER_POOL_PORT", "value": "${port}" },
      { "name": "ENVIRONMENT",       "value": "production" },
      { "name": "LOG_LEVEL",         "value": "info" },
      { "name": "AWS_REGION",        "value": "${region}" },
      { "name": "LOG_FORMAT",        "value": "json" },
      { "name": "CHROMIUM_FLAGS",    "value": "--no-sandbox --disable-dev-shm-usage --disable-gpu --headless=new" }
    ],
    "secrets": [
      {
        "name": "REDIS_URL",
        "valueFrom": "${ssm_prefix}/redis_url"
      },
      {
        "name": "S3_BUCKET",
        "valueFrom": "${ssm_prefix}/s3_bucket"
      }
    ],
    "linuxParameters": {
      "tmpfs": [
        {
          "containerPath": "/dev/shm",
          "size": ${shm_size},
          "mountOptions": ["rw", "nosuid", "nodev", "exec"]
        }
      ]
    },
    "logConfiguration": {
      "logDriver": "awslogs",
      "options": {
        "awslogs-group":         "${log_group}",
        "awslogs-region":        "${region}",
        "awslogs-stream-prefix": "browser-pool"
      }
    },
    "healthCheck": {
      "command":     ["CMD-SHELL", "wget -qO- http://localhost:${port}/health || exit 1"],
      "interval":    30,
      "timeout":     10,
      "retries":     3,
      "startPeriod": 60
    },
    "ulimits": [
      {
        "name": "nofile",
        "softLimit": 65536,
        "hardLimit": 65536
      }
    ],
    "readonlyRootFilesystem": false,
    "stopTimeout": 120
  }
]
