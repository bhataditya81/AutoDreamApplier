# Packer template — builds a pre-baked AL2023 ARM64 AMI for autodream-browser-pool.
# Pre-installs Docker, docker-compose, and aws-cli so user_data is trivial
# (just SSM agent start + script setup), cutting boot-to-SSM-ready to < 60 s.
#
# Usage:
#   packer init .
#   AWS_REGION=us-east-1 packer build autodream.pkr.hcl
#
# After build, update data "aws_ami" in ec2.tf to use the output AMI ID,
# or set SSM parameter /autodream/ec2_ami_id and read it in Terraform.

packer {
  required_plugins {
    amazon = {
      version = ">= 1.3.0"
      source  = "github.com/hashicorp/amazon"
    }
  }
}

variable "aws_region" {
  type    = string
  default = "us-east-1"
}

variable "instance_type" {
  type    = string
  default = "t4g.small" # Use small for the build (nano is too slow for dnf update)
}

# Use the latest official AL2023 STANDARD ARM64 AMI as the base.
# "al2023-ami-2023*" matches standard only — excludes "al2023-ami-minimal-*"
# which lacks amazon-ssm-agent and would cause SSM registration failures.
data "amazon-ami" "al2023_arm64" {
  region = var.aws_region
  owners = ["amazon"]
  filters = {
    name                = "al2023-ami-2023*-kernel-*-arm64"
    root-device-type    = "ebs"
    virtualization-type = "hvm"
  }
  most_recent = true
}

source "amazon-ebs" "autodream" {
  region        = var.aws_region
  instance_type = var.instance_type
  source_ami    = data.amazon-ami.al2023_arm64.id

  ami_name        = "autodream-browser-pool-{{timestamp}}"
  ami_description = "AutoDreamApplier browser-pool — Docker + docker-compose + aws-cli pre-installed"

  # ARM64
  ami_virtualization_type = "hvm"
  ena_support             = true

  launch_block_device_mappings {
    device_name           = "/dev/xvda"
    volume_size           = 20
    volume_type           = "gp3"
    delete_on_termination = true
  }

  # Use default VPC for the build instance (no special networking needed)
  associate_public_ip_address = true

  ssh_username = "ec2-user"

  tags = {
    Name        = "autodream-browser-pool-base"
    Project     = "autodream"
    ManagedBy   = "packer"
    BuildDate   = "{{timestamp}}"
  }
}

build {
  name    = "autodream-base-ami"
  sources = ["source.amazon-ebs.autodream"]

  # Full system update (run once at AMI build time, not on every boot)
  provisioner "shell" {
    inline = [
      "sudo dnf update -y --exclude='amazon-ssm-agent*'",
      "sudo dnf upgrade -y amazon-ssm-agent",
      "sudo systemctl enable amazon-ssm-agent",
    ]
  }

  # Install Docker
  provisioner "shell" {
    inline = [
      "sudo dnf install -y docker",
      "sudo systemctl enable docker",
      # Don't start docker — it starts on boot via systemd
      "sudo usermod -aG docker ec2-user",
    ]
  }

  # Docker Compose plugin (aarch64)
  provisioner "shell" {
    inline = [
      "sudo mkdir -p /usr/local/lib/docker/cli-plugins",
      "sudo curl -SL https://github.com/docker/compose/releases/latest/download/docker-compose-linux-aarch64 -o /usr/local/lib/docker/cli-plugins/docker-compose",
      "sudo chmod +x /usr/local/lib/docker/cli-plugins/docker-compose",
    ]
  }

  # AWS CLI
  provisioner "shell" {
    inline = [
      "sudo dnf install -y aws-cli",
    ]
  }

  # Nightly security cron
  provisioner "shell" {
    inline = [
      "echo '0 3 * * * root dnf upgrade -y --security >> /var/log/dnf-security-cron.log 2>&1' | sudo tee /etc/cron.d/autodream-security-updates",
    ]
  }

  # Clean up to reduce AMI size
  provisioner "shell" {
    inline = [
      "sudo dnf clean all",
      "sudo rm -rf /var/cache/dnf /tmp/*",
      # Remove SSH host keys (regenerated on first boot)
      "sudo shred -u /etc/ssh/*_key /etc/ssh/*_key.pub 2>/dev/null || true",
    ]
  }

  # Record the AMI source in SSM after build
  post-processor "manifest" {
    output     = "packer-manifest.json"
    strip_path = true
  }
}
