"""Form question answering router."""

from fastapi import APIRouter, HTTPException

from app.models.schemas import FormQARequest, FormQAResponse
from app.services.llm_service import llm_service

router = APIRouter()


@router.post("/answer", response_model=FormQAResponse)
async def answer_form_question(request: FormQARequest):
    """Answer a job application form question using resume context."""
    try:
        options_text = ""
        if request.options:
            options_text = f"\nAvailable options: {', '.join(request.options)}"
            options_text += "\nYou MUST select from these options only."

        system_prompt = f"""You are helping a job applicant fill out an application form.
Answer the question based on the candidate's resume. Be concise and professional.

Question type: {request.question_type}
{options_text}

Rules:
- For text/textarea: Give a direct, honest answer based on the resume
- For dropdown/radio: Select the most appropriate option from the given list
- For checkbox: List the applicable options separated by commas
- If the answer cannot be determined from the resume, give a reasonable professional answer
- For salary questions: Give a range, not a specific number
- For "Are you authorized to work" type questions: Answer "Yes" unless resume suggests otherwise
- For years of experience: Calculate from the resume dates
- Keep answers concise (1-2 sentences for text, single selection for dropdown/radio)

Return ONLY the answer, nothing else."""

        user_prompt = f"""Resume:
{request.resume_text}

Job Title: {request.job_title}

Question: {request.question}

Answer this question based on the resume."""

        result = await llm_service.generate(
            system_prompt, user_prompt, max_tokens=500, temperature=0.2
        )

        answer = result.strip()

        # Determine confidence
        confidence = 0.8
        if request.options and answer in request.options:
            confidence = 0.95
        elif request.question_type in ("dropdown", "radio") and request.options:
            # Check if the answer is close to any option
            answer_lower = answer.lower()
            for option in request.options:
                if option.lower() in answer_lower or answer_lower in option.lower():
                    answer = option
                    confidence = 0.9
                    break
            else:
                confidence = 0.5

        return FormQAResponse(answer=answer, confidence=confidence)

    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
