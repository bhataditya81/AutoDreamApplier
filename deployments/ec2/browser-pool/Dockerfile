# BASE_IMAGE is the pre-built Chromium base image (ECR :base tag).
# In CI this is set to the ECR base image so apt-get install is skipped entirely.
# For local builds: docker build --build-arg BASE_IMAGE=debian:bookworm-slim
#   (will be slow — run Dockerfile.base first for fast local iteration)
ARG BASE_IMAGE=debian:bookworm-slim

FROM golang:1.25-bookworm AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/root/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
    go build -trimpath -ldflags="-s -w" \
    -o browser-pool ./cmd/browser-pool/

FROM ${BASE_IMAGE}

COPY --from=builder /app/browser-pool /browser-pool

# Playwright-go looks for PLAYWRIGHT_BROWSERS_PATH
ENV PLAYWRIGHT_BROWSERS_PATH=/usr/bin
ENV CHROMIUM_PATH=/usr/bin/chromium

USER appuser
EXPOSE 9222
ENTRYPOINT ["/browser-pool"]
