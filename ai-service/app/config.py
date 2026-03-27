from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    app_env: str = "development"
    ai_provider: str = "gemini"  # "gemini" | "anthropic"

    # Gemini (default provider)
    gemini_api_key: str = ""
    gemini_model: str = "gemini-1.5-flash"

    # Anthropic (fallback)
    anthropic_api_key: str = ""
    anthropic_model: str = "claude-haiku-4-5-20251001"

    # AWS / S3
    aws_region: str = "us-east-1"
    aws_access_key_id: str = ""
    aws_secret_access_key: str = ""
    s3_bucket_resumes: str = "ada-resumes"
    s3_bucket_ai_outputs: str = "ada-screenshots"

    class Config:
        env_file = ".env"


settings = Settings()
