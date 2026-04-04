# Import pre-existing AWS resources into Terraform state.
# These existed before TF state was migrated to S3 (or were created by a
# partially-failed apply). Safe to remove these blocks after the first
# successful terraform apply — they are single-use.

# ── CloudWatch ───────────────────────────────────────────────────────────────
import {
  to = aws_cloudwatch_log_group.api_gateway_access
  id = "/aws/apigateway/autodream"
}

# ── IAM ──────────────────────────────────────────────────────────────────────
import {
  to = aws_iam_role.lambda_exec
  id = "autodream-lambda-exec"
}

import {
  to = aws_iam_role.ec2_instance
  id = "autodream-ec2-instance"
}

import {
  to = aws_iam_instance_profile.ec2
  id = "autodream-ec2-profile"
}

import {
  to = aws_iam_role_policy_attachment.lambda_basic
  id = "autodream-lambda-exec/arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# ── S3 ───────────────────────────────────────────────────────────────────────
import {
  to = aws_s3_bucket.resumes
  id = "autodream-resumes-prod"
}

import {
  to = aws_s3_bucket.screenshots
  id = "autodream-screenshots-prod"
}

# ── EC2 ──────────────────────────────────────────────────────────────────────
import {
  to = aws_security_group.browser_pool_ec2
  id = "sg-0c81758640b9d532a"
}

import {
  to = aws_instance.browser_pool
  id = "i-0b3cc0b9f33f2a4de"
}

import {
  to = aws_eip.browser_pool
  id = "eipalloc-0556bd532ca4b1fb8"
}

# ── SSM Parameters (created by partial apply, not yet in state) ───────────────
import {
  to = aws_ssm_parameter.database_url
  id = "/autodream/database_url"
}

import {
  to = aws_ssm_parameter.redis_url
  id = "/autodream/redis_url"
}

import {
  to = aws_ssm_parameter.jwt_secret
  id = "/autodream/jwt_secret"
}

import {
  to = aws_ssm_parameter.encryption_key
  id = "/autodream/encryption_key"
}

import {
  to = aws_ssm_parameter.ses_from_email
  id = "/autodream/ses_from_email"
}

import {
  to = aws_ssm_parameter.dashboard_url
  id = "/autodream/dashboard_url"
}

import {
  to = aws_ssm_parameter.ai_service_url
  id = "/autodream/ai_service_url"
}

import {
  to = aws_ssm_parameter.s3_bucket_resumes
  id = "/autodream/s3_bucket_resumes"
}

import {
  to = aws_ssm_parameter.s3_bucket_screenshots
  id = "/autodream/s3_bucket_screenshots"
}

import {
  to = aws_ssm_parameter.ecr_registry
  id = "/autodream/ecr_registry"
}

import {
  to = aws_ssm_parameter.image_tag
  id = "/autodream/image_tag"
}

import {
  to = aws_ssm_parameter.ec2_public_ip
  id = "/autodream/ec2_public_ip"
}

import {
  to = aws_ssm_parameter.api_endpoint
  id = "/autodream/api_endpoint"
}
