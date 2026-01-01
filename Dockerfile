# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Install goose for migrations
RUN go install github.com/pressly/goose/v3/cmd/goose@latest

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/notifd ./cmd/notifd

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/notifd /app/notifd
COPY --from=builder /go/bin/goose /usr/local/bin/goose
COPY db/migrations /app/migrations

# Entrypoint script
COPY <<'EOF' /app/entrypoint.sh
#!/bin/sh
set -e
echo "Running database migrations..."
goose -dir /app/migrations postgres "$DATABASE_URL" up
echo "Migrations complete. Starting server..."
exec /app/notifd
EOF
RUN chmod +x /app/entrypoint.sh

EXPOSE 8080

CMD ["/app/entrypoint.sh"]
