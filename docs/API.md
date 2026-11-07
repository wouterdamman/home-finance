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
POST /api/integration-tokens   (body: {"name","scopes":["kids:read"]})           (admin, reauth)
DEL  /api/integration-tokens/:id                                                 (admin)
GET  /api/docs                                                                   (admin)
GET  /api/openapi.yaml                                                           (admin)
```

## Integration API (bearer token)

Read-only API for external tools such as Home Assistant. It is separate from the session-auth
API above: no cookie and no CSRF header, and it never accepts a session.

Admins create tokens in **Settings > Integrations**, choosing a name and scopes. Each token
is shown once on creation and stored only as a SHA-256 hash, so it cannot be recovered later.
Tokens look like `hf_...`. Creating and revoking a token are audit-logged.

Send the token as a bearer header:

```
Authorization: Bearer hf_...
```

```
GET /api/integrations/v1/info               (any valid token)
GET /api/integrations/v1/kids/balances      (scope kids:read)
```

`info` returns the token's name and scopes:

```json
{"apiVersion":1,"tokenName":"Home Assistant","scopes":["kids:read"]}
```

`kids/balances` returns each kid's savings in cents, split by who contributed. `generatedAt` is
the server time the response was built:

```json
{
  "kids": [{"name":"Sam","totalCents":125000,"oursCents":100000,"theirsCents":25000}],
  "generatedAt": "2026-11-08T12:00:00Z"
}
```

Responses carry `Cache-Control: no-store`. Errors use the usual `{"error": {"code", "message"}}`
body:

| Status | Code | Meaning |
|---|---|---|
| 401 | `unauthorized` | Token missing, invalid, or revoked. Response includes `WWW-Authenticate: Bearer` |
| 403 | `insufficient_scope` | Token is valid but lacks the scope the endpoint needs |
| 500 | `db_error` | Server-side failure (e.g. database unreachable) — not a token problem, retry later |

The only scope today is `kids:read`. Future datasets (pots, periods, years) will be added as new
scopes under `/api/integrations/v1`.

### Home Assistant example

Add a `rest:` integration to `configuration.yaml`, with the token in `secrets.yaml`
(`home_finance_auth: "Bearer hf_..."`):

```yaml
rest:
  - resource: https://budget.app.example.com/api/integrations/v1/kids/balances
    headers:
      Authorization: !secret home_finance_auth   # "Bearer hf_..."
    scan_interval: 900
    sensor:
      - name: "Savings <kid name>"
        value_template: "{{ (value_json.kids | selectattr('name','eq','<kid name>') | first).totalCents / 100 }}"
        unit_of_measurement: "EUR"
        device_class: monetary
```
