import re
from fastapi import APIRouter, HTTPException
from app.models import ResumeTailorRequest, ResumeTailorResponse
from app.services import llm, prompt_builder

router = APIRouter()

@router.post("/resume/tailor", response_model=ResumeTailorResponse)
async def tailor_resume(req: ResumeTailorRequest):
    if not req.resume_text.strip():
        raise HTTPException(status_code=422, detail="resume_text is required")
    if not req.job_description.strip():
        raise HTTPException(status_code=422, detail="job_description is required")

    template = (
        prompt_builder.RESUME_FULL_REWRITE_PROMPT
        if req.mode == "full_rewrite"
        else prompt_builder.RESUME_KEYWORD_INJECT_PROMPT
    )
    prompt = template.format(
        job_title=req.job_title,
        company_name=req.company_name or "",
        job_description=req.job_description,
        resume_text=req.resume_text,
    )

    tailored = llm.complete(
        prompt,
        max_tokens=3000,
        system=prompt_builder.RESUME_TAILOR_SYSTEM,
    )

    # Detect keywords added (simple diff)
    jd_words = set(re.findall(r'\b[a-zA-Z]{4,}\b', req.job_description.lower()))
    orig_words = set(re.findall(r'\b[a-zA-Z]{4,}\b', req.resume_text.lower()))
    added = list(jd_words - orig_words)[:10]

    return ResumeTailorResponse(
        tailored_text=tailored,
        changes_made=["Tailored to job requirements", f"Mode: {req.mode}"],
        keywords_added=added,
    )
