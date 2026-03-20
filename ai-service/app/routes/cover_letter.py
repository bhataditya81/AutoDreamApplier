from fastapi import APIRouter, HTTPException
from app.models import CoverLetterRequest, CoverLetterResponse
from app.services import anthropic_client, prompt_builder

router = APIRouter()

@router.post("/cover-letter/generate", response_model=CoverLetterResponse)
async def generate_cover_letter(req: CoverLetterRequest):
    if not req.resume_text.strip():
        raise HTTPException(status_code=422, detail="resume_text is required")

    prompt = prompt_builder.COVER_LETTER_PROMPT.format(
        job_title=req.job_title,
        company_name=req.company_name,
        tone=req.tone,
        resume_text=req.resume_text,
        job_description=req.job_description,
    )

    cover_letter = anthropic_client.complete(prompt, max_tokens=800)

    # Enforce word limit — retry once if exceeded
    word_count = len(cover_letter.split())
    if word_count > 400:
        cover_letter = anthropic_client.complete(
            prompt + "\n\nIMPORTANT: Keep it under 300 words.", max_tokens=600
        )
        word_count = len(cover_letter.split())

    return CoverLetterResponse(cover_letter=cover_letter, word_count=word_count)
