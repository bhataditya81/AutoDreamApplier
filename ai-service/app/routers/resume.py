"""Resume tailoring router."""

from fastapi import APIRouter, HTTPException

from app.models.schemas import ResumeTailorRequest, ResumeTailorResponse
from app.services.llm_service import llm_service

router = APIRouter()


@router.post("/tailor", response_model=ResumeTailorResponse)
async def tailor_resume(request: ResumeTailorRequest):
    """Tailor a resume for a specific job posting."""
    try:
        if request.mode == "keyword_inject":
            return await _keyword_inject(request)
        elif request.mode == "full_rewrite":
            return await _full_rewrite(request)
        else:
            raise HTTPException(
                status_code=400,
                detail=f"Invalid mode: {request.mode}. Use 'keyword_inject' or 'full_rewrite'.",
            )
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


async def _keyword_inject(request: ResumeTailorRequest) -> ResumeTailorResponse:
    """Inject relevant keywords from the job description into the resume."""
    system_prompt = """You are a resume optimization expert. Your task is to subtly inject
relevant keywords from the job description into the existing resume WITHOUT changing the
overall structure or making it obvious. Only modify existing bullet points to naturally
include relevant keywords. Do NOT add fake experience or skills the candidate doesn't have.

Return the modified resume text followed by a section starting with "---CHANGES---" listing
each change made, and a section "---KEYWORDS---" listing keywords that were added."""

    user_prompt = f"""Original Resume:
{request.resume_text}

Job Title: {request.job_title}
Company: {request.company_name}
Job Description:
{request.job_description}

Modify the resume to better match this job. Only make subtle keyword optimizations."""

    result = await llm_service.generate(system_prompt, user_prompt, max_tokens=3000)

    # Parse the response
    parts = result.split("---CHANGES---")
    tailored_text = parts[0].strip()
    changes = []
    keywords = []

    if len(parts) > 1:
        remaining = parts[1]
        keyword_parts = remaining.split("---KEYWORDS---")
        changes = [
            c.strip().lstrip("- ")
            for c in keyword_parts[0].strip().split("\n")
            if c.strip()
        ]
        if len(keyword_parts) > 1:
            keywords = [
                k.strip().lstrip("- ")
                for k in keyword_parts[1].strip().split("\n")
                if k.strip()
            ]

    return ResumeTailorResponse(
        tailored_text=tailored_text,
        changes_made=changes,
        keywords_added=keywords,
    )


async def _full_rewrite(request: ResumeTailorRequest) -> ResumeTailorResponse:
    """Fully rewrite the resume optimized for the target job."""
    system_prompt = """You are an expert resume writer. Completely rewrite the resume to be
highly optimized for the target job while maintaining truthfulness. Reorganize sections,
rewrite bullet points with strong action verbs and quantified achievements, and ensure
ATS keyword optimization. Keep the same factual information but present it in the most
compelling way for this specific role.

Return the rewritten resume text followed by "---CHANGES---" listing major changes."""

    user_prompt = f"""Original Resume:
{request.resume_text}

Target Job: {request.job_title} at {request.company_name}
Job Description:
{request.job_description}

Rewrite this resume to be highly competitive for this position."""

    result = await llm_service.generate(
        system_prompt, user_prompt, max_tokens=4000, temperature=0.4
    )

    parts = result.split("---CHANGES---")
    tailored_text = parts[0].strip()
    changes = []

    if len(parts) > 1:
        changes = [
            c.strip().lstrip("- ")
            for c in parts[1].strip().split("\n")
            if c.strip()
        ]

    return ResumeTailorResponse(
        tailored_text=tailored_text,
        changes_made=changes,
        keywords_added=[],
    )
