.PHONY: build run test clean migrate migrate-status migrate-down

# Build configuration
BINARY_NAME=otis
DB_PATH ?= ./db/otis.db
MIGRATIONS_DIR=./aggregator/migrations

# Build the application
build:
	go build -o $(BINARY_NAME) .

# Run the application (migrations run automatically on startup)
run: build
	./$(BINARY_NAME)

# Run tests
test:
	go test ./aggregator/...

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)

# Install goose CLI for manual migration management
install-goose:
	go install github.com/pressly/goose/v3/cmd/goose@latest

# Run migrations manually (useful for CI/CD or manual intervention)
migrate:
	@mkdir -p $(dir $(DB_PATH))
	goose -dir $(MIGRATIONS_DIR) sqlite3 $(DB_PATH) up

# Check migration status
migrate-status:
	goose -dir $(MIGRATIONS_DIR) sqlite3 $(DB_PATH) status

# Rollback last migration
migrate-down:
	goose -dir $(MIGRATIONS_DIR) sqlite3 $(DB_PATH) down

# Create a new migration file
migrate-create:
	@read -p "Migration name: " name; \
	goose -dir $(MIGRATIONS_DIR) create $$name sql

# Development: run with live reload (requires air: go install github.com/air-verse/air@latest)
dev:
	air

# Format code
fmt:
	go fmt ./...

# Lint code (requires golangci-lint)
lint:
	golangci-lint run
