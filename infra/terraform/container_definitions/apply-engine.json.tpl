[
  {
    "name": "apply-engine",
    "image": "${image_url}",
    "essential": true,
    "portMappings": [
      {
        "containerPort": ${port},
        "hostPort": ${port},
        "protocol": "tcp"
      }
    ],
    "environment": [
      { "name": "APPLY_ENGINE_PORT", "value": "${port}" },
      { "name": "ENVIRONMENT",       "value": "production" },
      { "name": "LOG_LEVEL",         "value": "info" },
      { "name": "AWS_REGION",        "value": "${region}" },
      { "name": "LOG_FORMAT",        "value": "json" }
    ],
    "secrets": [
      {
        "name": "DATABASE_URL",
        "valueFrom": "${ssm_prefix}/database_url"
      },
      {
        "name": "REDIS_URL",
        "valueFrom": "${ssm_prefix}/redis_url"
      },
      {
        "name": "JWT_SECRET",
        "valueFrom": "${ssm_prefix}/jwt_secret"
      },
      {
        "name": "S3_BUCKET",
        "valueFrom": "${ssm_prefix}/s3_bucket"
      },
      {
        "name": "AI_SERVICE_URL",
        "valueFrom": "${ssm_prefix}/ai_service_url"
      },
      {
        "name": "ANTHROPIC_API_KEY",
        "valueFrom": "${ssm_prefix}/anthropic_api_key"
      }
    ],
    "logConfiguration": {
      "logDriver": "awslogs",
      "options": {
        "awslogs-group":         "${log_group}",
        "awslogs-region":        "${region}",
        "awslogs-stream-prefix": "apply-engine"
      }
    },
    "healthCheck": {
      "command":     ["CMD-SHELL", "wget -qO- http://localhost:${port}/health || exit 1"],
      "interval":    30,
      "timeout":     5,
      "retries":     3,
      "startPeriod": 30
    },
    "readonlyRootFilesystem": false,
    "stopTimeout": 60
  }
]
