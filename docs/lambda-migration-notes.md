# Lambda migration notes (for selected services)

You selected Lambda for:
- `api-gateway` (Go + Chi)
- `job-discovery` (Go + Chi)
- `job-matcher` (Go + Chi)
- `apply-engine` (Go + Chi + Asynq worker)
- `ai-service` (Python FastAPI)

Today these services run as **long-lived HTTP servers**. To deploy them on Lambda, add dedicated Lambda entrypoints while keeping the existing server entrypoints for local dev / ECS.

## Go services (Chi → API Gateway)

Recommended approach: keep your router setup, and adapt it via an API Gateway proxy adapter.

### 1) Refactor router construction
For each service, extract router creation into a reusable function:
- from `cmd/<service>/main.go`
- into `internal/<service>/http/router.go` (or similar)

That function should accept the dependencies it needs and return `http.Handler` (or `chi.Router`).

### 2) Add a Lambda entrypoint
Add a new command per service, for example:
- `cmd/api-gateway/lambda/main.go`
- `cmd/job-discovery/lambda/main.go`

In that Lambda `main.go`:
- initialize config + dependencies (same as existing `main.go`)\n- build the Chi router with the extracted function\n- wrap it with an API Gateway adapter (common library choice: `aws-lambda-go-api-proxy/chi`)\n- call `lambda.Start(adapter.ProxyWithContext)`

### 3) Build/package conventions
For zip-based Lambdas:
- build a Linux binary named `bootstrap` (custom runtime) or use `aws-lambda-go` conventions\n- zip it as `dist/<service>.zip`

For container-based Lambdas:
- build/push a Lambda-compatible image and set `--package-type Image`.

### 4) Apply-engine special case (async workers)
`apply-engine` currently runs:\n- an HTTP API **and**\n- an Asynq worker loop\n\nLambda is not a great fit for the worker loop because:\n- it expects long-lived concurrency\n- it relies on Redis-backed queues and steady processing\n\nRecommended split for Lambda:\n- keep **HTTP API** in Lambda (if desired)\n- move Asynq workers to **ECS/Fargate** (or separate worker service)\n\nIf you keep workers on ECS, the Lambda version should only expose HTTP routes.

## Python AI service (FastAPI → Lambda)

Recommended: adapt FastAPI to Lambda using an ASGI adapter.

### 1) Add adapter\n- Add dependency: `mangum`\n- Add `handler = Mangum(app)` next to your FastAPI `app`\n\n### 2) Package\n- Zip: install deps into a build directory and zip\n- Or container: build a Lambda image and deploy with `update-function-code --image-uri ...`

## GitLab CI integration

The repo already contains Lambda deploy templates:\n- `.gitlab/ci/deploy-lambda.yml`\n\nOnce `dist/*.zip` (or Lambda images) exist, add jobs that:\n- build the zip/image\n- call the deploy template with `LAMBDA_FUNCTION_NAME` and `LAMBDA_ZIP_PATH`/`IMAGE_URI`

