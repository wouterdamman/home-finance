# Security & Code-Quality Fixes

Changes applied after the v1.0.0 security review and code-quality review.
Each entry lists the exact file and line(s) where the change lives.

---

## Security Fixes

### SEC-1 — `ALLOWED_EMAILS` never enforced
**File:** `backend/internal/httpapi/auth_handlers.go:80`

The `AllowedEmails` config slice was parsed but never read. After claims extraction in `handleAuthCallback`, a loop now rejects any email not in the allowlist with HTTP 403.

---

### SEC-2 — Open redirect via `return_to`
**File:** `backend/internal/httpapi/auth_handlers.go:12–21`

`sanitizeReturnTo()` helper added. Rejects absolute URLs (`u.IsAbs()`) and any value that does not start with `/`. Applied to both the DevFakeAuth path (line 24) and the OIDC login path (line 109).

---

### SEC-3 — OIDC nonce not verified
**File:** `backend/internal/httpapi/auth_handlers.go:67, 74–78`

`Nonce string` field added to the claims struct. After `idToken.Claims()`, the value is compared to the session-stored nonce (`oidc_nonce`). A mismatch returns HTTP 400 before any user lookup.

---

### SEC-4 — Session `Secure` cookie flag tied to `ENV`
**Files:**
- `backend/internal/config/config.go:21` — `SESSION_SECURE bool` env var added (`envDefault:"true"`)
- `backend/cmd/server/main.go:62` — session manager now receives `cfg.SessionSecure` instead of `cfg.Env != "development"`
- `.env.example` — `SESSION_SECURE=true` documented

---

## Hardening

### HARDENING-1 — Session fixation on login
**File:** `backend/internal/httpapi/auth_handlers.go:27, 101`

`s.sm.RenewToken(r.Context())` called immediately before `Put("userID", ...)` in **both** login paths (DevFakeAuth and OIDC callback). Prevents a pre-auth session token from being elevated to an authenticated one.

### HARDENING-2 — Audit log missing index
**File:** `backend/migrations/0004_audit_log_index.sql`

`CREATE INDEX audit_log_created_at_idx ON audit_log (created_at DESC)` — ensures the `ORDER BY created_at DESC LIMIT 200` query in `handleListAuditLog` uses an index scan.

---

## Code-Quality Fixes

### CQ-H1 — Period creation not atomic
**File:** `backend/internal/httpapi/period_handlers.go:89–132`

`handleCreatePeriod` previously committed the `INSERT INTO periods` row first, then copied budget lines / pot splits / income entries outside that transaction. All four operations are now wrapped in a single `pgx` transaction with `defer tx.Rollback`.

---

### CQ-H2 — Ignored `Begin`/`Commit` errors
**Files:**
- `backend/internal/httpapi/period_handlers.go:392–397` — `handleReopenPeriod`: `tx, err := s.pool.Begin` with HTTP 500 on failure; `tx.Commit` error returned as HTTP 500
- `backend/internal/httpapi/masterdata_handlers.go:90–101` — `handleReorderCategories`: same pattern applied

---

### CQ-M1 — `handleClosePeriod` reads outside transaction
**File:** `backend/internal/httpapi/period_handlers.go:272–361`

Income totals, expense totals, and pot-split rows were previously read via the pool before `Begin`. All reads now happen inside the transaction with `SELECT ... FOR UPDATE` on the period row (line 281), preventing a race condition where the totals could differ from what gets committed to `pot_ledger`.

---

### CQ-M2 — `useUpdateBudgetLine` missing year-summary invalidation
**File:** `frontend/src/api/hooks/usePeriods.ts:63–73`

`useUpdateBudgetLine` now accepts a `year` parameter and invalidates `['year-summary', year]` on success, so toggling `tracksTransactions` immediately updates the year dashboard.
Call site: `frontend/src/pages/MonthOverview.tsx:46`.

---

### CQ-M3 — Tracking toggle coerces null label to empty string
**File:** `frontend/src/pages/MonthOverview.tsx:212`

`label: bl.label ?? ''` changed to `label: bl.label ?? null`. Sends `null` to the API when no label is set, rather than an empty string that would overwrite the stored null.

---

### CQ-L1 — Redundant manual cascade deletes
**File:** `backend/internal/httpapi/period_handlers.go:448`

`handleDeletePeriod` previously ran five manual `DELETE` statements for child tables. All FK relations to `periods` carry `ON DELETE CASCADE` (confirmed in `backend/migrations/0001_init.sql`). Replaced with a single `DELETE FROM periods WHERE id=$1`.

---

### CQ-L3 — `auditLog` unnecessary DB round-trip
**Files:**
- `backend/internal/httpapi/helpers.go:42` — `auditLog` reads the caller's email from the session key `"userEmail"` instead of issuing a `SELECT FROM users`
- `backend/internal/httpapi/auth_handlers.go:31, 105` — email stored in session at login time in both paths

---

### CQ-L4 — Hardcoded `"Bron #${inc.sourceId}"` fallback
**Files:**
- `frontend/src/pages/MonthOverview.tsx:126` — replaced with `t('month.unknownSource', { id: inc.sourceId })`
- `frontend/src/i18n/locales/en.json` — key `month.unknownSource`: `"Source #{{id}}"`
- `frontend/src/i18n/locales/nl.json` — key `month.unknownSource`: `"Bron #{{id}}"`

---

### CQ-L5 — Dutch carryover label stored in DB
**File:** `backend/internal/httpapi/period_handlers.go:340`

The `"Doorlopen maand <maand>"` string inserted into `income_entries.label` is a fixed data label, not UI text. It is displayed as-is in the income list. Accepted as a known limitation; no i18n applied to data-layer labels.

---

### CQ-L6 — Duplicated cent-parsing in income `onBlur`
**File:** `frontend/src/pages/MonthOverview.tsx:143`

Inline `Math.round(parseFloat(raw) * 100) || 0` replaced with `parseCents(raw)` (defined at line 18). The euro prefix and thousands-separator stripping still happens on line 142 before the call.

---

### CQ-L7 — `IncludeInTemplate` JSON-decode default
**Files:**
- `backend/internal/httpapi/masterdata_handlers.go:48` — `handleCreateCategory`
- `backend/internal/httpapi/masterdata_handlers.go:72` — `handleUpdateCategory`
- `backend/internal/httpapi/masterdata_handlers.go:154` — `handleCreateIncomeSource`
- `backend/internal/httpapi/masterdata_handlers.go:178` — `handleUpdateIncomeSource`

`body.IncludeInTemplate = true` is set before `DecodeJSON` so that a missing field in the request body defaults to `true` (opt-in by default). The client must explicitly send `false` to opt out.

---

### CQ-L8 — `handleReopenPeriod` missing guard for already-open period
**File:** `backend/internal/httpapi/period_handlers.go:375`

Status check added: returns HTTP 409 `period_already_open` if the period is not closed, mirroring the existing check in `handleClosePeriod`.
