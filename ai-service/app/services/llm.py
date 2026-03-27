"""Provider-agnostic LLM interface.

Routes to Gemini or Anthropic based on the AI_PROVIDER env var.
All routes import this module instead of a specific client so that
switching providers requires only a config change.
"""
from app.config import settings


def complete(prompt: str, max_tokens: int = 2048, system: str = "") -> str:
    if settings.ai_provider == "anthropic":
        from app.services import anthropic_client
        return anthropic_client.complete(prompt, max_tokens, system)
    # Default: Gemini
    from app.services import gemini_client
    return gemini_client.complete(prompt, max_tokens, system)
