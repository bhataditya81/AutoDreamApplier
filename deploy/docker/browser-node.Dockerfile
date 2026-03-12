# Browser Node Dockerfile - Playwright + Chromium for ATS automation
FROM golang:1.25-alpine AS builder

# Retry apk up to 3 times to handle transient Alpine mirror errors
RUN apk add --no-cache git ca-certificates tzdata || \
    (sleep 10 && apk add --no-cache git ca-certificates tzdata) || \
    (sleep 30 && apk add --no-cache git ca-certificates tzdata)

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /bin/browser-pool \
    ./cmd/browser-pool

# Runtime stage - uses Playwright base image with Chromium (Debian-based, no apk needed)
FROM mcr.microsoft.com/playwright:v1.42.1-jammy

RUN apt-get update && \
    apt-get install -y --no-install-recommends curl && \
    rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN groupadd -r appgroup && useradd -r -g appgroup -m appuser

WORKDIR /app

# Copy Go binary from builder
COPY --from=builder /bin/browser-pool .

# Set Playwright browser path
ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright

# Switch to non-root user
USER appuser

EXPOSE 8085

HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8085/health || exit 1

ENTRYPOINT ["./browser-pool"]
