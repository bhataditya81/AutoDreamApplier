FROM golang:1.25-alpine

RUN apk add --no-cache git ca-certificates tzdata curl || \
    (sleep 10 && apk add --no-cache git ca-certificates tzdata curl) || \
    (sleep 30 && apk add --no-cache git ca-certificates tzdata curl)

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

EXPOSE 8084

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8084/health || exit 1

ENTRYPOINT ["go", "run", "./cmd/apply-engine"]
