# Execute PLAN.md — Home-Finance One-Shot Build

## Context

The repo contains only `PLAN.md` and `README.md`. PLAN.md is a complete, user-approved build plan for a family-finance webapp replacing Wouter's Excel workflow: Go backend (chi + pgx/sqlc + goose), React 18 + TS + Vite + Mantine SPA, OIDC/BFF auth (Dex dev, Authentik prod), Postgres, one-time Excel importer, docker-compose + Helm packaging, i18n NL/EN. "doe plan" = execute that plan. All design decisions (tech stack, datamodel, API surface, close/carryover algorithm, rounding strategy) are already fixed in PLAN.md — this plan is about executing it faithfully, adapted to this remote environment.

## Approach

Follow PLAN.md's build order 1–11 exactly, committing after each verified phase on branch `claude/create-doe-plan-nnn8gr`, pushing at milestones:

1. **Scaffold** — repo tree per PLAN.md layout, Makefile, docker-compose.yml (db + dex + `full` profile), `deploy/dex/config.yaml` (static users wouter/ilse), `.env.example`.
2. **DB layer** — goose migration 0001 with the 10 tables from PLAN.md (integer cents BIGINT, `NUMERIC(5,2)` percentages, partial unique index on active carryover pot), `sqlc.yaml` + queries, embedded migrations with `AUTO_MIGRATE`.
3. **Domain + unit tests** — `internal/domain/`: surplus calc, largest-remainder split rounding (property: sum == surplus; cases 67.5/30/2.5, negative, 0%), close/reopen logic in one Tx per PLAN.md's close-algorithm.
4. **Auth** — go-oidc BFF flow, scs/pgxstore sessions (7d TTL), `DEV_FAKE_AUTH` bypass, CSRF via SameSite=Lax + `X-Requested-With`.
5. **API handlers** — full REST surface from PLAN.md (periods/overview/close/reopen, incomes, budget-lines, transactions, splits bulk-replace, masterdata CRUD+archive+reorder, pot balances/ledger/entries, year summary; 409 `period_closed` on closed-period writes).
6. **Importer** — `cmd/importer` with excelize, label-anchored parsing, idempotent `--wipe`, closes months via the same domain close function. Build a 2-month fixture xlsx in `internal/importer/testdata/` (generated with excelize) and verify the parser + import round-trip against it.
7. **Frontend core** — Vite + React Router v6 + TanStack Query v5 + react-i18next (NL default) + Mantine v7, auth guard, typed API client + hooks, `lib/money.ts` with vitest tests.
8. **Frontend pages** — Login, YearDashboard, MonthOverview (4 Excel blocks, close/reopen), MonthTransactions (category tabs + quick-entry row), Pots + PotDetail, Settings.
9. **Packaging** — 3-stage Dockerfile → distroless (server + importer + dist), compose `full` profile.
10. **Helm** — chart with CNPG Cluster, Deployment/Service/Ingress, `existingSecret` for OIDC.
11. **Integration tests + README** — `//go:build integration` tests against real Postgres: close→carryover→reopen round-trip, create-from-copy, closed-write rejection; httptest for overview + close conflicts; README with dev/prod/import instructions.

## One-shot feasibility check (verified read-only)

Verified in this environment before committing to the plan:
- **Toolchain present**: Go 1.24.7, Node 22, psql + Postgres 16 server binaries (`/usr/lib/postgresql/16/bin`) — DB-dependent phases (migrations, sqlc, integration tests, running the server) work even without Docker.
- **Network via proxy**: proxy.golang.org 200 (go mod + `go install sqlc` will work), registry.npmjs.org 200 (npm install works), Docker Hub/gcr.io respond (pulls work if dockerd starts). Blocked: `get.helm.sh` (403) → helm via `go install helm.sh/helm/v3/cmd/helm` or YAML-lint fallback.
- **Docker**: daemon not running, but we're root with CAP_SYS_ADMIN and dockerd is installed — starting it is likely to work; every docker-dependent verification has a non-docker fallback below, so no phase hard-blocks on it.
- **Chromium + Playwright** are pre-installed → frontend walkthrough (build steps 7–8) can be a real browser smoke test, not just `npm run build`.

**Honest boundary — cannot be verified here, ever**: the real Excel import (`Fam_Finance_2026.xlsx` is on Wouter's Mac), live Authentik, and an actual k8s/CNPG deploy. These get documented in the README as local/production steps. Everything else in PLAN.md's build order 1–11 is completable and verifiable in this single session.

**Main one-shot risk is session length, not blockers**: the build is large, so each phase is committed and pushed as soon as it verifies green. If the session context is summarized mid-build, PLAN.md + the phase commits are the recovery map — no work is lost and the build order resumes from the last green phase.

## Environment adaptations (verification fallbacks)

- **Docker daemon not running** in this container: try starting `dockerd` in the background first. If it won't start, fall back to **local Postgres 16** (`/usr/lib/postgresql/16/bin`: initdb + pg_ctl in scratchpad dir) for migrations, sqlc verification, integration tests, and running the server. Compose files still authored + validated with `docker compose config`.
- **Dex live login**: only feasible if docker works; otherwise verify the auth code paths via `DEV_FAKE_AUTH` + httptest, and validate the Dex config YAML statically. Browser walkthroughs (build order steps 7–8) become: `npm run build` green + vitest green + curl-driven API smoke against the running server with fake auth.
- **helm not installed** and `get.helm.sh` is proxy-blocked: install via `go install helm.sh/helm/v3/cmd/helm@latest` for `helm template`; if that fails, validate chart YAML by parsing/lint.
- **No real Excel file** here (it lives on Wouter's Mac): the importer is verified against the generated fixture; README documents running the real import locally.
- Go toolchain is 1.24 — use `go 1.24` in go.mod (plan said image go1.23; Dockerfile will use go1.24).

## Verification

- Per phase, per PLAN.md's build order: `go build ./...`, `go test ./...` (unit) after phase 3+; `psql \dt` shows 10 tables after migrations; curl smoke script against running server with `DEV_FAKE_AUTH` covering the full API; importer run on fixture prints verification report matching fixture totals with months closed and carryover flowing; `npm run build` + `npx vitest run` green; `docker compose config` valid; `helm template` (or YAML lint fallback) valid; integration tests green against local Postgres.
- Final: run server + built SPA together, curl the SPA fallback + `/api` endpoints end-to-end on imported fixture data.

## Deliverable

Complete codebase committed in verified-phase increments and pushed to `claude/create-doe-plan-nnn8gr` (no PR unless asked).
