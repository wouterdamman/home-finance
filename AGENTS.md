# AI Agent Instructions

Instructions for AI coding agents (Claude Code, Copilot, Cursor, etc.) working in this repo.

## Project overview

Family budget tracker replacing an Excel workflow. Go backend + React SPA + PostgreSQL.
See `README.md` for full stack details.

## Key conventions

### Money
- **Always integer cents** (`BIGINT`) — never floats for amounts
- Frontend formats via `Intl.NumberFormat`, backend never formats
- Rounding: largest-remainder algorithm in `backend/internal/domain/` — do not change

### Backend
- Router: `go-chi/chi/v5`
- DB access: raw `pgx/v5` — no ORM, no sqlc for new queries (just write SQL directly)
- Migrations: goose SQL files in `backend/migrations/` — always add a new numbered file, never edit existing ones
- Config: env vars via `caarlos0/env/v11`, all in `backend/internal/config/config.go`
- Auth: BFF pattern — HttpOnly session cookie, never expose tokens to the frontend
- Errors: return `{"error": {"code": "...", "message": "..."}}` JSON, use appropriate HTTP status codes
- Audit log: call `s.auditLog()` after any destructive or state-changing action (close, reopen, delete, pot entries)

### Frontend
- Mantine v7 components — do not mix with other UI libraries
- TanStack Query v5 for all server state — no local state for fetched data
- All user-visible strings must go through `react-i18next` — add keys to both `en.json` and `nl.json`
- Amounts: always pass/receive cents as integers; format with `MoneyText` component or `lib/money.ts`
- No comments in code unless the WHY is non-obvious

### Git
- Conventional commits: `feat:`, `fix:`, `ci:`, `chore:`, `docs:`, `perf:`
- release-please reads commit messages to generate CHANGELOG and bump version automatically
- Never add `Co-Authored-By: Claude` or AI attribution to commits

## Architecture decisions (do not revisit)

- Surplus calculation: `incomeTotalCents - expenseTotalCents` (transactions sum used when `tracks_transactions=true`)
- Close algorithm: single DB transaction — totals → allocation ledger rows (largest-remainder) → carryover entry in next period → status=closed
- Reopen: delete ledger rows with `source_period_id=N`, delete carryover income entry in N+1, status=open. Blocked if N+1 is already closed.
- Pot `kind='carryover'`: max 1 active (partial unique index). Close writes negative `carryover_out` + income_entry in next period.
- `tracks_transactions=false` on a budget_line: uses `amount_cents` directly (no transaction summing)
- `include_in_template=true` on sources/categories: copied when auto-templating a new period

## Dev setup

```bash
cp .env.example .env
make dev   # postgres + dex in docker, backend :8080, frontend :5173
```

`DEV_FAKE_AUTH=true` is set in `.env` — OIDC is bypassed locally.
`DELETE_PASSWORD=dev123` — any non-empty value accepted in dev mode.

## Testing

```bash
make test              # unit tests (Go + vitest)
make test-integration  # needs running postgres
cd frontend && npx tsc --noEmit  # type check
```

CI runs all three on every PR via `.github/workflows/ci.yml`.
