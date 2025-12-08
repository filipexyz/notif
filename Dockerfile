# Build stage
FROM rust:1.83-slim-bookworm AS builder

WORKDIR /app

# Install build dependencies
RUN apt-get update && apt-get install -y \
    pkg-config \
    libssl-dev \
    && rm -rf /var/lib/apt/lists/*

# Copy workspace files
COPY Cargo.toml Cargo.lock ./
COPY crates ./crates

# Build release binary (only CLI, not UI)
RUN cargo build --release -p notif

# Runtime stage
FROM debian:bookworm-slim

WORKDIR /app

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy binary from builder
COPY --from=builder /app/target/release/notif /usr/local/bin/notif

# Create data directory
RUN mkdir -p /data

# Environment variables
ENV NOTIF_SERVER_HOST=0.0.0.0
ENV NOTIF_SERVER_PORT=8787
ENV NOTIF_SERVER_DB=/data/notif.db

EXPOSE 8787

# Run server (use shell form to expand env vars)
CMD notif server --host ${NOTIF_SERVER_HOST:-0.0.0.0} --port ${NOTIF_SERVER_PORT:-8787}
