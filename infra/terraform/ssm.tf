resource "aws_ssm_parameter" "database_url" {
  name  = "/autodream/database_url"
  type  = "SecureString"
  value = var.database_url
}

resource "aws_ssm_parameter" "redis_url" {
  name  = "/autodream/redis_url"
  type  = "SecureString"
  value = var.redis_url
}

resource "aws_ssm_parameter" "jwt_secret" {
  name  = "/autodream/jwt_secret"
  type  = "SecureString"
  value = var.jwt_secret
}

resource "aws_ssm_parameter" "encryption_key" {
  name  = "/autodream/encryption_key"
  type  = "SecureString"
  value = var.encryption_key
}

resource "aws_ssm_parameter" "ses_from_email" {
  name  = "/autodream/ses_from_email"
  type  = "String"
  value = var.ses_from_email != "" ? var.ses_from_email : "disabled"
}

resource "aws_ssm_parameter" "dashboard_url" {
  name  = "/autodream/dashboard_url"
  type  = "String"
  value = var.dashboard_url
}

resource "aws_ssm_parameter" "ai_service_url" {
  name  = "/autodream/ai_service_url"
  type  = "String"
  value = "http://${aws_eip.browser_pool.public_ip}:8000"
}

resource "aws_ssm_parameter" "s3_bucket_resumes" {
  name  = "/autodream/s3_bucket_resumes"
  type  = "String"
  value = var.s3_bucket_resumes
}

resource "aws_ssm_parameter" "s3_bucket_screenshots" {
  name  = "/autodream/s3_bucket_screenshots"
  type  = "String"
  value = var.s3_bucket_screenshots
}

resource "aws_ssm_parameter" "gemini_api_key" {
  name  = "/autodream/gemini_api_key"
  type  = "SecureString"
  value = var.gemini_api_key != "" ? var.gemini_api_key : "disabled"
}

resource "aws_ssm_parameter" "ecr_registry" {
  name  = "/autodream/ecr_registry"
  type  = "String"
  value = "${data.aws_caller_identity.current.account_id}.dkr.ecr.${var.aws_region}.amazonaws.com"
}

resource "aws_ssm_parameter" "image_tag" {
  name  = "/autodream/image_tag"
  type  = "String"
  value = "latest"

  lifecycle {
    ignore_changes = [value] # Managed by CI/CD
  }
}

# ── CI/CD discovery parameters (written by Terraform, read by update pipeline) ─

resource "aws_ssm_parameter" "ec2_public_ip" {
  name  = "/autodream/ec2_public_ip"
  type  = "String"
  value = aws_eip.browser_pool.public_ip

  lifecycle {
    ignore_changes = [] # Always updated by Terraform
  }
}

resource "aws_ssm_parameter" "api_endpoint" {
  name  = "/autodream/api_endpoint"
  type  = "String"
  value = aws_apigatewayv2_stage.default.invoke_url
}
