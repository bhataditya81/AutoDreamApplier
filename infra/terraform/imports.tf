# Import pre-existing AWS resources into Terraform state.
# Resources are removed from this file once they appear in TF state.
# Safe to delete this entire file after the first fully-successful apply.

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
