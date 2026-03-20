"""Wrapper around the Anthropic Python SDK with retry logic."""
import time
import anthropic
from app.config import settings

_client: anthropic.Anthropic | None = None

def get_client() -> anthropic.Anthropic:
    global _client
    if _client is None:
        _client = anthropic.Anthropic(api_key=settings.anthropic_api_key)
    return _client

def complete(prompt: str, max_tokens: int = 2048, system: str = "") -> str:
    """Call Claude with simple retry on overload."""
    client = get_client()
    for attempt in range(3):
        try:
            msg = client.messages.create(
                model=settings.anthropic_model,
                max_tokens=max_tokens,
                system=system or "You are a professional career coach and expert resume writer.",
                messages=[{"role": "user", "content": prompt}],
            )
            return msg.content[0].text
        except anthropic.RateLimitError:
            if attempt == 2:
                raise
            time.sleep(2 ** attempt)
    return ""
