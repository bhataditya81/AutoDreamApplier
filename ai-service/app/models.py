from typing import Optional
from pydantic import BaseModel, field_validator

class ResumeTailorRequest(BaseModel):
    resume_text: str
    job_title: str
    job_description: str
    company_name: Optional[str] = ""
    mode: str = "keyword_inject"  # "keyword_inject" | "full_rewrite"

class ResumeTailorResponse(BaseModel):
    tailored_text: str
    changes_made: list[str]
    keywords_added: list[str]

class CoverLetterRequest(BaseModel):
    resume_text: str
    job_title: str
    job_description: str
    company_name: str
    tone: str = "professional"

    @field_validator("tone")
    @classmethod
    def validate_tone(cls, v):
        allowed = {"professional", "enthusiastic", "conversational", "concise"}
        if v not in allowed:
            raise ValueError(f"tone must be one of {allowed}")
        return v

class CoverLetterResponse(BaseModel):
    cover_letter: str
    word_count: int

class FormQARequest(BaseModel):
    question: str
    resume_text: str
    job_title: Optional[str] = ""
    question_type: Optional[str] = "text"  # text, textarea, dropdown, radio, checkbox
    options: Optional[list[str]] = []

class FormQAResponse(BaseModel):
    answer: str
    confidence: float
