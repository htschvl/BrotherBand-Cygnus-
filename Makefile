.DEFAULT_GOAL := help

BINARY := bin/brotherband-api
MODULE := github.com/htschvl/BrotherBand-Cygnus-
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: help
help: ## Show this help.
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ─── Build ───────────────────────────────────────────────────────────────────
.PHONY: build
build: ## Compile the API binary into bin/.
	@mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/api

.PHONY: run
run: ## Run the API in foreground (auto-loads .env if present).
	set -a; [ -f .env ] && . ./.env; set +a; go run -ldflags "$(LDFLAGS)" ./cmd/api

# ─── Quality ─────────────────────────────────────────────────────────────────
.PHONY: tidy
tidy: ## Tidy go.mod / go.sum.
	go mod tidy

.PHONY: vet
vet: ## go vet across all packages.
	go vet ./...

.PHONY: fmt
fmt: ## gofmt + goimports.
	gofmt -s -w .

.PHONY: lint
lint: ## Static analysis (requires staticcheck).
	@which staticcheck > /dev/null 2>&1 || (echo "Install: go install honnef.co/go/tools/cmd/staticcheck@latest" && exit 1)
	staticcheck ./...

# ─── Tests ───────────────────────────────────────────────────────────────────
.PHONY: test
test: ## Run unit + handler tests (no Docker needed).
	go test -race -count=1 ./...

.PHONY: test-cover
test-cover: ## Run tests with coverage.
	go test -race -count=1 -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

.PHONY: test-integration
test-integration: ## Run repository tests against ephemeral Postgres (Docker required).
	go test -race -count=1 -tags=integration ./internal/adapter/persistence/...

# ─── Code generation ────────────────────────────────────────────────────────
.PHONY: sqlc
sqlc: ## Regenerate the sqlc-managed query code.
	@which sqlc > /dev/null 2>&1 || (echo "Install: brew install sqlc OR go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest" && exit 1)
	sqlc -f internal/adapter/persistence/postgres/sqlc.yaml generate

# ─── Local infrastructure ───────────────────────────────────────────────────
.PHONY: dev-up
dev-up: ## Start local Postgres + MinIO via docker compose.
	./scripts/dev-up.sh up

.PHONY: dev-down
dev-down: ## Stop local Postgres + MinIO.
	./scripts/dev-up.sh down

# ─── Migrations ─────────────────────────────────────────────────────────────
.PHONY: migrate
migrate: ## Apply pending migrations against $DATABASE_URL (requires goose CLI).
	@which goose > /dev/null 2>&1 || (echo "Install: go install github.com/pressly/goose/v3/cmd/goose@latest" && exit 1)
	goose -dir internal/infrastructure/postgres/migrations postgres "$$DATABASE_URL" up
