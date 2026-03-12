# Job Matcher Engine (`cmd/job-matcher`)

The Job Matcher is responsible for continually evaluating newly discovered `jobs` against configured `user_preferences`.

## 1. Overview
As the Job Discovery service inflates the `jobs` database table with active records from Indeed and Glassdoor, the Job Matcher asynchronously cross-references this dataset to determine if a job aligns perfectly with a user's career goals. If a high-confidence match is established, it generates a `matches` row which alerts the user on the React frontend dashboard.

## 2. Match Analysis Modes

The system fundamentally integrates two distinct mechanisms for ranking jobs against a user.

### Mode 1: Deterministic Keyword Matching (MVP)
The baseline matching engine uses hard-constraints configured in a user's `user_preferences` object:
1. **Title Filtering:** Standard `ILIKE` SQL queries isolating jobs with titles matching targeted strings (e.g. `Software Engineer`, `Site Reliability Engineer`).
2. **Location Proximity:** Filters matching state/city or remote constraints.
3. **Salary Floors:** Evaluates `job.salary_min` against the user's explicit floor requirement.

### Mode 2: Semantic Vector Matching (Phase 2 & Extensibility)
Relying solely on keywords is fragile (i.e. missing a match because a job title is "Code Builder" instead of "Software Engineer"). 
AutoDreamApplier heavily integrates `PostgreSQL pgvector` to enable Semantic matching.

1. **Embedding Generation:** Upon job ingestion, the full job description and title is sent to the `ai-service` to be vectorized using a dense encoder model (e.g., `all-MiniLM-L6-v2`). The resulting Float array is stored in the `jobs.embedding` column.
2. **User Context Building:** The Matcher maintains a highly detailed Vector of the user representing their `user_preferences` profile and historical resume.
3. **Cosine Similarity Lookup:**
```sql
SELECT id, title, company, 1 - (embedding <=> $1) AS similarity_score
FROM jobs
WHERE 1 - (embedding <=> $1) > 0.82 
  AND is_active = true
ORDER BY similarity_score DESC;
```
Jobs breaching the `0.82` cosine-similarity boundary are flagged as highly-relevant without exact keyword matching.

## 3. Match Context Output
A critical UX component of the Matcher is informing the user *why* a particular job was queued on their dashboard.
When a match is established, a `match_breakdown` artifact is compiled outlining what criteria (or what portions of semantic overlap) triggered the hit.

## 4. The `Auto-Apply` Pipeline Concept
A user's `apply_mode` is configured to `manual` by default. 
If toggled to `auto`, the Job Matcher skips the UI approval queue. When a match score exceeds the user's `auto_threshold`, the Job Matcher immediately acts as the API Gateway: It invokes the insertion of an `applications` row and dispatches the `TypeAIPrep` pipeline task to Redis via Asynq, making the system 100% physically autonomous.
