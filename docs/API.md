# API reference

All endpoints under `/api`, session-auth via cookie, amounts in cents.

Endpoints marked **(admin)** are gated by `requireAdmin` — read endpoints stay open to any
authenticated user so a `role=user` account can still use the app day-to-day. Endpoints marked
**(reauth)** additionally need a step-up re-authentication completed in the last 5 minutes
(`GET /api/reauth-status`); this app never accepts a password in a request body.

```
GET   /api/me
PATCH /api/me                  (own display name)
GET   /api/reauth-status       ({"fresh": bool} — step-up reauth freshness)
POST  /api/me/avatar           (multipart; bytes land in users.avatar_data, not object storage)
GET   /api/users/:id/avatar    (auth-gated proxy — avatars are never a public URL)
GET   /api/users                                (admin)
PATCH /api/users/:id/role      (body: {"role": "admin"|"user"})  (admin — can't demote the last admin)

GET  /api/years
POST /api/years                                 (admin)
GET  /api/years/:year/summary
POST /api/years/:year/lock                      (admin, reauth)
POST /api/years/:year/unlock                    (admin, reauth)

GET  /api/periods?year=
POST /api/periods
GET  /api/periods/:id/overview
POST /api/periods/:id/close                     (admin)
POST /api/periods/:id/reopen                    (admin)
DEL  /api/periods/:id                           (admin, reauth)

CRUD /api/periods/:id/incomes,             /api/incomes/:id
CRUD /api/periods/:id/budget-lines,        /api/budget-lines/:id
CRUD /api/periods/:id/transactions,        /api/transactions/:id
CRUD /api/periods/:id/income-transactions, /api/income-transactions/:id
PUT  /api/periods/:id/splits   (carryover pot's percentage is always server-computed)

GET  /api/categories
POST/PUT /api/categories, /api/categories/:id, /api/categories/:id/archive     (admin)
GET  /api/income-sources
POST/PUT /api/income-sources/*                                                  (admin)
GET  /api/pots
POST/PUT /api/pots, /api/pots/:id, /api/pots/:id/archive                        (admin)
GET  /api/pots/balances
GET  /api/pots/:id/ledger
POST  /api/pots/:id/entries
PATCH /api/pot-entries/:id     (manual entries only — amountCents)
DEL   /api/pot-entries/:id     (manual entries only — allocation/carryover_out are lifecycle-managed)

GET   /api/kids, /api/kids/balances, /api/kids/:id/ledger
POST  /api/kids/:id/entries
PATCH /api/kid-entries/:id
DEL   /api/kid-entries/:id
PATCH /api/kids/:id/reported-balance                                            (admin)

GET  /api/trends/years, /api/trends/category-totals, /api/trends/monthly-totals
GET  /api/trends/income-sources, /api/trends/pot-balances
GET  /api/trends/descriptions?categoryId=&sourceId=&limit=  (limit default 20, max 100)

GET  /api/export/years/:year?months=1,2,3   (whole year if months omitted)      (admin)
POST /api/import/xlsx          (multipart: file, year, wipe, resetMaster, closeThrough)  (admin)
                                (legacy Fam_Finance workbook or this app's own export, auto-detected;
                                 wipe/resetMaster need a fresh reauth and are audit-logged)

GET  /api/audit-log?limit=&before=&action=&entityType=&userEmail=&from=&to=     (admin)
GET  /api/integration-tokens                                                     (admin)
POST /api/integration-tokens   (body: {"name","scopes":["kids:read","pots:read"],"expiresAt"?})  (admin, reauth)
DEL  /api/integration-tokens/:id                                                 (admin)
GET  /api/docs                                                                   (admin)
GET  /api/openapi.yaml                                                           (admin)
```

## Integration API (bearer token)

A separate, read-only API for external tools such as Home Assistant: no cookie, no CSRF header, and
it never accepts a session. Admins create tokens (`hf_...`, shown once, always expiring) in
**Settings > Integrations**; clients send them as `Authorization: Bearer hf_...`. Responses carry
`Cache-Control: no-store`, and errors use the usual `{"error": {"code", "message"}}` body
(`401 unauthorized`, `403 insufficient_scope`, `404 not_found`, `500 db_error`).

```
GET /api/integrations/v1/info                       (any valid token)
GET /api/integrations/v1/kids                       (scope kids:read)  all kids + household totals
GET /api/integrations/v1/kids/balances              (scope kids:read)  legacy summary, no slugs
GET /api/integrations/v1/kids/:slug                 (scope kids:read)  one kid, full detail
GET /api/integrations/v1/kids/:slug/ours            (scope kids:read)  sub-pot "door ons gespaard"
GET /api/integrations/v1/kids/:slug/theirs          (scope kids:read)  sub-pot "eigen geld"
GET /api/integrations/v1/pots                       (scope pots:read)  all savings pots + household totals
GET /api/integrations/v1/pots/:slug                 (scope pots:read)  one pot, full detail
```

The scopes today are `kids:read` and `pots:read`. Token setup and rotation, the full endpoint reference with
example responses and field tables, concepts, a Home Assistant cookbook and troubleshooting live in
**[INTEGRATIONS.md](INTEGRATIONS.md)**, which is the single source of truth for this API.
