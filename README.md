# AutoDreamApplier

Multi-service app with:
- Go backend services (`./cmd/*`)
- Python AI service (`./ai-service`)
- Next.js frontend (`./frontend`)

## Local development

### Prereqs
- Docker Desktop
- Node.js 18+
- Go (matching `go.mod`)

### Start everything

```bash
make dev
```

On Windows:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\\dev.ps1
```

### Useful commands
- `make test`: run Go tests
- `make lint`: run `golangci-lint`
- `make docker-up`: start containers
- `make docker-down`: stop containers

## CI/CD

This repo is designed to run on GitLab CI with reusable pipelines for AWS deployments (ECS Fargate / Lambda / EC2).

