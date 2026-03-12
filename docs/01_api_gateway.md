# API Gateway Architecture (`cmd/api-gateway`)

The API Gateway acts as the strict entrypoint for all User-facing traffic requesting the AutoDreamApplier system. It connects the React frontend to the backend Postgres database and the Redis task queues.

## 1. Responsibilities
- Terminating JWT Authentication.
- Providing RESTful CRUD endpoints for Profiles, User Preferences, and Job Match states.
- Offloading heavy computation: Triggers Asynchronous workflows but does NOT execute them.

## 2. Router & Middleware Architecture
The gateway utilizes the highly efficient `go-chi/chi` router.

### Middleware Chain
1. **Recoverer**: Prevents panics from crashing the server instance.
2. **CORS**: Defines strict headers allowing communication from the Next.js `frontend`.
3. **Logger**: Custom request lifecycle logging for observability.
4. **Authentication (`auth.Middleware`)**: 
   - **Production Mode:** Parses the RSA signature from AWS Cognito JWTs to identify the user sub (`user_id`).
   - **Local Mode:** Reads `HS256` tokens signed by a secret `DEV_JWT_SECRET` for testing without relying on AWS billing.

## 3. High-Level Endpoint Map

| Category       | Method | Path                               | Description / Controller Logic                                                                                                                                                 |
| -------------- | ------ | ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Health**     | GET    | `/health`                          | Unauthenticated ping to verify readiness probes.                                                                                                                                 |
| **Profile**    | GET    | `/api/v1/profile`                  | Returns `users`, `user_preferences`, and current `resumes` joining data based on the authenticated context ID.                                                                 |
| **Profile**    | PUT    | `/api/v1/profile`                  | Upserts core identity data (Name, LinkedIn).                                                                                                                                   |
| **Matches**    | GET    | `/api/v1/matches`                  | Paginated fetch pulling `matches` rows with state = `pending`.                                                                                                                 |
| **Matches**    | POST   | `/api/v1/matches/{id}/approve`     | **Critical Endopint**: Updates match to `approved`. Creates a new `applications` row in Postgres. Serializes a `TypeAIPrep` job and broadcasts it instantly to Redis via Asynq. |
| **Matches**    | POST   | `/api/v1/matches/{id}/reject`      | Updates match status to `rejected`, hiding it from the dashboard.                                                                                                              |
| **Apps**       | GET    | `/api/v1/applications`             | Fetches the current states (`queued`, `applying`, `applied`, `failed`) for the dashboard timeline view.                                                                        |

## 4. State Management (Approve Match Flow)

When a user approves a job match, the gateway must execute a transactional workflow.

```go
// Psuedo-logic representation of `handlers.ApproveMatch`
tx := db.Begin()
defer tx.Rollback()

// 1. Mark match as approved
db.Exec("UPDATE matches SET status = 'approved' WHERE id = ?", matchID)

// 2. Create foundational Application row
appID := db.Query("INSERT INTO applications (match_id, status) VALUES (?, 'queued') RETURNING id", matchID)

// 3. Queue to Asynq
payload, _ := json.Marshal(tasks.AIPrepPayload{ApplicationID: appID, UserID: userID, JobID: jobID})
task := asynq.NewTask(tasks.TypeAIPrep, payload)
client.Enqueue(task, asynq.Queue("ai_prep"))

tx.Commit()

// 4. Return immediately
w.WriteHeader(http.StatusAccepted) // 202
```

By delegating immediately after creating the database row, the API remains highly available regardless of how long the actual LLM resume generation or Playwright browser automation takes.
