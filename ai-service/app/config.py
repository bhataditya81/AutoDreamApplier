from pydantic_settings import BaseSettings

class Settings(BaseSettings):
    app_env: str = "development"
    anthropic_api_key: str = ""
    aws_region: str = "us-east-1"
    aws_access_key_id: str = ""
    aws_secret_access_key: str = ""
    s3_bucket_resumes: str = "ada-resumes"
    s3_bucket_ai_outputs: str = "ada-screenshots"  # reuse bucket for AI outputs
    anthropic_model: str = "claude-haiku-4-5-20251001"  # cost-efficient

    class Config:
        env_file = ".env"

settings = Settings()
