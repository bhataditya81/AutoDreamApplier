# GitLab CI templates (reusable)

This folder contains reusable GitLab CI includes you can copy into other repos.

## Templates
- `templates/aws/docker-to-ecr.yml`: build + push image to ECR (optionally also GitLab registry)
- `templates/aws/deploy-ecs-service.yml`: update ECS task definition image + deploy service
- `templates/aws/deploy-lambda-zip.yml`: update Lambda function code from a zip artifact
- `templates/aws/deploy-lambda-container.yml`: update Lambda function code from a container image
- `templates/aws/deploy-ec2-asg.yml`: deploy to EC2 via SSM (or hook for CodeDeploy)
- `templates/iac/terraform-plan-apply.yml`: Terraform plan/apply with remote state (S3 + DynamoDB)

## Usage

In another repo, copy `ci-templates/templates` and include:

```yaml
include:
  - local: ci-templates/templates/aws/deploy-ecs-service.yml
```

