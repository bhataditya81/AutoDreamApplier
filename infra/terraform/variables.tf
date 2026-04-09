variable "aws_region" {
  description = "AWS region for all resources"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Deployment environment"
  type        = string
  default     = "production"
}

variable "project_name" {
  description = "Project identifier used in resource names"
  type        = string
  default     = "autodream"
}

# --- External service URLs (Neon, Upstash, Vercel) ---

variable "database_url" {
  description = "Neon PostgreSQL connection string (postgres://user:pass@host/db?sslmode=require)"
  type        = string
  sensitive   = true
}

variable "redis_url" {
  description = "Upstash Redis connection string (redis://default:pass@host:port)"
  type        = string
  sensitive   = true
}

variable "jwt_secret" {
  description = "HS256 JWT secret for dev tokens (min 32 chars) — used as DEV_JWT_SECRET"
  type        = string
  sensitive   = true
}

variable "dev_jwt_secret" {
  description = "Alias for jwt_secret — passed as DEV_JWT_SECRET env var"
  type        = string
  sensitive   = true
  default     = ""
}

variable "app_env" {
  description = "APP_ENV value: 'production' (Cognito RS256) or 'staging' (dev HS256 auth)"
  type        = string
  default     = "staging"
}

# --- Cognito (required when app_env=production) ---

variable "cognito_user_pool_id" {
  description = "Cognito User Pool ID (e.g. us-east-1_XXXXXXXXX)"
  type        = string
  default     = ""
}

variable "cognito_app_client_id" {
  description = "Cognito App Client ID"
  type        = string
  default     = ""
}

variable "cognito_domain" {
  description = "Cognito hosted UI domain (optional)"
  type        = string
  default     = ""
}

# --- AI / LLM ---

variable "anthropic_api_key" {
  description = "Anthropic API key (optional, used when ai_provider=anthropic)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "gemini_api_key" {
  description = "Google Gemini API key (used when ai_provider=gemini)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "ai_provider" {
  description = "AI provider: 'gemini' (default), 'anthropic', 'python' (self-hosted)"
  type        = string
  default     = "gemini"
}

variable "llm_model" {
  description = "LLM model name (e.g. gemini-1.5-flash or claude-3-haiku-20240307)"
  type        = string
  default     = "gemini-1.5-flash"
}

# --- Browser pool ---

variable "encryption_key" {
  description = "32-byte hex key for field encryption"
  type        = string
  sensitive   = true
}

variable "ses_from_email" {
  description = "Verified SES sender email"
  type        = string
  default     = ""
}

variable "dashboard_url" {
  description = "Frontend URL for notification links"
  type        = string
  default     = "https://autodreamapplier.vercel.app"
}

variable "linkedin_proxy_url" {
  description = "Residential proxy URL for LinkedIn scraper (optional)"
  type        = string
  default     = ""
  sensitive   = true
}

variable "frontend_cors_origins" {
  description = "Allowed CORS origins for the API"
  type        = list(string)
  default     = ["https://autodreamapplier.vercel.app", "https://autodreamapplier.com"]
}

# --- EC2 ---

variable "admin_ssh_cidr" {
  description = "Your IP in CIDR notation for SSH access (e.g. 1.2.3.4/32)"
  type        = string
}

variable "ec2_ami_id" {
  description = "Pre-baked Packer AMI with Docker + aws-cli pre-installed. Empty = use latest AL2023 ARM64 (slower boot)."
  type        = string
  default     = ""
}

# --- S3 ---

variable "s3_bucket_resumes" {
  description = "S3 bucket name for resume PDFs"
  type        = string
  default     = "autodream-resumes"
}

variable "s3_bucket_screenshots" {
  description = "S3 bucket name for application screenshots"
  type        = string
  default     = "autodream-screenshots"
}
