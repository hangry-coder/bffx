# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Cache Go module downloads before copying sources
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

    # Cache Go build artifacts
    RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /bffx-server cmd/api/main.go

# Production stage
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata wget su-exec
RUN addgroup -g 1001 bffx && adduser -u 1001 -G bffx -s /bin/sh -D bffx

WORKDIR /app
RUN chown bffx:bffx /app
COPY --from=builder --chown=bffx:bffx /bffx-server /app/bffx-server
COPY --from=builder --chown=bffx:bffx /app/.bffx /app/.bffx
COPY --from=builder --chown=bffx:bffx /app/bffx /app/bffx
COPY --from=builder --chown=bffx:bffx /app/internal /app/internal
COPY --from=builder --chown=bffx:bffx /app/assets /app/assets
COPY --from=builder --chown=bffx:bffx /app/config /app/config
COPY --from=builder --chown=bffx:bffx /app/db /app/db
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=5s --start-period=60s --retries=8 \
  CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["/app/bffx-server"]
