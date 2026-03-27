[
  {
    "name": "ai-service",
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
      { "name": "PORT",        "value": "${port}" },
      { "name": "ENVIRONMENT", "value": "production" },
      { "name": "LOG_LEVEL",   "value": "info" },
      { "name": "AWS_REGION",  "value": "${region}" }
    ],
    "secrets": [
      {
        "name": "ANTHROPIC_API_KEY",
        "valueFrom": "${ssm_prefix}/anthropic_api_key"
      },
      {
        "name": "DATABASE_URL",
        "valueFrom": "${ssm_prefix}/database_url"
      },
      {
        "name": "REDIS_URL",
        "valueFrom": "${ssm_prefix}/redis_url"
      },
      {
        "name": "S3_BUCKET",
        "valueFrom": "${ssm_prefix}/s3_bucket"
      }
    ],
    "logConfiguration": {
      "logDriver": "awslogs",
      "options": {
        "awslogs-group":         "${log_group}",
        "awslogs-region":        "${region}",
        "awslogs-stream-prefix": "ai-service"
      }
    },
    "healthCheck": {
      "command":     ["CMD-SHELL", "curl -sf http://localhost:${port}/health || exit 1"],
      "interval":    30,
      "timeout":     10,
      "retries":     3,
      "startPeriod": 60
    },
    "readonlyRootFilesystem": false,
    "stopTimeout": 30
  }
]
