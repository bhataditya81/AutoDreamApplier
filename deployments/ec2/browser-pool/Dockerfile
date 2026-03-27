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

# Use a Debian slim base and install Playwright deps
FROM debian:bookworm-slim

# Install Chromium and its dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    chromium \
    chromium-driver \
    fonts-liberation \
    libnss3 \
    libatk-bridge2.0-0 \
    libdrm2 \
    libxkbcommon0 \
    libxcomposite1 \
    libxdamage1 \
    libxfixes3 \
    libxrandr2 \
    libgbm1 \
    libasound2 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN groupadd -r appuser && useradd -r -g appuser appuser

COPY --from=builder /app/browser-pool /browser-pool

# Playwright-go looks for PLAYWRIGHT_BROWSERS_PATH
ENV PLAYWRIGHT_BROWSERS_PATH=/usr/bin
ENV CHROMIUM_PATH=/usr/bin/chromium

USER appuser
EXPOSE 9222
ENTRYPOINT ["/browser-pool"]
