# Home Finance

Family budget tracker — Go API + React SPA + PostgreSQL. Replaces an Excel-based workflow for tracking monthly income, expenses, and savings pot allocations.

## Features

- Monthly income & expense tracking (budget lines + itemized transactions)
- Savings pots with % allocation on month close; carryover pot flows to next month as income
- Template system: income sources and categories marked for auto-copy to new months
- Audit log for close, reopen, delete, and pot entry actions
- Dark/light/system theme toggle
- Dutch + English UI
- OIDC auth (Authentik in prod, Dex in dev)

## Stack

| Layer | Technology |
|---|---|
| Backend | Go, chi, pgx/v5, goose migrations |
| Frontend | React 18, Vite, Mantine v7, TanStack Query v5 |
| Database | PostgreSQL (CloudNativePG in prod) |
| Auth | OIDC/BFF — HttpOnly session cookie |
| Deploy | Kubernetes + Helm, GHCR image |

## Local dev

**Prerequisites:** Docker, Go 1.25+, Node 22+

```bash
cp .env.example .env
make dev          # starts postgres + dex, backend on :8080, frontend on :5173
```

The frontend proxies `/api` and `/auth` to `:8080`. Open http://localhost:5173.

Dev login: any user (DEV_FAKE_AUTH=true bypasses OIDC).

### Useful commands

```bash
make test                    # unit tests (backend + frontend)
make test-integration        # integration tests (needs running postgres)
make lint                    # go vet + golangci-lint + eslint
make build-image             # build docker image locally

# Import 2026 Excel data
make import ARGS="--xlsx /path/to/Fam_Finance_2026.xlsx --year 2026 --wipe --close-through 6"
```

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | — | PostgreSQL connection string |
| `PORT` | `8080` | HTTP listen port |
| `AUTO_MIGRATE` | `false` | Run goose migrations on startup |
| `DEV_FAKE_AUTH` | `false` | Skip OIDC, inject dev user |
| `DELETE_PASSWORD` | — | PIN required to delete a period |
| `OIDC_ISSUER_URL` | — | OIDC provider discovery URL |
| `OIDC_CLIENT_ID` | — | OIDC client ID |
| `OIDC_CLIENT_SECRET` | — | OIDC client secret |
| `OIDC_REDIRECT_URL` | — | OAuth callback URL |
| `ALLOWED_EMAILS` | — | Comma-separated allowlist (empty = all) |
| `STATIC_DIR` | — | Path to frontend dist (empty = no SPA serving) |

## API overview

All endpoints under `/api`, session-auth via cookie, amounts in cents.

```
GET  /api/me
GET  /api/years/:year/summary

GET  /api/periods?year=
POST /api/periods
GET  /api/periods/:id/overview
POST /api/periods/:id/close
POST /api/periods/:id/reopen
DEL  /api/periods/:id          (body: {"password": "..."})

CRUD /api/income-entries
CRUD /api/budget-lines
CRUD /api/transactions
PUT  /api/periods/:id/splits

CRUD /api/categories
CRUD /api/income-sources
CRUD /api/pots
GET  /api/pots/balances
GET  /api/pots/:id/ledger
POST /api/pots/:id/entries

GET  /api/audit-log
```

## Releasing

Push a `v*` tag → GitHub Actions builds a multi-arch image, pushes to `ghcr.io/wouterdamman/home-finance`, creates a GitHub release, and bumps the Helm chart version on `main`.

```bash
git tag v1.0.0
git push origin v1.0.0
```

## Kubernetes deploy

```bash
# Install / upgrade
helm upgrade --install home-finance deploy/helm/home-finance \
  --set oidc.issuerURL=https://auth.example.com/application/o/home-finance/ \
  --set oidc.clientID=home-finance \
  --set ingress.host=finance.example.com \
  --set-string env.DELETE_PASSWORD=changeme \
  --set oidc.existingSecret=home-finance-oidc

# One-off Excel import
kubectl run importer --rm -it --restart=Never \
  --image=ghcr.io/wouterdamman/home-finance:latest \
  --env="DATABASE_URL=$(kubectl get secret home-finance-pg-app -o jsonpath='{.data.uri}' | base64 -d)" \
  -- /app/importer --xlsx /tmp/Finance.xlsx --year 2026 --wipe --close-through 6
```

## Database migrations

Migrations live in `backend/migrations/` and run automatically on startup when `AUTO_MIGRATE=true`. To run manually:

```bash
make migrate
```
