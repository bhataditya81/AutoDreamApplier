"""Cover letter generation router."""

from fastapi import APIRouter, HTTPException

from app.models.schemas import CoverLetterRequest, CoverLetterResponse
from app.services.llm_service import llm_service

router = APIRouter()


@router.post("/generate", response_model=CoverLetterResponse)
async def generate_cover_letter(request: CoverLetterRequest):
    """Generate a tailored cover letter."""
    try:
        system_prompt = f"""You are an expert cover letter writer. Write a compelling,
{request.tone} cover letter that:
1. Opens with a strong hook specific to the company/role
2. Highlights 2-3 most relevant experiences from the resume
3. Shows genuine interest in the company's mission
4. Closes with a confident call to action
5. Is between 250-350 words
6. Does NOT use generic phrases like "I am writing to express my interest"
7. Does NOT use "Dear Hiring Manager" - use "Dear {request.company_name} Team" instead

Return ONLY the cover letter text, no additional commentary."""

        user_prompt = f"""Resume:
{request.resume_text}

Job Title: {request.job_title}
Company: {request.company_name}
Job Description:
{request.job_description}

Write a compelling cover letter for this position."""

        result = await llm_service.generate(
            system_prompt, user_prompt, max_tokens=1500, temperature=0.5
        )

        cover_letter = result.strip()
        word_count = len(cover_letter.split())

        return CoverLetterResponse(
            cover_letter=cover_letter,
            word_count=word_count,
        )

    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
