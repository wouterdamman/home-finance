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
- Pot split percentages must always total 100%. If a carryover pot exists, `handleReplaceSplits` never stores its client-supplied percentage — it's always recomputed server-side as `100 − sum(other pots)` and rejects the save if that would go negative. Without a carryover pot, the submitted total must equal 100% exactly. Do not let a code path treat the carryover pot's percentage as user-editable data; `LargestRemainderSplit` assumes the total is already 100% and will misallocate the surplus onto an arbitrary pot if that invariant is broken upstream.
- `years` is a plain registry table (not derived from `periods`), populated by explicit creation (`POST /api/years`, Settings > Years UI) or automatically when a period is created for a new year. The year-list UI (sidebar) reads from this table, not from a computed range — a year with zero periods can still be "created" and will show up.
- `tracks_transactions=false` on a budget_line: uses `amount_cents` directly (no transaction summing)
- `include_in_template=true` on sources/categories: copied when auto-templating a new period

## Known gotchas

- `ModalsProvider` (from `@mantine/modals`, mounted in `frontend/src/main.tsx`) must wrap the app for `modals.openConfirmModal()` to render anything — used for close/reopen month and pot-entry delete confirmations. Without it, calls silently no-op with no console error.
- `@mantine/modals` has no `styles.css` of its own (it reuses core `Modal` styling) — never add a `'@mantine/modals/styles.css'` import, it breaks the Vite build. `@mantine/core`, `@mantine/notifications`, `@mantine/charts`, and `@mantine/dates` each do ship one.
- Local dev Postgres volume must mount at `/var/lib/postgresql` (not `.../data`) — required by the postgres:18+ image layout.
- `cd frontend` explicitly before `npx tsc --noEmit` / `npx vitest run` if unsure of shell cwd — running from the repo root or `backend/` can silently resolve the wrong `tsc` binary and report a false-clean result.

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
