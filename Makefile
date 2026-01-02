.PHONY: build build-cli run test generate dev dev-down clean seed migrate migrate-down migrate-status publish-ts publish-py publish-sdks

# Build the server binary
build:
	go build -o bin/notifd ./cmd/notifd

# Build the CLI binary
build-cli:
	go build -o bin/notif ./cmd/notif

# Build all binaries
build-all: build build-cli

# Run locally (requires dev services running)
# Load .env file if it exists (for CLERK_SECRET_KEY, etc.)
run: build
	@if [ -f .env ]; then \
		set -a && . ./.env && set +a && ./bin/notifd; \
	else \
		DATABASE_URL="postgres://notif:notif_dev@localhost:5432/notif?sslmode=disable" \
		NATS_URL="nats://localhost:4222" \
		LOG_LEVEL=debug \
		LOG_FORMAT=text \
		./bin/notifd; \
	fi

# Run unit tests
test:
	go test -v -race ./internal/...
	cd sdk/typescript && npm test
	cd sdk/python && pytest

# Run e2e tests (requires Docker)
test-e2e:
	go test -v -race -timeout 5m ./tests/e2e/...

# Run integration tests (requires: make dev && make seed && make run)
test-integration:
	go test -v -race -tags=integration ./tests/integration/...

# Run all tests (excludes integration tests that need a running server)
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

# Run database migrations (requires: go install github.com/pressly/goose/v3/cmd/goose@latest)
GOOSE := ~/go/bin/goose
DB_URL := postgres://notif:notif_dev@localhost:5432/notif?sslmode=disable

migrate:
	$(GOOSE) -dir db/migrations postgres "$(DB_URL)" up

migrate-down:
	$(GOOSE) -dir db/migrations postgres "$(DB_URL)" down

migrate-status:
	$(GOOSE) -dir db/migrations postgres "$(DB_URL)" status

# Seed test data
seed:
	PGPASSWORD=notif_dev psql -h localhost -U notif -d notif -f scripts/seed.sql

# Run with live reload (requires: go install github.com/air-verse/air@latest)
watch:
	air

# Show test key for curl commands
key:
	@echo "Test API Key: nsh_test_abcdefghij12345678901234"

# Publish TypeScript SDK to npm
publish-ts:
	cd sdk/typescript && npm run build && npm publish

# Publish Python SDK to PyPI
publish-py:
	cd sdk/python && python -m build && twine upload dist/*

# Publish all SDKs
publish-sdks: publish-ts publish-py
