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
