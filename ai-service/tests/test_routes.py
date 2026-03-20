"""Unit tests for AI service routes using mocked Anthropic client."""
from unittest.mock import patch, MagicMock
from fastapi.testclient import TestClient

from main import app

client = TestClient(app)

def mock_complete(prompt, max_tokens=2048, system=""):
    return "Mocked AI response for testing purposes with enough content."

def test_health():
    resp = client.get("/health")
    assert resp.status_code == 200
    assert resp.json()["status"] == "ok"

@patch("app.services.anthropic_client.complete", side_effect=mock_complete)
def test_tailor_resume(mock_ai):
    resp = client.post("/api/v1/resume/tailor", json={
        "resume_text": "Experienced engineer with Python and Go skills.",
        "job_title": "Senior Engineer",
        "job_description": "We need Python, Go, and Kubernetes experience.",
        "company_name": "Acme",
        "mode": "keyword_inject",
    })
    assert resp.status_code == 200
    data = resp.json()
    assert "tailored_text" in data
    assert "changes_made" in data

@patch("app.services.anthropic_client.complete", side_effect=mock_complete)
def test_cover_letter(mock_ai):
    resp = client.post("/api/v1/cover-letter/generate", json={
        "resume_text": "Experienced engineer.",
        "job_title": "Engineer",
        "job_description": "Build things.",
        "company_name": "Stripe",
        "tone": "professional",
    })
    assert resp.status_code == 200
    assert "cover_letter" in resp.json()

def test_form_qa_salary_question():
    resp = client.post("/api/v1/form-qa/answer", json={
        "question": "What is your expected salary?",
        "resume_text": "Engineer with 5 years experience.",
    })
    assert resp.status_code == 200
    assert "salary" in resp.json()["answer"].lower() or "flexible" in resp.json()["answer"].lower()

@patch("app.services.anthropic_client.complete", side_effect=mock_complete)
def test_form_qa_general(mock_ai):
    resp = client.post("/api/v1/form-qa/answer", json={
        "question": "Describe your experience with distributed systems.",
        "resume_text": "Built distributed systems at scale.",
        "job_title": "Senior Engineer",
    })
    assert resp.status_code == 200
    assert "answer" in resp.json()

def test_cover_letter_invalid_tone():
    resp = client.post("/api/v1/cover-letter/generate", json={
        "resume_text": "Engineer.",
        "job_title": "Dev",
        "job_description": "Code things.",
        "company_name": "Acme",
        "tone": "angry",  # invalid
    })
    assert resp.status_code == 422
