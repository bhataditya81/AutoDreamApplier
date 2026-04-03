# BASE_IMAGE is the pre-built deps image (ECR :deps tag).
# In CI this is set to the ECR deps image so pip install is skipped entirely.
# For local builds: docker build --build-arg BASE_IMAGE=python:3.12-slim
#   (will be slow — run Dockerfile.deps first for fast local iteration)
ARG BASE_IMAGE=python:3.12-slim
FROM ${BASE_IMAGE}

WORKDIR /app

COPY . .

EXPOSE 8085

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8085"]
