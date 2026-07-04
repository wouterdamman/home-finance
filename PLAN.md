# Home-Finance App — One-Shot Build Plan

## Context

Wouter beheert gezinsfinanciën nu in Excel (`/Users/wouter/Documents/Finance/Fam_Finance_2026.xlsx`): per maand inkomsten invullen, uitgaven (vast budget of transactie-lijstjes per categorie), overschot ("Totaal over") procentueel verdelen over spaarpotjes; "Doorlopen"-potje wordt inkomst volgende maand. Doel: zelfde workflow als webapp, straks op Kubernetes met CloudNativePG. Dit is een one-shot build in lege dir `/Users/wouter/personal/home-finance`.

**Besliste keuzes (met user):** Go backend · React+TS+Vite SPA · OIDC via Authentik (prod), Dex als dev stand-in · Postgres (CNPG prod, container dev) · generiek/configureerbaar datamodel · one-time Excel-import · Helm chart · i18n NL+EN · docker compose lokaal.

## Tech stack

| Concern | Keuze |
|---|---|
| Geld | `BIGINT` integer cents overal; percentages `NUMERIC(5,2)`; formatting in frontend (`Intl.NumberFormat`) |
| Router | `go-chi/chi/v5` |
| DB | `jackc/pgx/v5` + `sqlc` (SQL-first, typed) |
| Migraties | `pressly/goose/v3`, embedded, auto-run bij start (flag `AUTO_MIGRATE`) |
| Config/logging | `caarlos0/env/v11` + `.env`; `log/slog` |
| OIDC | `coreos/go-oidc/v3` + `x/oauth2`; **BFF-patroon**: backend doet auth-code flow, HttpOnly session cookie (geen tokens in browser) |
| Sessions | `alexedwards/scs/v2` + pgxstore (sessions-tabel), TTL 7d |
| Dev IdP | Dex (`dexidp/dex`), static config + static users wouter/ilse |
| Prod serving | Eén container: Go serveert `/api` én SPA uit `frontend/dist` (SPA-fallback) → geen CORS |
| Frontend | React 18, react-router v6, TanStack Query v5, react-i18next, **Mantine v7** |
| Importer | Go CLI `cmd/importer`, leest xlsx via `qax-os/excelize/v2`, label-anchored parsing, idempotent (`--wipe`) |
| Images | Multi-stage node22 + go1.23 → `distroless/static-debian12:nonroot` |

## Repo layout

```
home-finance/
├── Makefile, docker-compose.yml, README.md, .env.example
├── backend/
│   ├── go.mod, sqlc.yaml
│   ├── cmd/server/main.go, cmd/importer/main.go
│   ├── migrations/           # goose SQL, embed.go
│   └── internal/
│       ├── config/  auth/  httpapi/  domain/  store/  importer/
├── frontend/
│   └── src/  api/ (client+hooks)  lib/money.ts  i18n/ (nl,en)  components/  pages/
└── deploy/
    ├── dex/config.yaml
    ├── docker/Dockerfile
    └── helm/home-finance/   # Chart, values, templates incl. cnpg-cluster.yaml
```

## Datamodel (migration 0001)

Principes:
- **Periods** = `(year, month)` rijen, status `open|closed` → multi-year.
- **Twee lagen**: masterdata (`income_sources`, `categories`, `pots` met defaults) + per-maand rijen (`income_entries`, `budget_lines`, `pot_splits`). Nieuwe maand = kopie vorige maand (of defaults).
- **Vast vs itemized**: `budget_lines.tracks_transactions`; effectief bedrag = SUM(transactions) indien true, anders `amount_cents`. "Extern Sparen" = gewone itemized categorie (Goud/Silver/… als transacties).
- **Potjes = ledger** (`pot_ledger`, signed bedragen); saldo = SUM. Ledger-rijen alleen gematerialiseerd bij **maand sluiten**; open maand toont projectie. Reopen = delete rijen met `source_period_id`. Handmatige opname/storting = ledger entry.
- **Carryover**: pot `kind='carryover'` (Doorlopen). Close maand N schrijft: allocaties alle potjes + negatieve `carryover_out` op Doorlopen + `income_entries` rij (type `carryover`) in N+1. Alles met `source_period_id=N`. Reopen geweigerd als N+1 al closed.
- **Afronding**: largest-remainder → SUM(allocaties) == surplus exact (unit-tested). Negatief surplus toegestaan (proportioneel negatief).
- Soft-delete via `archived_at`, volgorde via `sort_order`.

Tabellen (volledige DDL in plan-agent output, kern):

```sql
users(id, oidc_subject UNIQUE, email, display_name)
sessions(token PK, data, expiry)                      -- scs pgxstore
periods(id, year, month, status open|closed, closed_at, UNIQUE(year,month))
income_sources(id, name, default_amount_cents, sort_order, archived_at)
income_entries(id, period_id FK, source_id FK NULL, label, amount_cents,
               entry_type normal|carryover, source_period_id FK NULL, sort_order)
categories(id, name, default_amount_cents, is_itemized, sort_order, archived_at)
budget_lines(id, period_id FK, category_id FK NULL, label, amount_cents,
             tracks_transactions, sort_order, UNIQUE(period_id,category_id))
transactions(id, period_id FK, category_id FK, amount_cents, description, tx_date)
pots(id, name, kind normal|carryover, sort_order, archived_at)
     -- unique partial index: max 1 actieve carryover-pot
pot_splits(id, period_id FK, pot_id FK, percentage NUMERIC(5,2), UNIQUE(period_id,pot_id))
pot_ledger(id, pot_id FK, period_id FK, source_period_id FK,
           entry_type allocation|carryover_out|withdrawal|deposit|adjustment|opening_balance,
           amount_cents signed, description, entry_date)
```

**Close-algoritme** (`domain/period.go`, één Tx): totalen berekenen → valideer splits == 100.00 (anders 409) → allocation-ledger rijen (largest-remainder) → carryover naar N+1 (auto-create N+1 indien nodig) → status closed.

## REST API (`/api`, session-auth, cents)

- Auth: `GET /auth/login`, `GET /auth/callback`, `POST /auth/logout`, `GET /api/me`, `GET /healthz|/readyz`
- Periods: `GET /api/periods?year=`, `POST /api/periods {year,month,copyFromPeriodId?}`, `GET /api/periods/{id}/overview` (hele maandscherm in 1 call: incomes, budgetLines incl. effectiveCents, surplus, splits+projectedCents), `POST /api/periods/{id}/close|reopen`
- Children (409 `period_closed` bij closed): CRUD incomes, budget-lines, transactions (`?categoryId=`), `PUT /api/periods/{id}/splits` (bulk replace)
- Masterdata: CRUD+archive+reorder voor categories, income-sources, pots; `GET /api/pots/balances`, `GET /api/pots/{id}/ledger` (running balance), `POST /api/pots/{id}/entries` (withdrawal/deposit/adjustment)
- Dashboard: `GET /api/years/{year}/summary`

CSRF: SameSite=Lax + verplichte header `X-Requested-With` op mutaties. `ALLOWED_EMAILS` optioneel (Authentik gate-t toch al). Dev: Vite proxy `/api`+`/auth` → :8080, geen CORS.

## Frontend paginas

- `/login` — knop naar `/auth/login`
- `/years/:year` — **YearDashboard**: 12 maanden In/Uit/Over (zoals sheet "2026"), jaartotalen, potjes-saldo cards
- `/months/:y/:m` — **MonthOverview**: 4 Excel-blokken (Inkomsten inline-edit, Gezamenlijk Rekening budget lines, Totaal-over banner, potjes-percentage-editor met projectie), Close/Reopen met confirm
- `/months/:y/:m/transactions` — **MonthTransactions**: categorie-tabs, snelle invoerrij (bedrag+omschrijving, Enter), totalen
- `/pots`, `/pots/:id` — saldi, ledger, handmatige opname/storting
- `/settings` — categorieën / inkomstenbronnen / potjes beheren (rename, defaults, itemized-flag, archive, reorder)

State: alleen TanStack Query. `lib/money.ts`: parse/format NL+EN decimalen. i18n: react-i18next, NL default, switcher, user-data niet vertaald.

## Excel importer

`cmd/importer --xlsx <pad> --year 2026 --dsn --wipe --close-through 6`
1. Sheets via regex `^(\d+)-\d{2}-(Overview|Details)$`; blokken label-anchored (niet cel-coördinaat).
2. Labels upserten in masterdata; Details-categorieën → `is_itemized=true`; Details-paren (bedrag, omschrijving) → transactions; Totaal-rijen = checksum (warn bij mismatch).
3. "Doorlopen maand X"-inkomsten NIET importeren voor maand ≥2 — die regenereert de close-flow.
4. Volgorde: masterdata → periods 1–12 → jan–jul actuals → aug–dec templates → **maanden 1..6 sluiten via dezelfde domain-close-functie als de API** → verificatierapport (per maand in/uit/over + potsaldi) vergelijken met sheet.
5. Bedragen: `math.Round(v*100)`. Fixture-xlsx (2 maanden) in `internal/importer/testdata/`.

## Lokaal dev + packaging

- compose: `db` (postgres:16-alpine, healthcheck), `dex` (config mount, :5556); profile `full`: `api` + `web`.
- `make dev` = compose up db+dex, `go run ./cmd/server`, `npm run dev`.
- Dockerfile 3-stage → distroless, server+importer binaries + `dist/`.
- Makefile: dev, migrate, sqlc, test, test-integration, import, build-image, lint.

## Helm (`deploy/helm/home-finance`)

- `cnpg-cluster.yaml`: CNPG `Cluster`, 1 instance, 5Gi, initdb `homefinance`. App leest `DATABASE_URL` uit auto-secret `<cluster>-app` key `uri`.
- Deployment (1 replica, probes, `AUTO_MIGRATE=true`), Service, Ingress (host/TLS uit values). OIDC-creds uit `values.oidc.existingSecret`.
- Importer in prod: one-off `kubectl run` met zelfde image — README.

## Tests

- Unit: split-rounding (property: som == surplus; cases 67.5/30/2.5, negatief, 0%), surplus-calc, importer-parser vs fixture, money.ts (vitest).
- Integration (`//go:build integration`, echte pg): close→carryover→reopen round-trip, create-from-copy, closed-write-rejection.
- httptest op overview + close-conflicten. Optioneel Playwright smoke via Dex.

## Build-volgorde (elke fase geverifieerd)

1. Scaffold (tree, compose db+dex, dex config) → compose healthy, Dex discovery-URL antwoordt
2. DB-laag (migration, sqlc) → `psql \dt` 10 tabellen
3. Domain + unit tests → `go test` groen
4. Auth (OIDC/BFF/sessies) → browser-login via Dex werkt, `/api/me` 401→200
5. API-handlers compleet → curl smoke-script (dev-bypass `DEV_FAKE_AUTH` voor curl)
6. Importer → rapport matcht sheet-totalen; jan–jun closed, Doorlopen stroomt naar juli
7. Frontend core (router, i18n, guard, hooks) → login → dashboard toont geïmporteerd jaar
8. Frontend paginas → handmatige walkthrough Excel-workflow voor augustus
9. Packaging → `docker compose --profile full up`, walkthrough op :8080
10. Helm → `helm template` valide
11. Integration tests + README

## Risico's

- Enige echte logica = split-rounding + close/reopen-idempotentie → in `domain/` achter tests; rest is CRUD.
- Importer: warn-and-continue bij onbekende labels, niet falen.
- Excel bevat float-artefacten (1.1e-13 totalen) → importer rondt af, checksum-warn tolerantie ±1 cent.
