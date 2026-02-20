.PHONY: help build run test clean docker-up docker-down migrate-up migrate-down lint build-prod docker-prod

# Default target
help:
	@echo "Available commands:"
	@echo ""
	@echo "Development:"
	@echo "  make build          - Build the application"
	@echo "  make run            - Run the application"
	@echo "  make dev            - Run with hot reload (requires air)"
	@echo "  make test           - Run tests"
	@echo "  make test-coverage  - Run tests with coverage"
	@echo "  make docker-up      - Start Docker containers"
	@echo "  make docker-down    - Stop Docker containers"
	@echo "  make docker-logs    - View Docker logs"
	@echo ""
	@echo "Production:"
	@echo "  make build-prod     - Build production binary"
	@echo "  make docker-prod    - Build production Docker image"
	@echo "  make deploy-prod    - Deploy to production"
	@echo ""
	@echo "Database:"
	@echo "  make migrate-up     - Run database migrations"
	@echo "  make migrate-down   - Rollback database migrations"
	@echo "  make migrate-create - Create a new migration (name=migration_name)"
	@echo "  make migrate-status - Check migration status"
	@echo "  make seed           - Seed database with sample data"
	@echo "  make seed-demo      - Seed database with comprehensive demo data"
	@echo "  make db-reset       - Remove all data from database (keeps schema)"
	@echo "  make seed-sell-categories - Seed sell_categories only (no data wipe)"
	@echo ""
	@echo "Code Quality:"
	@echo "  make lint           - Run linter"
	@echo "  make fmt            - Format code"
	@echo "  make security-scan  - Run security scanner"
	@echo ""
	@echo "Utilities:"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make deps           - Download dependencies"
	@echo "  make install-tools  - Install development tools"

# Build the application
build:
	@echo "Building application..."
	go build -o bin/server cmd/server/main.go

# Run the application
run:
	@echo "Running application..."
	go run cmd/server/main.go

# Run tests
test:
	@echo "Running tests..."
	go test -v -race ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean

# Start Docker containers
docker-up:
	@echo "Starting Docker containers..."
	docker-compose up -d

# Stop Docker containers
docker-down:
	@echo "Stopping Docker containers..."
	docker-compose down

# View Docker logs
docker-logs:
	docker-compose logs -f

# Rebuild and restart Docker containers
docker-restart:
	@echo "Restarting Docker containers..."
	docker-compose down
	docker-compose up -d --build

# Run database migrations up
migrate-up:
	@echo "Running migrations..."
	go run cmd/migrate/main.go up

# Run database migrations down
migrate-down:
	@echo "Rolling back migrations..."
	go run cmd/migrate/main.go down

# Create a new migration
migrate-create:
	@if [ -z "$(name)" ]; then \
		echo "Error: migration name required. Usage: make migrate-create name=migration_name"; \
		exit 1; \
	fi
	@echo "Creating migration: $(name)"
	@timestamp=$$(date +%Y%m%d%H%M%S); \
	touch migrations/$${timestamp}_$(name).up.sql; \
	touch migrations/$${timestamp}_$(name).down.sql; \
	echo "Created migrations/$${timestamp}_$(name).up.sql"; \
	echo "Created migrations/$${timestamp}_$(name).down.sql"

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	gofmt -s -w .

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/swaggo/swag/cmd/swag@latest

# Generate Swagger documentation
swagger:
	@echo "Generating Swagger documentation..."
	swag init -g cmd/server/main.go -o ./docs

# Run in development mode with hot reload (requires air)
dev:
	@echo "Running in development mode..."
	air

# Install air for hot reload
install-air:
	@echo "Installing air..."
	go install github.com/cosmtrek/air@latest

# ============================================================================
# Production Commands
# ============================================================================

# Build production binary with optimizations
build-prod:
	@echo "Building production binary..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-a -installsuffix cgo \
		-ldflags="-w -s" \
		-o bin/server-linux-amd64 \
		cmd/server/main.go
	@echo "Production binary created: bin/server-linux-amd64"

# Build production Docker image
docker-prod:
	@echo "Building production Docker image..."
	docker build -f Dockerfile.prod -t hamsaya-backend:latest .
	@echo "Docker image built: hamsaya-backend:latest"

# Tag and push Docker image
docker-push:
	@if [ -z "$(REGISTRY)" ]; then \
		echo "Error: REGISTRY required. Usage: make docker-push REGISTRY=your-registry.com"; \
		exit 1; \
	fi
	@echo "Tagging and pushing image to $(REGISTRY)..."
	docker tag hamsaya-backend:latest $(REGISTRY)/hamsaya-backend:latest
	docker tag hamsaya-backend:latest $(REGISTRY)/hamsaya-backend:$$(git describe --tags --always)
	docker push $(REGISTRY)/hamsaya-backend:latest
	docker push $(REGISTRY)/hamsaya-backend:$$(git describe --tags --always)

# Deploy to production (Docker Compose)
deploy-prod:
	@echo "Deploying to production..."
	docker-compose -f docker-compose.prod.yml pull
	docker-compose -f docker-compose.prod.yml up -d
	@echo "Production deployment complete"

# Check migration status
migrate-status:
	@echo "Checking migration status..."
	go run cmd/migrate/main.go status

# Seed database with sample data
seed:
	@echo "Seeding database with sample data..."
	go run cmd/seed/main.go
	@echo "Database seeding complete"

# Seed database with comprehensive demo data
seed-demo:
	@echo "Seeding database with comprehensive demo data..."
	go run cmd/seed-demo/main.go
	@echo "Demo database seeding complete"

# Remove all data from database (keeps schema)
db-reset:
	@echo "Removing all data from database..."
	go run cmd/db-reset/main.go
	@echo "Database reset complete"

# Backfill notifications from existing post_likes and post_comments (one-time after notification fix)
backfill-notifications:
	@echo "Backfilling notifications from existing likes and comments..."
	go run cmd/backfill-notifications/main.go
	@echo "Backfill complete"

# Seed sell_categories only (no data wipe). Use when categories are empty.
seed-sell-categories:
	@echo "Seeding sell categories..."
	SEED_CATEGORIES_ONLY=1 go run cmd/db-reset/main.go
	@echo "Sell categories seeded"

# Run security scanner (gosec)
security-scan:
	@echo "Running security scanner..."
	@command -v gosec >/dev/null 2>&1 || { \
		echo "Installing gosec..."; \
		go install github.com/securego/gosec/v2/cmd/gosec@latest; \
	}
	gosec -fmt=json -out=security-report.json ./...
	@echo "Security scan complete. Report: security-report.json"

# Check for vulnerabilities in dependencies
vuln-check:
	@echo "Checking for vulnerabilities..."
	@command -v govulncheck >/dev/null 2>&1 || { \
		echo "Installing govulncheck..."; \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
	}
	govulncheck ./...

# Database backup (production)
db-backup:
	@if [ -z "$(DB_URL)" ]; then \
		echo "Error: DB_URL required. Usage: make db-backup DB_URL=postgresql://..."; \
		exit 1; \
	fi
	@echo "Creating database backup..."
	@timestamp=$$(date +%Y%m%d_%H%M%S); \
	pg_dump -Fc "$(DB_URL)" -f backups/backup_$${timestamp}.dump
	@echo "Backup created: backups/backup_$${timestamp}.dump"

# Database restore (production)
db-restore:
	@if [ -z "$(DB_URL)" ] || [ -z "$(BACKUP_FILE)" ]; then \
		echo "Error: DB_URL and BACKUP_FILE required."; \
		echo "Usage: make db-restore DB_URL=postgresql://... BACKUP_FILE=backup.dump"; \
		exit 1; \
	fi
	@echo "Restoring database from $(BACKUP_FILE)..."
	pg_restore -Fc -d "$(DB_URL)" -c $(BACKUP_FILE)
	@echo "Database restored"

# Health check
health-check:
	@echo "Checking application health..."
	@curl -f http://localhost:8080/health/ready || { \
		echo "Health check failed!"; \
		exit 1; \
	}
	@echo "Application is healthy"

# Benchmark tests
benchmark:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem -run=^# ./...

# Generate all documentation
docs:
	@echo "Generating documentation..."
	@command -v swag >/dev/null 2>&1 || { \
		echo "Installing swag..."; \
		go install github.com/swaggo/swag/cmd/swag@latest; \
	}
	swag init -g cmd/server/main.go -o ./docs
	@echo "API documentation generated in ./docs"

# CI/CD commands
ci-test:
	@echo "Running CI tests..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

ci-lint:
	@echo "Running CI linter..."
	golangci-lint run --timeout=5m ./...

ci-build:
	@echo "Running CI build..."
	CGO_ENABLED=0 go build -o bin/server cmd/server/main.go

# Complete CI pipeline
ci: ci-lint ci-test ci-build
	@echo "CI pipeline complete"
