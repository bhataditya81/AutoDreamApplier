# ── SSH Key Pair (auto-generated, private key stored in Secrets Manager) ───

resource "tls_private_key" "ec2" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "aws_key_pair" "ec2" {
  key_name   = "${var.project_name}-ec2"
  public_key = tls_private_key.ec2.public_key_openssh
}

resource "aws_secretsmanager_secret" "ec2_ssh_key" {
  name                    = "/${var.project_name}/ec2_ssh_private_key"
  description             = "EC2 SSH private key (RSA) — used by CI/CD pipelines"
  recovery_window_in_days = 0 # allow immediate deletion on destroy
}

resource "aws_secretsmanager_secret_version" "ec2_ssh_key" {
  secret_id     = aws_secretsmanager_secret.ec2_ssh_key.id
  secret_string = tls_private_key.ec2.private_key_pem
}

# ── Data sources ───────────────────────────────────────────────────────────

# Latest Amazon Linux 2023 ARM64 AMI
data "aws_ami" "amazon_linux_2023_arm64" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-*-arm64"]
  }

  filter {
    name   = "architecture"
    values = ["arm64"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

# Default VPC (Lambda runs without VPC; EC2 uses default VPC public subnet)
data "aws_vpc" "default" {
  default = true
}

data "aws_subnets" "default_public" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

# ── Security Group ─────────────────────────────────────────────────────────

resource "aws_security_group" "browser_pool_ec2" {
  name        = "${var.project_name}-browser-pool"
  description = "Security group for apply-engine + browser-pool EC2"
  vpc_id      = data.aws_vpc.default.id

  # SSH access from admin IP only
  ingress {
    description = "SSH from admin"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [var.admin_ssh_cidr]
  }

  # Apply-engine API (emergency stop) — restrict to known IPs in production
  # TODO: Narrow this CIDR after setup, or use API key authentication
  ingress {
    description = "Apply engine API"
    from_port   = 8084
    to_port     = 8084
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # All outbound (to reach Neon DB, Upstash Redis, ATS portals, S3, ECR)
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# ── EC2 Instance ───────────────────────────────────────────────────────────

resource "aws_instance" "browser_pool" {
  ami                    = data.aws_ami.amazon_linux_2023_arm64.id
  instance_type          = "t4g.nano"
  key_name               = aws_key_pair.ec2.key_name
  vpc_security_group_ids = [aws_security_group.browser_pool_ec2.id]
  subnet_id              = tolist(data.aws_subnets.default_public.ids)[0]
  iam_instance_profile   = aws_iam_instance_profile.ec2.name

  # Auto-assign public IP (no NAT needed)
  associate_public_ip_address = true

  root_block_device {
    volume_type = "gp3"
    volume_size = 30 # GB — AL2023 ARM64 AMI requires ≥ 30GB
    encrypted   = true
  }

  user_data = base64encode(file("${path.module}/../../deployments/ec2/setup.sh"))

  tags = {
    Name = "${var.project_name}-browser-pool"
  }

  lifecycle {
    # Don't replace instance on AMI update — use deploy.sh instead.
    # subnet_id: tolist() ordering is non-deterministic; ignore to prevent
    # accidental replacement when the instance is imported into state.
    ignore_changes = [ami, user_data, subnet_id]
  }
}

# Elastic IP — stable address for Lambda env vars and SSH
resource "aws_eip" "browser_pool" {
  instance = aws_instance.browser_pool.id
  domain   = "vpc"
}

output "ec2_public_ip" {
  description = "Elastic IP of the browser pool EC2 instance"
  value       = aws_eip.browser_pool.public_ip
}

output "ec2_ssh_command" {
  description = "SSH command to access the EC2 instance"
  value       = "ssh -i <(aws secretsmanager get-secret-value --secret-id /${var.project_name}/ec2_ssh_private_key --query SecretString --output text) ec2-user@${aws_eip.browser_pool.public_ip}"
}

output "ec2_ssh_secret_name" {
  description = "Secrets Manager secret name holding the EC2 private key"
  value       = aws_secretsmanager_secret.ec2_ssh_key.name
}
