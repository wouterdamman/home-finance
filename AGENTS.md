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
- Mantine v9 components — do not mix with other UI libraries
- TanStack Query v5 for all server state — no local state for fetched data
- All user-visible strings must go through `react-i18next` — add keys to both `en.json` and `nl.json`
- Amounts: always pass/receive cents as integers; format with `MoneyText` component or `lib/money.ts`
- No comments in code unless the WHY is non-obvious
- Theme (light/dark/system) and language controls live in Settings > Preferences, not the header — don't re-add them to `AppShell.tsx`

### Mobile (below Mantine's `sm` breakpoint, 768px)
- Not a responsive shrink of desktop — a separate mobile IA (bottom tab bar, list rows, bottom
  sheets). Desktop JSX must stay untouched; see `immutable-jingling-treehouse.md` plan for the
  full rationale and rejected "shrink the table" approach.
- Each page forks its render tree with `useMediaQuery('(max-width: 47.99em)')` — a structural
  fork, not `hiddenFrom`/`visibleFrom` (those mount both trees, just hide one with CSS).
- Reuse `frontend/src/components/mobile/{MobileList,BottomSheet,HeroStat}.tsx` instead of
  building new mobile-only table/modal patterns per page.
- `MobileList` rows can take a `swipeAction` (via `react-swipeable-list`) for
  delete/archive — mirrors the desktop trash-icon action, don't add a second delete affordance.
- `BottomSheet` is a thin wrapper around Mantine `Drawer` (`position="bottom"`) — always override
  `styles.content` with `height: 'auto'` + a `maxHeight`; the `size` prop does not accept `'auto'`
  for Drawer and silently renders full-height instead of content-height.

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
- Mantine `Text` renders a `<p>` by default. Never pass a block-level component (`Progress`, another `Box`/`div`-based component) as its `children` without `component="div"` — a `<p>` containing a `<div>` is invalid HTML and throws a React hydration warning in the console, not a build error, so it's easy to ship unnoticed. This bit `MobileListRow`'s `subtitle` slot (fixed by forcing `component="div"` there).
- `react-swipeable-list` gestures cannot be verified with synthetic `dispatchEvent(MouseEvent(...))` calls in a scripted browser session — the library needs real, continuous pointer movement. A single tool-driven `left_click_drag` works; a JS loop of manual `mousedown`/`mousemove`/`mouseup` does not. Don't conclude swipe is broken from a failed synthetic-event test — retest with an actual drag gesture.
- PWA icon source SVGs live in `frontend/design-assets/`, not `frontend/public/` — anything in `public/` ships verbatim in the Vite build output, so raw source files there would be served as dead weight. Only the rendered PNG/SVG outputs (`pwa-*.png`, `favicon.svg`, etc.) belong in `public/`.
- Mantine layout region config (`AppShell` `header`/`navbar`/`footer` props) accepts a responsive `{ base, sm, ... }` object for `height`, letting a region collapse to 0 on desktop without a separate conditional — used for the mobile-only bottom tab bar footer.

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
