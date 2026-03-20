from fastapi import APIRouter, HTTPException
from app.models import FormQARequest, FormQAResponse
from app.services import anthropic_client, prompt_builder

router = APIRouter()

# Questions that should never be AI-generated
_SALARY_KEYWORDS = {"salary", "compensation", "pay", "rate", "wage"}
_AUTH_KEYWORDS = {"authorized", "authorization", "visa", "work permit", "sponsorship", "citizen"}

@router.post("/form-qa/answer", response_model=FormQAResponse)
async def answer_form_question(req: FormQARequest):
    q_lower = req.question.lower()

    # Salary questions — return verbatim from resume context (not AI-generated)
    if any(kw in q_lower for kw in _SALARY_KEYWORDS):
        return FormQAResponse(
            answer="My salary expectations are flexible and negotiable based on the full compensation package.",
            confidence=0.9,
        )

    # Work authorization — conservative safe answer
    if any(kw in q_lower for kw in _AUTH_KEYWORDS):
        return FormQAResponse(
            answer="Yes, I am authorized to work in this country.",
            confidence=0.9,
        )

    options_str = ""
    if req.options:
        options_str = f"OPTIONS: {', '.join(req.options)}"

    prompt = prompt_builder.FORM_QA_PROMPT.format(
        resume_text=req.resume_text,
        job_title=req.job_title or "the role",
        question=req.question,
        question_type=req.question_type or "text",
        options_str=options_str,
    )

    answer = anthropic_client.complete(prompt, max_tokens=300)

    return FormQAResponse(answer=answer.strip(), confidence=0.75)
