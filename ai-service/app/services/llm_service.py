"""LLM service for interacting with Claude API."""

import os

import anthropic


class LLMService:
    """Service for generating AI content using Claude.

    Uses AsyncAnthropic so that all API calls are fully non-blocking
    and do not stall the FastAPI event loop.
    """

    def __init__(self):
        self.client = anthropic.AsyncAnthropic(
            api_key=os.getenv("ANTHROPIC_API_KEY", ""),
        )
        self.model = os.getenv("LLM_MODEL", "claude-3-haiku-20240307")

    async def generate(
        self,
        system_prompt: str,
        user_prompt: str,
        max_tokens: int = 2048,
        temperature: float = 0.3,
    ) -> str:
        """Generate a response using Claude (non-blocking)."""
        message = await self.client.messages.create(
            model=self.model,
            max_tokens=max_tokens,
            temperature=temperature,
            system=system_prompt,
            messages=[{"role": "user", "content": user_prompt}],
        )

        return message.content[0].text


# Singleton instance
llm_service = LLMService()
