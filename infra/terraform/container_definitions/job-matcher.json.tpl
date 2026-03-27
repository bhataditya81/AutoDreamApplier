[
  {
    "name": "job-matcher",
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
      { "name": "JOB_MATCHER_PORT", "value": "${port}" },
      { "name": "ENVIRONMENT",      "value": "production" },
      { "name": "LOG_LEVEL",        "value": "info" },
      { "name": "AWS_REGION",       "value": "${region}" },
      { "name": "LOG_FORMAT",       "value": "json" },
      { "name": "RUN_ONCE",         "value": "true" }
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
        "awslogs-stream-prefix": "job-matcher"
      }
    },
    "readonlyRootFilesystem": false,
    "stopTimeout": 30
  }
]
