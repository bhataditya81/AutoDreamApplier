# Cognito User Pool — only created when app_env=production and no existing pool is provided.
# For staging/MVP: skip Cognito entirely — dev JWT auth (HS256) works fine.
# To use an existing Cognito pool, set cognito_user_pool_id in terraform.tfvars
# and set create_cognito=false.

variable "create_cognito" {
  description = "Create a new Cognito User Pool. Set false if using an existing pool."
  type        = bool
  default     = false
}

resource "aws_cognito_user_pool" "main" {
  count = var.create_cognito ? 1 : 0

  name = "${var.project_name}-users"

  # Password policy
  password_policy {
    minimum_length                   = 8
    require_lowercase                = true
    require_uppercase                = true
    require_numbers                  = true
    require_symbols                  = false
    temporary_password_validity_days = 7
  }

  # Allow users to sign up with email
  username_attributes      = ["email"]
  auto_verified_attributes = ["email"]

  # Account recovery via email
  account_recovery_setting {
    recovery_mechanism {
      name     = "verified_email"
      priority = 1
    }
  }

  # Email configuration (uses Cognito default sender for free tier)
  email_configuration {
    email_sending_account = "COGNITO_DEFAULT"
  }

  tags = {
    Name = "${var.project_name}-users"
  }
}

resource "aws_cognito_user_pool_client" "main" {
  count = var.create_cognito ? 1 : 0

  name         = "${var.project_name}-app"
  user_pool_id = aws_cognito_user_pool.main[0].id

  # No client secret — used from browser/mobile
  generate_secret = false

  # Token validity
  access_token_validity  = 1   # 1 hour
  id_token_validity      = 1   # 1 hour
  refresh_token_validity = 30  # 30 days

  token_validity_units {
    access_token  = "hours"
    id_token      = "hours"
    refresh_token = "days"
  }

  # Auth flows
  explicit_auth_flows = [
    "ALLOW_USER_PASSWORD_AUTH",
    "ALLOW_REFRESH_TOKEN_AUTH",
    "ALLOW_USER_SRP_AUTH",
  ]

  # Allowed callback URLs (update for Vercel domain)
  callback_urls = [
    "https://autodreamapplier.vercel.app/auth/callback",
    "http://localhost:3000/auth/callback",
  ]

  logout_urls = [
    "https://autodreamapplier.vercel.app",
    "http://localhost:3000",
  ]

  supported_identity_providers = ["COGNITO"]

  prevent_user_existence_errors = "ENABLED"
}

# Outputs — used to populate COGNITO_USER_POOL_ID etc. in Lambda env vars
output "cognito_user_pool_id" {
  description = "Cognito User Pool ID — set as COGNITO_USER_POOL_ID in Lambda"
  value       = var.create_cognito ? aws_cognito_user_pool.main[0].id : var.cognito_user_pool_id
}

output "cognito_app_client_id" {
  description = "Cognito App Client ID — set as COGNITO_APP_CLIENT_ID in Lambda"
  value       = var.create_cognito ? aws_cognito_user_pool_client.main[0].id : var.cognito_app_client_id
}
