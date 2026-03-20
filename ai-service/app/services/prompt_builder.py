"""Prompt templates for AI generation tasks."""

RESUME_TAILOR_SYSTEM = """You are an expert resume writer. Your job is to tailor resumes to specific job descriptions.
Rules:
1. Never invent experience — only reorder, rephrase, highlight existing content
2. Inject relevant keywords from the job description naturally
3. Keep all factual details accurate
4. Return ONLY the tailored resume text, no commentary"""

RESUME_KEYWORD_INJECT_PROMPT = """Given the resume and job description below, return a lightly tailored version that:
- Incorporates key technical terms and skills from the job description
- Reorders bullet points to highlight the most relevant experience first
- Keeps all original facts intact

JOB TITLE: {job_title}
COMPANY: {company_name}
JOB DESCRIPTION:
{job_description}

ORIGINAL RESUME:
{resume_text}

Return only the tailored resume text."""

RESUME_FULL_REWRITE_PROMPT = """Rewrite the following resume to maximize relevance for this specific role.
Focus on alignment, clarity, and impact metrics.

JOB TITLE: {job_title}
COMPANY: {company_name}
JOB DESCRIPTION:
{job_description}

ORIGINAL RESUME:
{resume_text}

Return only the rewritten resume text."""

COVER_LETTER_PROMPT = """Write a concise, compelling cover letter (3 paragraphs, max 300 words) for a candidate
applying to {job_title} at {company_name}. Tone: {tone}.

Structure:
1. Opening: Why this specific role and company
2. Middle: 2-3 most relevant achievements from resume (specific numbers/impact)
3. Closing: Brief call to action

RESUME:
{resume_text}

JOB DESCRIPTION:
{job_description}

Return only the cover letter text, starting with "Dear Hiring Team,"."""

FORM_QA_PROMPT = """You are helping a job applicant answer application form questions honestly and concisely.

RESUME:
{resume_text}

JOB TITLE: {job_title}

QUESTION: {question}
QUESTION TYPE: {question_type}
{options_str}

Rules:
- Answer based ONLY on information in the resume
- For yes/no questions, answer Yes or No
- For text fields: max 150 words
- If you cannot answer confidently, say "I'd be happy to discuss this further in the interview"
- For dropdown/radio: pick the best matching option from the provided list

Return only the answer, no preamble."""
