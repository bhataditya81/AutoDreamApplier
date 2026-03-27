output "api_endpoint" {
  description = "API Gateway HTTP API endpoint — set as NEXT_PUBLIC_API_URL in Vercel"
  value       = aws_apigatewayv2_stage.default.invoke_url
}

output "setup_commands" {
  description = "Post-deployment setup commands"
  value       = <<-EOT
    # 1. Copy docker-compose.yml to EC2
    scp -i ~/.ssh/${var.ec2_key_pair_name}.pem \
        deployments/ec2/docker-compose.yml \
        ec2-user@${aws_eip.browser_pool.public_ip}:/opt/autodream/docker-compose.yml

    # 2. Start services on EC2
    ssh -i ~/.ssh/${var.ec2_key_pair_name}.pem ec2-user@${aws_eip.browser_pool.public_ip} \
        "sudo systemctl start autodream"

    # 3. Set NEXT_PUBLIC_API_URL in Vercel:
    #    ${aws_apigatewayv2_stage.default.invoke_url}

    # 4. Verify Lambda is working:
    curl ${aws_apigatewayv2_stage.default.invoke_url}/health
  EOT
}

output "monthly_cost_estimate" {
  description = "Estimated monthly AWS cost for Tier 4"
  value       = <<-EOT
    EC2 t4g.nano (always-on):  $3.07/mo
    Lambda (all functions):    ~$0.30/mo
    API Gateway HTTP API:      ~$0.15/mo
    S3 (resumes+screenshots):  ~$0.50/mo
    CloudWatch Logs:           ~$1.50/mo
    Elastic IP:                ~$0.00/mo (free when attached)
    Data Transfer:             ~$0.50/mo
    ─────────────────────────────────
    Total:                     ~$6–8/mo
    External (Neon, Upstash, Vercel): $0/mo (free tiers)
  EOT
}
