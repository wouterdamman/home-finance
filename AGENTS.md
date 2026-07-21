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
- Chart color palette (income/expenses/surplus + a 4-slot categorical set) is a per-user choice in Settings > Profile (`useChartPalette()` from `frontend/src/contexts/ChartPaletteContext.tsx`), not Preferences — it sits next to avatar/display-name because it's personal, not app-wide. Never hardcode a Mantine color string (`'teal.6'`, `'red.6'`, ...) directly on a chart `series` — pull from `palette.income`/`palette.expenses`/`palette.surplus`/`palette.categorical[i]` (`frontend/src/lib/chartPalette.ts`) so all three palettes (including the hex-based Okabe-Ito one) render correctly everywhere

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
- `budget_lines.tracks_transactions` is deliberately editable **per period**, independent of `categories.is_itemized` — this is intentional, not drift to "fix away". Some categories (e.g. a pet-costs line) are a single amount most months but need splitting into several dated transactions in others; the toggle icon in `MonthOverview` (with a confirm dialog, see below) exists for exactly that. `categories.is_itemized` is only the *default* applied when a new period is templated (`handleCreatePeriod`'s copy-from-previous-period INSERT derives `tracks_transactions` fresh from `categories.is_itemized` via a `LEFT JOIN`, not by copying the prior period's value forward) — after creation, the per-period value is the only thing that matters, and a change in Settings never retroactively touches existing periods.
- The `tracks_transactions` toggle (`MonthOverview.tsx`, `handleToggleTracking`) requires confirmation via `modals.openConfirmModal` before mutating — a prior version toggled on a bare click and a stray click during unrelated testing silently flipped two budget lines' state for a whole month, which went unnoticed until the numbers looked wrong weeks later. Don't remove the confirm step to "simplify" the interaction.
- RBAC (migration 0008): `users.role` is `'admin'|'user'`, default `'user'`. `auth.RequireAdmin` (DB lookup per request, not session-cached, so a role change in Settings > Users takes effect immediately without re-login) gates write endpoints for categories/income-sources/pots/years, year lock/unlock, period close/reopen/delete, `/api/audit-log`, `/api/docs`, `/api/openapi.yaml`, and `/api/users*`. Read endpoints stay open — a `role=user` account still needs them to use the app. `INITIAL_ADMIN_EMAILS` only sets the role on the INSERT branch of `upsertUserCtx` — an existing user's role is never touched on login, so it can't silently revert an admin's change. The last remaining admin can't be demoted (`handleUpdateUserRole` counts admins first).
- Excel export (`export_handlers.go`) and import (`internal/importer`) are deliberately two different sheet formats read by one auto-detecting parser (`importer.DetectAndParse`, dispatches on sheet-name pattern): the legacy Fam_Finance layout (`7-26-Overview`/`7-26-Details`, fixed columns, no per-transaction dates) for migrating old data, and this app's own export layout (`YYYY-MM` sheets, readable Inkomsten/Uitgaven/Transacties sections, real per-transaction dates) for a genuine export→edit→reimport round-trip. Pot split percentages don't round-trip through the app's own export — it only snapshots current balances, not each period's `pot_splits` — so a reimported period allocates nothing to pots on close (silent no-op, matches existing `mapper.go` behavior for periods with zero splits).
- S3 (avatars + audit-log export) is one shared, fully-optional `S3Config`/`internal/s3client.New()` — both features degrade to "disabled" (avatar upload 503s, export goroutine never starts) rather than failing startup when it's unset. Avatars are never served from a public bucket URL; `GET /api/users/:id/avatar` proxies the object through session auth.
- Audit-log S3 export (`internal/auditexport`, opt-in via `AUDIT_EXPORT_INTERVAL`) is an in-app goroutine, not a Kubernetes CronJob — cron can't schedule below one minute and the interval is user-configurable down to seconds. Keyset-resumes from `audit_log_export_state` (singleton row, `last_exported_id`), ships newline-delimited JSON, and a failed tick just logs a warning and retries next interval — same best-effort philosophy as `s.auditLog()` itself, an export hiccup must never affect a user-facing request.
- `/trends` is a modular widget dashboard (`frontend/src/lib/trendsDashboard.ts` — `WidgetConfig` union, `frontend/src/pages/Trends.tsx` — grid/drag/resize/edit-mode), layout persisted in `localStorage` only (`trends-dashboard-v1`, no backend table — single/family-user app, same reasoning as the `AppShell` sidebar-pin pattern). Each widget type is **self-contained**: it owns its own year/month/category selection and does not read the page's single global year `Select` — that global filter only feeds `kpi`/`categoryChart` widgets. Don't wire a new widget type to the global filter unless it genuinely can't stand alone; every widget added after the first redesign (`monthCompare`, `allTimeTrend`, `monthAcrossYears`) is self-contained specifically because the global-filter-coupled ones (the removed `yearCompare` widget, the old always-all-years behavior) were the recurring source of "why does this look empty/wrong" bugs.
- Per-category small-multiple widgets (one card per category) are **not** in `defaultWidgets()` — explicitly rejected as dashboard clutter. The default set is 4 KPI tiles + one all-time trend + one multi-category widget (top 4 by spend, in a *single* widget) + one recent-months widget. `categoryChart` (now multi-select, up to 4 categories per widget) stays addable via "Widget toevoegen" for anyone who wants more.
- `/trends/months` (`frontend/src/pages/MonthCompareDetail.tsx`) is a separate drill-down page, not a widget — compares 2-4 arbitrary (year, month) periods side by side with a per-category diff table (sortable, searchable, per-row hide toggle) plus two charts (income/expenses/surplus per period, and a "biggest movers" ranking of categories by `|last − first|`). Reuses `useTrendsCategoryTotals()`/`useYearSummary()` — no dedicated backend endpoint for this page.

## Known gotchas

- `ModalsProvider` (from `@mantine/modals`, mounted in `frontend/src/main.tsx`) must wrap the app for `modals.openConfirmModal()` to render anything — used for close/reopen month and pot-entry delete confirmations. Without it, calls silently no-op with no console error.
- `@mantine/modals` has no `styles.css` of its own (it reuses core `Modal` styling) — never add a `'@mantine/modals/styles.css'` import, it breaks the Vite build. `@mantine/core`, `@mantine/notifications`, `@mantine/charts`, and `@mantine/dates` each do ship one.
- Local dev Postgres volume must mount at `/var/lib/postgresql` (not `.../data`) — required by the postgres:18+ image layout.
- `cd frontend` explicitly before `npx tsc --noEmit` / `npx vitest run` if unsure of shell cwd — running from the repo root or `backend/` can silently resolve the wrong `tsc` binary and report a false-clean result.
- Mantine `Text` renders a `<p>` by default. Never pass a block-level component (`Progress`, another `Box`/`div`-based component) as its `children` without `component="div"` — a `<p>` containing a `<div>` is invalid HTML and throws a React hydration warning in the console, not a build error, so it's easy to ship unnoticed. This bit `MobileListRow`'s `subtitle` slot (fixed by forcing `component="div"` there).
- `react-swipeable-list` gestures cannot be verified with synthetic `dispatchEvent(MouseEvent(...))` calls in a scripted browser session — the library needs real, continuous pointer movement. A single tool-driven `left_click_drag` works; a JS loop of manual `mousedown`/`mousemove`/`mouseup` does not. Don't conclude swipe is broken from a failed synthetic-event test — retest with an actual drag gesture.
- PWA icon source SVGs live in `frontend/design-assets/`, not `frontend/public/` — anything in `public/` ships verbatim in the Vite build output, so raw source files there would be served as dead weight. Only the rendered PNG/SVG outputs (`pwa-*.png`, `favicon.svg`, etc.) belong in `public/`.
- Mantine layout region config (`AppShell` `header`/`navbar`/`footer` props) accepts a responsive `{ base, sm, ... }` object for `height`, letting a region collapse to 0 on desktop without a separate conditional — used for the mobile-only bottom tab bar footer.
- pgx v5's extended query protocol rejects multiple SQL statements in one `pool.Exec()` call (`cannot insert multiple commands into a prepared statement`) — a semicolon-separated string of statements silently never worked (found in `importer.wipe()`, which nobody had exercised end-to-end until E2's UI made the wipe option reachable). Split into separate `Exec` calls, or better, rely on `ON DELETE CASCADE` where the schema already has it.
- pgx can't scan a `DATE`/`TIMESTAMPTZ` column into `*string` — it silently errors, and if that error is swallowed (e.g. `if row.Scan(...) != nil { continue }`), the row just vanishes with no visible failure. Scan into `*time.Time` and `.Format(...)` it yourself. (Found in `export_handlers.go`'s transaction rows — every export shipped an empty Transacties section for one full session before a round-trip test surfaced it.)
- A Mantine `Select`/`Tabs`/array-of-object-literal list needs its element type annotated at the point of the literal (e.g. `const items: Item[] = [...]`), not just on a variable that later gets `.filter()`/`.map()`'d — TS widens a union-typed `key: 'a'|'b'` field to plain `string` if the object literal itself isn't in a directly-typed position, and the narrowing doesn't survive the chained call.
- `@mantine/charts` y-axis tick labels fall back to the chart's `valueFormatter` (full-precision `formatCents`, e.g. `"€10.000,00"`) unless you pass `yAxisProps.tickFormatter` explicitly — on any chart whose values can reach 4+ digits, that overflows past the card's left border since the axis gets no extra width for it. Always pass `yAxisProps={{ tickFormatter: (v) => formatCentsCompact(...), width: 56, ticks, domain: [0, ticks.at(-1)] }}` (see `lib/chartAxis.ts`'s `niceAxisTicks` for `ticks`) — every chart in `components/trends/` and `YearDashboard.tsx` follows this pattern, don't add a new chart without it.
- Recharts' own x-axis label auto-thinning (when 8-12+ category ticks don't all fit) picks an uneven subset — e.g. dropping just "Nov" while keeping every other month, instead of a clean "every 2nd". Pass an explicit `xAxisProps={{ interval }}` computed from the point count (`Math.ceil(length / 6) - 1`) instead of trusting the default.
- The Trends widget size picker (`SizeGridPicker.tsx`) is a Home-Assistant-style 2D grid: clicking any cell sets **both** width and height from that cell's column/row position, not just the dimension you meant to change — clicking to widen a widget while its height slider is at 3 will silently reset height to whatever row you clicked in. Use the width slider and height slider independently, or double-check "N breed × M hoog" after a grid click.
- A `SimpleGrid` row only equal-heights its cells if `h="100%"` sits on the element that is *directly* the grid item — wrapping it one level deeper (e.g. `<Table.ScrollContainer><Paper h="100%">`) means the outer `ScrollContainer` stays content-sized and the `h="100%"` on `Paper` resolves against that shrunk parent, not the grid row. Put `h="100%"` on the outermost child of the grid cell.
- A period's effective income/expense total (itemized income sources sum `income_transactions`, tracked budget lines sum `transactions`, everything else uses its own `amount_cents`) must always be computed with `domain.EffectiveIncomeCentsSQL`/`domain.EffectiveExpenseCentsSQL` (`backend/internal/domain/period.go`) — never a plain `SUM(amount_cents)`. Found because `handleClosePeriod` had drifted onto the plain-SUM formula (migration 0010 added itemized income after that handler was written), silently writing a wrong surplus into pot allocations for any period with an itemized source; the same stale formula was duplicated in the importer and the Excel export. Every query that totals a period's income or expenses uses these two shared snippets now — if you add a new one, use them instead of re-deriving the CASE.
- `window.localStorage` comes back `undefined` in this repo's jsdom+Node version combo under vitest (a Node 22+ global/jsdom interaction, not app code) — any test touching `localStorage`-backed code needs the in-memory `Storage` polyfill installed in `frontend/src/test-setup.ts`. Don't add a second ad-hoc mock per test file; the polyfill already runs for every test via `setupFiles`.
- scs (`backend/internal/auth/session.go`) stores session data as `map[string]interface{}`, gob-encoded. Any concrete type put into the session beyond gob's built-ins (string/int/bool/...) must be `gob.Register`'d first, or the session `Commit` fails silently from the handler's point of view: scs's `sessionResponseWriter.Write`/`WriteHeader` doesn't check the commit error before writing the handler's own response body, so the client gets a corrupted response — a 500 "Internal Server Error" body immediately followed by the handler's real success output on the same connection, both rendered as text. This broke the entire step-up-reauth flow (`sm.Put(ctx, "reauthAt", time.Now())` stores a `time.Time`) from the day it shipped, undetected because unit tests never round-trip a session through the real Postgres store. `time.Time` is now registered in `session.go`'s `init()` — if a future session key stores any other non-builtin type, register it there too.

## Dev setup

```bash
cp .env.example .env
make dev   # postgres + dex in docker, backend :8080, frontend :5173
```

`DEV_FAKE_AUTH=true` is set in `.env` — OIDC is bypassed locally, including
the step-up reauth popup used for destructive actions (delete period,
lock/unlock year, import wipe/reset) — it's granted automatically, no
Authentik round trip in dev.

## Testing

```bash
make test              # unit tests (Go + vitest)
make test-integration  # needs running postgres
cd frontend && npx tsc --noEmit  # type check
```

CI runs all three on every PR via `.github/workflows/ci.yml`.
