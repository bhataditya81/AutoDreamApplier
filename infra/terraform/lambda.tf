locals {
  lambda_env_vars = {
    # Connection strings — config.go parses these into individual components
    DATABASE_URL = var.database_url
    REDIS_URL    = var.redis_url

    # App
    APP_ENV       = var.app_env
    DEV_JWT_SECRET = var.dev_jwt_secret   # used when APP_ENV != production
    ENCRYPTION_KEY = var.encryption_key

    # Cognito (empty string skips Cognito in dev/staging)
    COGNITO_USER_POOL_ID  = var.cognito_user_pool_id
    COGNITO_APP_CLIENT_ID = var.cognito_app_client_id
    COGNITO_DOMAIN        = var.cognito_domain

    # S3
    S3_BUCKET_RESUMES     = var.s3_bucket_resumes
    S3_BUCKET_SCREENSHOTS = var.s3_bucket_screenshots

    # Notifications (empty string disables SES)
    SES_FROM_EMAIL = var.ses_from_email
    DASHBOARD_URL  = var.dashboard_url

    # AI / LLM
    AI_SERVICE_URL    = "http://${aws_eip.browser_pool.public_ip}:8000"
    ANTHROPIC_API_KEY = var.anthropic_api_key
    GEMINI_API_KEY    = var.gemini_api_key
    AI_PROVIDER       = var.ai_provider
    LLM_MODEL         = var.llm_model
    GEMINI_MODEL      = var.llm_model

    # Browser pool — auto-derived from the Elastic IP Terraform manages
    BROWSER_POOL_URL = "http://${aws_eip.browser_pool.public_ip}:9222"
  }
}

# ── API Gateway Lambda ─────────────────────────────────────────────────────

resource "aws_lambda_function" "api_gateway" {
  function_name = "${var.project_name}-api-gateway"
  role          = aws_iam_role.lambda_exec.arn
  package_type  = "Image"
  image_uri     = "${aws_ecr_repository.services["api-gateway"].repository_url}:latest"
  architectures = ["x86_64"]
  memory_size   = 256
  timeout       = 30

  environment {
    variables = local.lambda_env_vars
  }
}

resource "aws_cloudwatch_log_group" "api_gateway" {
  name              = "/aws/lambda/${aws_lambda_function.api_gateway.function_name}"
  retention_in_days = 3
}

# ── Job Discovery Lambda ───────────────────────────────────────────────────

resource "aws_lambda_function" "job_discovery" {
  function_name = "${var.project_name}-job-discovery"
  role          = aws_iam_role.lambda_exec.arn
  package_type  = "Image"
  image_uri     = "${aws_ecr_repository.services["job-discovery"].repository_url}:latest"
  architectures = ["x86_64"]
  memory_size   = 256
  timeout       = 840 # 14 minutes — scraping can take a while

  environment {
    variables = merge(local.lambda_env_vars, {
      LINKEDIN_PROXY_URL = var.linkedin_proxy_url
    })
  }
}

resource "aws_cloudwatch_log_group" "job_discovery" {
  name              = "/aws/lambda/${aws_lambda_function.job_discovery.function_name}"
  retention_in_days = 3
}

# ── Job Matcher Lambda ─────────────────────────────────────────────────────

resource "aws_lambda_function" "job_matcher" {
  function_name = "${var.project_name}-job-matcher"
  role          = aws_iam_role.lambda_exec.arn
  package_type  = "Image"
  image_uri     = "${aws_ecr_repository.services["job-matcher"].repository_url}:latest"
  architectures = ["x86_64"]
  memory_size   = 256
  timeout       = 300 # 5 minutes

  environment {
    variables = local.lambda_env_vars
  }
}

resource "aws_cloudwatch_log_group" "job_matcher" {
  name              = "/aws/lambda/${aws_lambda_function.job_matcher.function_name}"
  retention_in_days = 3
}

# ── Follow-Up Scheduler Lambda ─────────────────────────────────────────────

resource "aws_lambda_function" "followup_scheduler" {
  function_name = "${var.project_name}-followup-scheduler"
  role          = aws_iam_role.lambda_exec.arn
  package_type  = "Image"
  image_uri     = "${aws_ecr_repository.services["followup-scheduler"].repository_url}:latest"
  architectures = ["x86_64"]
  memory_size   = 128
  timeout       = 120

  environment {
    variables = local.lambda_env_vars
  }
}

resource "aws_cloudwatch_log_group" "followup_scheduler" {
  name              = "/aws/lambda/${aws_lambda_function.followup_scheduler.function_name}"
  retention_in_days = 3
}
