# AI Orchestration Service (`ai-service/`)

The AI Service is a Python-based FastAPI microservice responsible for translating unstructured text formats into strictly formatted payloads required by the Apply Engine.

## 1. Architectural Role
Similar to the Browser Pool, Python is the native tongue of Machine Learning ecosystems (Langchain, PyTorch, Tokenizers). By extracting AI logic into `ai-service/`, AutoDreamApplier cleanly isolates its REST API codebase from experimental ML pipelines.

The service utilizes **Anthropic Claude (Haiku)** as its base LLM due to its industry-leading speed/cost ratio for vast text parsing.

## 2. API Contract
The main entrypoint for the `Apply Engine` orchestration is the transformation of a generic resume into a strictly targeted one.

### `POST /tailor`
```json
{
  "job_description": "We are looking for a Senior Go Engineer with Kubernetes experience...",
  "base_resume_text": "John Doe. Software Engineer. Experienced in Python and Docker."
}
```

## 3. The Tailoring Pipeline
When a request arrives, the FastAPI server formats a heavily structured system prompt (RAG).

1. **Information Extraction (Job):** The LLM parses the Raw Job Description to extract the top 5 "hard requirements" (e.g., Go, Kubernetes, System Design).
2. **Information Extraction (Resume):** The LLM parses the user's base resume object.
3. **Re-weaving (The Transformation):** A new prompt is evaluated directing the LLM to rewrite the "Summary" and "Experience" bullet points of the user's resume, explicitly pulling the Job's Hard Requirements to the linguistic forefront, *without fabricating* skills the user does not possess.
4. **Formatting:** The AI Service converts the LLM's raw markdown output back into a sanitized PDF object using `WeasyPrint` or similar rendering libraries.
5. **Return:** The PDF byte stream is shipped back across the internal network to the Go Apply Engine for S3 persistence.

## 4. Future Phase 2: Embeddings & PGVector

Currently under design, the AI Service will expose a secondary `POST /embed` endpoint.
- As the Job Discovery engine scrapes new postings, it will asynchronously fire the titles/descriptions to the `ai-service`.
- The `ai-service` will utilize a dense sentence transformer (e.g., huggingface `all-MiniLM-L6-v2`) locally to compute a Float32 vector array (`[0.142, -0.015, ..., 0.884]`).
- This vector is appended to the job object and returned for storage in the PostgreSQL `pgvector` column, allowing the Go Matcher to perform semantic nearest-neighbor searches at near zero-latency computationally.
