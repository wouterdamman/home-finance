.PHONY: dev db-up db-down migrate sqlc test test-integration import build-image lint e2e generate

# ── Local dev ────────────────────────────────────────────────────
dev:
	docker compose up -d db dex
	cd frontend && npm install --silent && npm run dev &
	cd backend && go run ./cmd/server

db-up:
	docker compose up -d db dex

db-down:
	docker compose down

# ── Backend ──────────────────────────────────────────────────────
migrate:
	cd backend && go run ./cmd/server --migrate-only

sqlc:
	cd backend && sqlc generate

generate: sqlc

test:
	cd backend && go test ./...
	cd frontend && npx vitest run

test-integration:
	cd backend && go test -tags integration ./...

import:
	cd backend && go run ./cmd/importer $(ARGS)

# ── Frontend ─────────────────────────────────────────────────────
frontend-build:
	cd frontend && npm install && npm run build

# ── Docker ───────────────────────────────────────────────────────
build-image:
	docker build -f deploy/docker/Dockerfile -t home-finance:latest .

# ── Lint ─────────────────────────────────────────────────────────
lint:
	cd backend && go vet ./... && golangci-lint run ./...
	cd frontend && npm run lint

# ── E2E ──────────────────────────────────────────────────────────
e2e:
	cd frontend && npx playwright test
