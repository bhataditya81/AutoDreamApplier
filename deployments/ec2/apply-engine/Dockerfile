FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/root/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
    go build -trimpath -ldflags="-s -w" \
    -o apply-engine ./cmd/apply-engine/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /app/apply-engine /apply-engine
EXPOSE 8084
ENTRYPOINT ["/apply-engine"]
