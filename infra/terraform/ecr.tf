locals {
  lambda_services = ["api-gateway", "job-discovery", "job-matcher", "followup-scheduler"]
  ec2_services    = ["apply-engine", "browser-pool", "ai-service"]
  all_services    = concat(local.lambda_services, local.ec2_services)
}

resource "aws_ecr_repository" "services" {
  for_each = toset(local.all_services)

  name                 = "${lower(var.project_name)}/${each.key}"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_ecr_lifecycle_policy" "services" {
  for_each   = aws_ecr_repository.services
  repository = each.value.name

  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "Keep last 5 images"
      selection = {
        tagStatus   = "any"
        countType   = "imageCountMoreThan"
        countNumber = 5
      }
      action = { type = "expire" }
    }]
  })
}

output "ecr_registry" {
  description = "ECR registry URL (account.dkr.ecr.region.amazonaws.com)"
  value       = "${data.aws_caller_identity.current.account_id}.dkr.ecr.${var.aws_region}.amazonaws.com"
}

output "ecr_repositories" {
  description = "Map of service name to ECR repository URL"
  value       = { for k, v in aws_ecr_repository.services : k => v.repository_url }
}
