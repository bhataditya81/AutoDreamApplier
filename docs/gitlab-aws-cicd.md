# GitLab CI/CD → AWS (ECS + Lambda)

This repo ships with a modular GitLab pipeline in `.gitlab/ci/` that:
- runs Go/Python/Next.js CI
- builds and pushes Docker images to **GitLab Container Registry**
- mirrors images to **AWS ECR** (dev on default branch, prod on tags)
- provides reusable deploy jobs for **ECS** (and scaffolding for **Lambda**)

## Required GitLab CI variables

Set these in GitLab: **Settings → CI/CD → Variables**.

### AWS authentication (static keys)
- `AWS_REGION` (example: `us-east-1`)
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`

### Accounts / registries
- `AWS_ACCOUNT_ID_DEV`
- `AWS_ACCOUNT_ID_PROD`
- `ECR_REPO_PREFIX` (optional; default `autodreamapplier`)

### ECS (deploy jobs)
- `ECS_CLUSTER_DEV`
- `ECS_CLUSTER_PROD`

## How image publishing works

On the **default branch**:
- images are pushed to GitLab registry with tag `${CI_COMMIT_SHA}` and `latest`
- images are mirrored to **dev** ECR:\n  `${AWS_ACCOUNT_ID_DEV}.dkr.ecr.${AWS_REGION}.amazonaws.com/${ECR_REPO_PREFIX}/${service}:${CI_COMMIT_SHA}` and `latest`

On **tags**:
- images are pushed to GitLab registry with tag `${CI_COMMIT_SHA}`\n  (and you can extend this to push `${CI_COMMIT_TAG}` if desired)\n- images are mirrored to **prod** ECR:\n  `${AWS_ACCOUNT_ID_PROD}.dkr.ecr.${AWS_REGION}.amazonaws.com/${ECR_REPO_PREFIX}/${service}:${CI_COMMIT_TAG}`

ECR repositories are auto-created if they do not exist.

## ECS deploy prerequisites

The ECS deploy jobs update an existing service by registering a new task definition revision with an updated container image.

You must create (per service/environment):
- an ECS cluster (name stored in `ECS_CLUSTER_DEV` / `ECS_CLUSTER_PROD`)
- ECS services: `api-gateway`, `job-discovery`, `job-matcher`, `apply-engine`, `browser-pool`, `ai-service`
- task definition families matching those names
- container definition **names** matching those names

If your container names differ, update `ECS_CONTAINER_NAME` per deploy job in `.gitlab/ci/deploy-ecs.yml`.

## Lambda notes (current state)

The repo includes deploy job templates in `.gitlab/ci/deploy-lambda.yml`, but **Lambda packaging is not implemented** yet because the services currently run as long-lived HTTP servers.\n\nTo run these services on Lambda, add Lambda entrypoints:\n- **Go (Chi)**: use an API Gateway adapter (e.g. `aws-lambda-go-api-proxy/chi`) to reuse your existing router.\n- **Python (FastAPI)**: use a Lambda adapter (e.g. Mangum) and package as zip or container.\n\nOnce those entrypoints exist, add build jobs that produce `dist/*.zip` (or ECR images) and connect them to the deploy templates.

