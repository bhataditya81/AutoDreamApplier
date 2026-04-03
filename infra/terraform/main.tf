terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  # GitLab-managed HTTP state backend.
  # In CI the init flags are injected by .gitlab/ci/terraform.yml.
  # For local dev, pass -backend-config flags or set TF_HTTP_* env vars.
  # See: https://docs.gitlab.com/ee/user/infrastructure/iac/terraform_state.html
  backend "http" {}
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "AutoDreamApplier"
      Environment = var.environment
      ManagedBy   = "Terraform"
    }
  }
}

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}
