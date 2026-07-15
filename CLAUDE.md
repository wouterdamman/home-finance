# CLAUDE.md

Project-specific notes for AI agents working in this repo. See `README.md` for stack/setup — this file covers things that aren't obvious from reading the code alone.

## Domain concepts

- **Pot split percentages must always total 100%.** If a `carryover`-kind pot exists (`pots.kind = 'carryover'`), its percentage is never client-supplied — `handleReplaceSplits` (`backend/internal/httpapi/period_handlers.go`) always recomputes it server-side as `100 − sum(other pots)`. This mirrors the family's original Excel workflow, where a "Doorlopen" row absorbed whatever wasn't allocated to a named savings goal. Don't let a code path submit a stored percentage for the carryover pot — it will be overwritten anyway.
- **Only one active carryover pot** is allowed at a time (`pots_one_carryover_idx` unique index). It's excluded from the `/pots` savings-overview UI and the year-dashboard pot cards — it always nets to €0 (its monthly allocation moves out again as a `carryover_out` entry, then reappears as next month's income), so it isn't a savings goal and showing it there is misleading.
- **Period close** (`handleClosePeriod`) computes surplus, splits it via `domain.LargestRemainderSplit`, and writes one `allocation` ledger entry per pot. This function assumes the input percentages already sum to 100% — that invariant is enforced at save time (see above), not inside the split function itself.
- **Template copy**: creating a new period without `copyFromPeriodId` copies budget lines/income entries from the most recent prior period, filtered to `includeInTemplate = true` (categories/income sources) — not from `pots`/`pot_splits`.

## Local dev gotchas

- `rtk npx tsc --noEmit` / `rtk npx vitest run` must run from `frontend/` — if the shell's cwd has drifted (e.g. after a backgrounded `go run` in another directory), rtk's npx wrapper silently resolves the wrong `tsc` package and reports a false "No errors found" with exit code 1. Always `cd frontend` explicitly before these checks if unsure of cwd.
- `@mantine/modals` ships no `styles.css` of its own (it reuses core `Modal` styling) — don't add a `'@mantine/modals/styles.css'` import, it breaks the Vite build. `@mantine/core`, `@mantine/notifications`, `@mantine/charts`, `@mantine/dates` do each ship one.
- `ModalsProvider` must be mounted in `main.tsx` for `modals.openConfirmModal()` (used for close/reopen month, delete pot entry) to render anything — without it, calls are a silent no-op with no console error.
- Local dev Postgres volume is mounted at `/var/lib/postgresql` (not `.../data`) — required by the postgres:18+ image layout. If bumping the postgres image major version again, expect this to need re-checking.
- Real family data can be imported for local testing: `cmd/importer` reads `~/Library/Mobile Documents/com~apple~CloudDocs/Documents/Finance/Fam_Finance_<year>.xlsx`. `--reset-masterdata` wipes all categories/sources/pots/periods first; confirm with the user before running it if there's existing local data worth keeping.

## Conventions

- Conventional commits (`feat:`, `fix:`, `chore:`) — release-please computes the version bump from these on push to `main`. Don't hand-edit `.release-please-manifest.json`, `CHANGELOG.md`, or `Chart.yaml`.
- No `Co-Authored-By` / AI attribution in commit messages.
- New migrations go in `backend/migrations/NNNN_description.sql` with `+goose Up`/`+goose Down`, next sequential number.
- Before considering a feature "done," click through it in the browser against real or realistic multi-item data — single-item test fixtures have repeatedly hidden real bugs (layout wrapping, split-percentage edge cases) that only show up with more than one row/category/pot.
