"""Wrapper around Google Generative AI (Gemini) SDK with retry logic."""
import time
import google.generativeai as genai
from app.config import settings

_model = None


def get_model():
    global _model
    if _model is None:
        genai.configure(api_key=settings.gemini_api_key)
        _model = genai.GenerativeModel(settings.gemini_model)
    return _model


def complete(prompt: str, max_tokens: int = 2048, system: str = "") -> str:
    """Call Gemini with simple retry on transient errors."""
    model = get_model()
    full_prompt = f"{system}\n\n{prompt}" if system else prompt
    for attempt in range(3):
        try:
            response = model.generate_content(
                full_prompt,
                generation_config=genai.GenerationConfig(max_output_tokens=max_tokens),
            )
            return response.text
        except Exception:
            if attempt == 2:
                raise
            time.sleep(2 ** attempt)
    return ""
