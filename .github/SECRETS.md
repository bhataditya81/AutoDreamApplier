# Required GitHub Actions Secrets

Go to: Settings → Secrets and Variables → Actions → New repository secret

## AWS Authentication (OIDC — no long-lived credentials)
| Secret | Value |
|--------|-------|
| `AWS_DEPLOY_ROLE_ARN` | ARN of IAM role for GitHub Actions (e.g. `arn:aws:iam::123456789:role/autodream-github-deploy`) |
| `AWS_REGION` | `us-east-1` |

## EC2 Access
| Secret | Value |
|--------|-------|
| `EC2_SSH_PRIVATE_KEY` | Content of your `.pem` key file |
| `EC2_PUBLIC_IP` | Elastic IP of the browser pool EC2 |

## Terraform Variables (sensitive)
| Secret | Value |
|--------|-------|
| `DATABASE_URL` | Neon PostgreSQL connection string |
| `REDIS_URL` | Upstash Redis connection string |
| `JWT_SECRET` | 32+ char random secret |
| `ENCRYPTION_KEY` | 32-byte hex key |
| `EC2_KEY_PAIR_NAME` | Name of EC2 key pair (not the PEM, just the name) |
| `ADMIN_SSH_CIDR` | Your IP as CIDR (e.g. `1.2.3.4/32`) |

## Setting up OIDC (Recommended — no long-lived AWS credentials)

Create an IAM OIDC identity provider and role in AWS:

```bash
# 1. Create OIDC provider (one-time per account)
aws iam create-open-id-connect-provider \
  --url https://token.actions.githubusercontent.com \
  --client-id-list sts.amazonaws.com \
  --thumbprint-list 6938fd4d98bab03faadb97b34396831e3780aea1

# 2. Create the deploy role (see infra/terraform/github_oidc.tf)
```
