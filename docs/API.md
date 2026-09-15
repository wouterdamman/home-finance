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

GET  /api/export/years/:year?months=1,2,3   (whole year if months omitted)      (admin)
POST /api/import/xlsx          (multipart: file, year, wipe, resetMaster, closeThrough)  (admin)
                                (legacy Fam_Finance workbook or this app's own export, auto-detected;
                                 wipe/resetMaster need a fresh reauth and are audit-logged)

GET  /api/audit-log?limit=&before=&action=&entityType=&userEmail=&from=&to=     (admin)
GET  /api/docs                                                                   (admin)
GET  /api/openapi.yaml                                                           (admin)
```
