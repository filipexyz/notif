.PHONY: build run test generate dev dev-down clean seed

# Build the binary
build:
	go build -o bin/notif ./cmd/notif

# Run locally (requires dev services running)
run: build
	DATABASE_URL="postgres://notif:notif_dev@localhost:5432/notif?sslmode=disable" \
	NATS_URL="nats://localhost:4222" \
	LOG_LEVEL=debug \
	LOG_FORMAT=text \
	./bin/notif

# Run unit tests
test:
	go test -v -race ./internal/...

# Run e2e tests (requires Docker)
test-e2e:
	go test -v -race -timeout 5m ./tests/e2e/...

# Run all tests
test-all:
	go test -v -race -timeout 5m ./...

# Generate sqlc code
generate:
	cd db && ~/go/bin/sqlc generate

# Start local dev environment (NATS + Postgres only)
dev:
	docker compose up -d nats postgres
	@echo "Waiting for services..."
	@sleep 3
	@echo "NATS: http://localhost:8222"
	@echo "Postgres: localhost:5432"
	@echo ""
	@echo "Run 'make seed' to create test API key"
	@echo "Run 'make run' to start the server"

# Start all services including notif
up:
	docker compose up -d --build

# Stop dev environment
dev-down:
	docker compose down

# Clean build artifacts and volumes
clean:
	rm -rf bin/
	docker compose down -v

# Seed test data
seed:
	PGPASSWORD=notif_dev psql -h localhost -U notif -d notif -f scripts/seed.sql

# Run with live reload (requires: go install github.com/air-verse/air@latest)
watch:
	air

# Show test key for curl commands
key:
	@echo "Test API Key: nsh_test_abcdefghij12345678901234"
