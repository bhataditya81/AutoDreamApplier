[
  {
    "name": "job-discovery",
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
      { "name": "JOB_DISCOVERY_PORT", "value": "${port}" },
      { "name": "ENVIRONMENT",        "value": "production" },
      { "name": "LOG_LEVEL",          "value": "info" },
      { "name": "AWS_REGION",         "value": "${region}" },
      { "name": "LOG_FORMAT",         "value": "json" },
      { "name": "RUN_ONCE",           "value": "true" }
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
        "name": "S3_BUCKET",
        "valueFrom": "${ssm_prefix}/s3_bucket"
      }
    ],
    "logConfiguration": {
      "logDriver": "awslogs",
      "options": {
        "awslogs-group":         "${log_group}",
        "awslogs-region":        "${region}",
        "awslogs-stream-prefix": "job-discovery"
      }
    },
    "readonlyRootFilesystem": false,
    "stopTimeout": 30
  }
]
