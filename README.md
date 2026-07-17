# Home Finance

Family budget tracker — Go API + React SPA + PostgreSQL. Replaces an Excel-based workflow for tracking monthly income, expenses, and savings pot allocations.

## Features

- Monthly income & expense tracking (budget lines + itemized transactions, dated)
- Budget-vs-actual progress bars per tracked category
- Year dashboard: income/expense/surplus chart, savings pot balances
- Savings pots overview with full ledger (deposits, withdrawals, adjustments, opening balance) and balance-trend chart
- Savings goals: optional target amount + target date per pot, with progress bars
- Pot % allocation on month close; carryover pot always absorbs whatever isn't allocated to another pot (percentage is computed, not entered) and flows to next month as income
- Explicit year registry (Settings > Years) — no hardcoded year range
- Template system: income sources and categories marked for auto-copy to new months
- Excel export per year or selected months (income, expenses, transactions, year overview, pot balances)
- Excel import: legacy Fam_Finance workbook migration or re-importing the app's own export (auto-detected), with optional wipe/reset behind a password confirm
- Role-based access (Admin/User): admins manage structure (categories, sources, pots, years, users) and destructive month/year actions; users handle day-to-day entries. Bootstrapped via `INITIAL_ADMIN_EMAILS`
- Profile: display name + avatar upload (S3-compatible object storage, optional)
- Audit log for close, reopen, delete, pot entry, import, and role-change actions — paginated/filterable viewer in Settings (admin-only), with optional periodic S3 export (any interval, seconds to hours)
- Collapsible sidebar (icon rail by default; peek or pin open)
- Dark/light/system theme toggle
- Dutch + English UI
- OIDC auth (Authentik in prod, Dex in dev)
- Installable PWA (offline-capable static assets, network-first API)
- Mobile-native UI below tablet width: bottom tab bar, swipeable lists, bottom-sheet editing — not a shrunk desktop layout

## Stack

| Layer | Technology |
|---|---|
| Backend | Go, chi, pgx/v5, goose migrations |
| Frontend | React 19, Vite, Mantine v9, TanStack Query v5 |
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
| `INITIAL_ADMIN_EMAILS` | — | Comma-separated emails granted `role=admin` on first login (upsert only — doesn't override a role changed later in Settings > Users) |
| `STATIC_DIR` | — | Path to frontend dist (empty = no SPA serving) |
| `SESSION_SECURE` | `true` | `Secure` flag on the session cookie — `false` only for plain-HTTP local dev |
| `S3_ENDPOINT` | — | S3-compatible object storage endpoint (avatars + audit-log export). Empty disables both |
| `S3_BUCKET` | — | Bucket name |
| `S3_ACCESS_KEY_ID` | — | Access key |
| `S3_SECRET_ACCESS_KEY` | — | Secret key |
| `S3_REGION` | — | Region (optional, provider-dependent) |
| `S3_USE_SSL` | `true` | Use HTTPS to reach the S3 endpoint |
| `AUDIT_EXPORT_INTERVAL` | — | Go duration string (`"10s"`, `"1h"`, `"12h"`) for periodic audit-log export to S3. Empty disables it; requires S3 to also be configured |

## API overview

All endpoints under `/api`, session-auth via cookie, amounts in cents.

Endpoints marked **(admin)** are gated by `requireAdmin` — read endpoints stay open to any
authenticated user so a `role=user` account can still use the app day-to-day.

```
GET   /api/me
PATCH /api/me                  (own display name)
POST  /api/me/avatar           (multipart; 503 if S3 not configured)
GET   /api/users/:id/avatar    (auth-gated proxy — object storage is never public)
GET   /api/users                                (admin)
PATCH /api/users/:id/role      (body: {"role": "admin"|"user"})  (admin — can't demote the last admin)

GET  /api/years
POST /api/years                                 (admin)
GET  /api/years/:year/summary
POST /api/years/:year/lock     (body: {"password": "..."})  (admin)
POST /api/years/:year/unlock   (body: {"password": "..."})  (admin)

GET  /api/periods?year=
POST /api/periods
GET  /api/periods/:id/overview
POST /api/periods/:id/close                     (admin)
POST /api/periods/:id/reopen                    (admin)
DEL  /api/periods/:id          (body: {"password": "..."})  (admin)

CRUD /api/income-entries
CRUD /api/budget-lines
CRUD /api/transactions
PUT  /api/periods/:id/splits   (carryover pot's percentage is always server-computed)

GET  /api/categories
POST/PUT /api/categories, /api/categories/:id, /api/categories/:id/archive     (admin)
GET  /api/income-sources
POST/PUT /api/income-sources/*                                                  (admin)
GET  /api/pots
POST/PUT /api/pots, /api/pots/:id, /api/pots/:id/archive                        (admin)
GET  /api/pots/balances
GET  /api/pots/:id/ledger
POST /api/pots/:id/entries
DEL  /api/pot-entries/:id      (manual entries only — allocation/carryover_out are lifecycle-managed)

GET  /api/export/years/:year?months=1,2,3        (whole year if months omitted)
POST /api/import/xlsx          (multipart: file, year, wipe, resetMaster, closeThrough, password)
                                (legacy Fam_Finance workbook or this app's own export, auto-detected;
                                 wipe/resetMaster require password and are audit-logged)

GET  /api/audit-log?limit=&before=&action=&entityType=&userEmail=&from=&to=     (admin)
GET  /api/docs                                                                   (admin)
GET  /api/openapi.yaml                                                           (admin)
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
  --set env.INITIAL_ADMIN_EMAILS=you@example.com \
  --set oidc.existingSecret=home-finance-oidc

# One-off Excel import
kubectl run importer --rm -it --restart=Never \
  --image=ghcr.io/wouterdamman/home-finance:latest \
  --env="DATABASE_URL=$(kubectl get secret home-finance-pg-app -o jsonpath='{.data.uri}' | base64 -d)" \
  -- /app/importer --xlsx /tmp/Finance.xlsx --year 2026 --wipe --close-through 6
```

### Object storage (avatars + audit-log export)

Both optional and off by default. Enable by pointing `s3.existingSecret` at a Secret with
`ENDPOINT` / `ACCESS_KEY_ID` / `SECRET_ACCESS_KEY` keys (any S3-compatible provider):

```bash
kubectl create secret generic home-finance-s3 \
  --from-literal=ENDPOINT=s3.eu-central-1.example.com \
  --from-literal=ACCESS_KEY_ID=... \
  --from-literal=SECRET_ACCESS_KEY=...

helm upgrade --install home-finance deploy/helm/home-finance \
  --set s3.enabled=true \
  --set s3.existingSecret=home-finance-s3 \
  --set s3.bucket=home-finance \
  --set auditExport.interval=1h \
  # ... plus the oidc/ingress flags above
```

`auditExport.interval` is a Go duration string (`10s`, `5m`, `1h`, `12h`) and runs as a goroutine
inside the app pod, not a separate CronJob — that's why sub-minute intervals work. Leave it unset
to keep S3 enabled for avatars only.

## Database migrations

Migrations live in `backend/migrations/` and run automatically on startup when `AUTO_MIGRATE=true`. To run manually:

```bash
make migrate
```
