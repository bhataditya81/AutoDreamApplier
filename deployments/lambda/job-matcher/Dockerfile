FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/root/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath -ldflags="-s -w" \
    -o bootstrap ./deployments/lambda/job-matcher/

FROM public.ecr.aws/lambda/provided:al2023
COPY --from=builder /app/bootstrap /var/task/bootstrap
ENTRYPOINT ["/var/task/bootstrap"]
