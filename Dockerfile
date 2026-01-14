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

# Build server and CLI
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/notifd ./cmd/notifd
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/notif ./cmd/notif

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/notifd /app/notifd
COPY --from=builder /app/notif /app/notif
COPY --from=builder /go/bin/goose /usr/local/bin/goose
COPY db/migrations /app/migrations

# Set CLI path for web terminal
ENV CLI_BINARY_PATH=/app/notif

# Entrypoint script
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

EXPOSE 8080

CMD ["/app/entrypoint.sh"]
