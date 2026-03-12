"""Pydantic schemas for AI service requests and responses."""

from pydantic import BaseModel, Field


class ResumeTailorRequest(BaseModel):
    """Request to tailor a resume for a specific job."""

    resume_text: str = Field(..., description="The original resume text")
    job_title: str = Field(..., description="Target job title")
    job_description: str = Field(..., description="Full job description")
    company_name: str = Field(default="", description="Company name")
    mode: str = Field(
        default="keyword_inject",
        description="Tailoring mode: 'keyword_inject' or 'full_rewrite'",
    )


class ResumeTailorResponse(BaseModel):
    """Response with tailored resume content."""

    tailored_text: str
    changes_made: list[str] = Field(default_factory=list)
    keywords_added: list[str] = Field(default_factory=list)


class CoverLetterRequest(BaseModel):
    """Request to generate a cover letter."""

    resume_text: str = Field(..., description="Candidate's resume text")
    job_title: str = Field(..., description="Target job title")
    job_description: str = Field(..., description="Full job description")
    company_name: str = Field(..., description="Company name")
    tone: str = Field(
        default="professional",
        description="Tone: 'professional', 'enthusiastic', 'conversational'",
    )


class CoverLetterResponse(BaseModel):
    """Response with generated cover letter."""

    cover_letter: str
    word_count: int


class FormQARequest(BaseModel):
    """Request to answer a form question."""

    question: str = Field(..., description="The form question to answer")
    resume_text: str = Field(..., description="Candidate's resume text")
    job_title: str = Field(default="", description="Target job title")
    question_type: str = Field(
        default="text",
        description="Question type: 'text', 'textarea', 'dropdown', 'radio', 'checkbox'",
    )
    options: list[str] = Field(
        default_factory=list,
        description="Available options for dropdown/radio/checkbox questions",
    )


class FormQAResponse(BaseModel):
    """Response with the answer to a form question."""

    answer: str
    confidence: float = Field(
        default=0.0,
        ge=0.0,
        le=1.0,
        description="Confidence score of the answer",
    )
