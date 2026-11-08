# Integrations guide

How to read Home Finance data from external tools such as Home Assistant, using the read-only
bearer-token **Integration API**. This guide is meant to be complete enough to build an
integration without reading the code. For the session-authenticated app API, see
[API.md](API.md). The machine-readable spec for the same endpoints is served by the app at
`/api/openapi.yaml` (admin session) and rendered at `/api/docs`.

- [1. Overview](#1-overview)
- [2. Token setup](#2-token-setup)
- [3. Reaching the API](#3-reaching-the-api)
- [4. Endpoint reference](#4-endpoint-reference)
- [5. Concepts](#5-concepts)
- [6. Home Assistant cookbook](#6-home-assistant-cookbook)
- [7. Errors and troubleshooting](#7-errors-and-troubleshooting)
- [8. Security notes](#8-security-notes)
- [9. Roadmap](#9-roadmap)

Example data in this guide uses invented kids ("Sam", "Noor", "Mila") and invented savings pots
("Vacation", "New car", "Buffer"). Amounts are always
**integer cents**, dates are `YYYY-MM-DD`, timestamps are RFC 3339 in UTC.

## 1. Overview

The Integration API lives under `/api/integrations/v1`. It is:

- **Read-only.** There are no write endpoints.
- **Token-authenticated.** Every request carries `Authorization: Bearer hf_...`. It never accepts
  a session cookie and needs no CSRF header.
- **Scoped.** Each token has one or more scopes. Today there are four: `kids:read`, which grants
  access to all kids' savings data, `pots:read`, which grants access to the household's
  savings pots (there is no per-kid or per-pot restriction), `periods:read` for the monthly
  totals and `years:read` for the yearly totals.
- **Privacy-minimal.** Responses contain no internal ids and no ledger descriptions (those are
  free text). Kid and pot names are included. The month and year totals contain only sums, no
  names at all.
- **Uncached.** Every response carries `Cache-Control: no-store`.

**Versioning promise.** Within `/v1` only *additive* changes happen: new endpoints, new fields in
existing objects, new scopes. Existing fields keep their name, type, unit and meaning. Clients
must ignore unknown fields. A breaking change will be published as `/v2`, and `/v1` keeps working
alongside it for a transition period.

## 2. Token setup

1. Sign in to the app as an **admin** and open **Settings > Integrations**.
2. Give the token a recognisable **name** (for example `Home Assistant`). The name is shown in the
   token list and in the audit log.
3. Tick the **scope(s)** you need: `kids:read` for the kids' savings, `pots:read` for the savings
   pots, `periods:read` for the monthly totals and `years:read` for the yearly totals. For
   everything in this guide that is all four. A token's scopes cannot be changed after
   creation, so to add a scope to an existing integration, create a new token with the scopes you
   need and rotate to it (see below).
4. Choose the **expiry**. Every token expires: the default is **1 year**, the minimum is **1 hour**
   and the maximum is **2 years** (via the API's `expiresAt`, RFC 3339; one minute of slack at each
   end). An expiry outside that range is rejected with `400 bad_request`.
5. Create the token. Creating a token is a sensitive action, so the app asks for a **step-up
   re-authentication** (the reauth popup): confirm with your identity provider and the create goes
   through. The reauth stays fresh for 5 minutes.
6. **Copy the token now.** It looks like `hf_...` and is shown exactly once. Only a SHA-256 hash is
   stored, so it cannot be shown again. If you lose it, create a new one.
7. Store it in a secret manager (for Home Assistant: `secrets.yaml`, see
   [section 6](#6-home-assistant-cookbook)). Never commit it to a repository.

**Rotating a token before it expires** (do this a few weeks ahead; the token list shows each
token's expiry):

1. Create a new token with the same scopes.
2. Put the new value in your integration's secret store and reload/restart the integration.
3. Confirm it works (see [testing with curl](#testing-a-token-with-curl)).
4. Revoke the old token in Settings > Integrations. Revoking takes effect immediately.

An expired token is rejected exactly like a revoked or unknown one (`401 unauthorized`). If a
sensor suddenly goes `unavailable` after a year, check the expiry first.

## 3. Reaching the API

| Where the client runs | Base URL | Notes |
|---|---|---|
| Anywhere (public) | `https://budget.app.example.com/api/integrations/v1` | The same host as the app. Works wherever the app's HTTPS endpoint is reachable. Use this when Home Assistant lives outside the cluster. |
| Inside the Kubernetes cluster | `http://home-finance.<namespace>.svc.cluster.local/api/integrations/v1` | Service port 80 forwards to the app on 8080. Skips the ingress/gateway, but only works if the NetworkPolicy allows your client. |

If the chart's Cilium `networkPolicy` is enabled, in-cluster clients must be allowed to reach
the app on port 8080 (see the comments in `deploy/helm/home-finance/values.yaml`). Traffic that
arrives through a **hostNetwork** gateway cannot be matched by a pod selector, which is why the
chart has a dedicated `gatewayIngress` setting. If the public URL works but an in-cluster URL
times out, the NetworkPolicy is the first suspect.

The examples below use the public URL, and `$TOKEN` holds the token:

```bash
export TOKEN='hf_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'
export BASE='https://budget.app.example.com/api/integrations/v1'
```

## 4. Endpoint reference

```
/api/integrations/v1                      (Bearer token)
├── GET /info                             token name + scopes (any valid token)
├── /kids                                 scope kids:read
│   ├── GET /kids                         all active kids (summary) + household totals
│   ├── GET /kids/balances                legacy summary without slugs (kept unchanged)
│   └── /kids/{slug}
│       ├── GET /kids/{slug}              one kid, full detail (both sub-pots + combined)
│       ├── GET /kids/{slug}/ours         sub-pot "door ons gespaard"
│       └── GET /kids/{slug}/theirs       sub-pot "eigen geld"
├── /pots                                 scope pots:read
│   ├── GET /pots                         all active pots (summary) + household totals
│   └── GET /pots/{slug}                  one pot, full detail
├── /periods                              scope periods:read
│   ├── GET /periods/current              totals of the current month (see how it is picked)
│   └── GET /periods/{yyyy-mm}            totals of one month
└── /years                                scope years:read
    ├── GET /years                        totals of every year, oldest first
    ├── GET /years/current                totals of the current year (zeros if no data yet)
    └── GET /years/{yyyy}                 totals of one year
```

`{slug}` is the kid's or pot's name slug, see [Slugs](#slugs). An unknown or archived slug returns
`404 not_found`. Any other path under `/api/integrations/v1` also returns a JSON 404, and a
wrong method returns a JSON 405.

### GET /info

Scope: any valid token. Returns the token's name and scopes. A cheap way to validate a token.

```bash
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/info"
```

```json
{
  "apiVersion": 1,
  "tokenName": "Home Assistant",
  "scopes": ["kids:read", "pots:read"]
}
```

| Field | Type | Meaning | Example |
|---|---|---|---|
| `apiVersion` | integer | Major version of this API. | `1` |
| `tokenName` | string | The name given to the token in Settings. | `"Home Assistant"` |
| `scopes` | string[] | Scopes the token holds. | `["kids:read", "pots:read"]` |

### GET /kids

Scope: `kids:read`. All active kids (ordered as in the app) with their balances, plus household
totals. Use it for a household total and to discover slugs.

```bash
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/kids"
```

```json
{
  "kids": [
    {
      "slug": "sam",
      "name": "Sam",
      "totalCents": 104000,
      "oursCents": 87500,
      "theirsCents": 16500
    },
    {
      "slug": "noor",
      "name": "Noor",
      "totalCents": 52000,
      "oursCents": 50000,
      "theirsCents": 2000
    },
    {
      "slug": "mila",
      "name": "Mila",
      "totalCents": 25300,
      "oursCents": 22300,
      "theirsCents": 3000
    }
  ],
  "totals": {
    "totalCents": 181300,
    "oursCents": 159800,
    "theirsCents": 21500
  },
  "generatedAt": "2026-11-08T13:16:15Z"
}
```

**Response**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `kids` | KidSummary[] | | Active kids, in app order. | |
| `totals` | Totals | | Sum over all listed kids. | |
| `generatedAt` | string | RFC 3339, UTC | When the server built the response. | `"2026-11-08T13:16:15Z"` |

**KidSummary**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `slug` | string | | URL key for the per-kid endpoints. See [Slugs](#slugs). | `"sam"` |
| `name` | string | | Display name. | `"Sam"` |
| `totalCents` | integer | cents | Whole balance: `oursCents + theirsCents`. | `104000` |
| `oursCents` | integer | cents | Balance of the "door ons gespaard" sub-pot (`ours`). | `87500` |
| `theirsCents` | integer | cents | Balance of the "eigen geld" sub-pot (`theirs`). | `16500` |

**Totals**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `totalCents` | integer | cents | Sum of every kid's `totalCents`. | `181300` |
| `oursCents` | integer | cents | Sum of every kid's `oursCents`. | `159800` |
| `theirsCents` | integer | cents | Sum of every kid's `theirsCents`. | `21500` |

### GET /kids/balances

Scope: `kids:read`. The original phase-1 endpoint, kept byte-for-byte so existing configurations
keep working. It is the same as `GET /kids` minus the `slug` field and the `totals` object. New
integrations should use `GET /kids`.

```bash
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/kids/balances"
```

```json
{
  "kids": [
    {
      "name": "Sam",
      "totalCents": 104000,
      "oursCents": 87500,
      "theirsCents": 16500
    }
  ],
  "generatedAt": "2026-11-08T13:16:15Z"
}
```

(Shortened to one kid; the real response lists every active kid.) Field meanings are as in
KidSummary above.

### GET /kids/{slug}

Scope: `kids:read`. Everything about one kid: both sub-pots, the combined flows, the bank
reconciliation and the most recent entry. This is the endpoint to poll from Home Assistant.

```bash
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/kids/sam"
```

```json
{
  "slug": "sam",
  "name": "Sam",
  "totalCents": 104000,
  "combined": {
    "openingBalanceCents": 95000,
    "depositsCents": 12500,
    "withdrawalsCents": 3500,
    "adjustmentsCents": 0,
    "thisMonth": {
      "depositsCents": 3000,
      "withdrawalsCents": 0,
      "adjustmentsCents": 0,
      "netCents": 3000
    },
    "thisYear": {
      "depositsCents": 6500,
      "withdrawalsCents": 3500,
      "adjustmentsCents": 0,
      "netCents": 3000
    }
  },
  "pots": {
    "ours": {
      "owner": "ours",
      "balanceCents": 87500,
      "openingBalanceCents": 80000,
      "depositsCents": 7500,
      "withdrawalsCents": 0,
      "adjustmentsCents": 0,
      "thisMonth": {
        "depositsCents": 2500,
        "withdrawalsCents": 0,
        "adjustmentsCents": 0,
        "netCents": 2500
      },
      "thisYear": {
        "depositsCents": 5000,
        "withdrawalsCents": 0,
        "adjustmentsCents": 0,
        "netCents": 5000
      },
      "lastEntry": {
        "date": "2026-11-02",
        "type": "deposit",
        "owner": "ours",
        "amountCents": 2500
      }
    },
    "theirs": {
      "owner": "theirs",
      "balanceCents": 16500,
      "openingBalanceCents": 15000,
      "depositsCents": 5000,
      "withdrawalsCents": 3500,
      "adjustmentsCents": 0,
      "thisMonth": {
        "depositsCents": 500,
        "withdrawalsCents": 0,
        "adjustmentsCents": 0,
        "netCents": 500
      },
      "thisYear": {
        "depositsCents": 1500,
        "withdrawalsCents": 3500,
        "adjustmentsCents": 0,
        "netCents": -2000
      },
      "lastEntry": {
        "date": "2026-11-01",
        "type": "deposit",
        "owner": "theirs",
        "amountCents": 500
      }
    }
  },
  "bank": {
    "reportedBalanceCents": 103500,
    "reportedBalanceDate": "2026-11-01",
    "differenceCents": 500
  },
  "lastEntry": {
    "date": "2026-11-02",
    "type": "deposit",
    "owner": "ours",
    "amountCents": 2500
  },
  "generatedAt": "2026-11-08T13:16:15Z"
}
```

Key order in a real response may differ (JSON objects are unordered); do not depend on it.

**KidDetail**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `slug` | string | | The slug you requested. | `"sam"` |
| `name` | string | | Display name. | `"Sam"` |
| `totalCents` | integer | cents | `pots.ours.balanceCents + pots.theirs.balanceCents`. | `104000` |
| `combined` | Flows | | Both sub-pots added together, field by field. Has no `balanceCents`; use `totalCents`. | |
| `pots` | object | | Always has exactly the keys `ours` and `theirs`, even when a sub-pot has no entries. | |
| `pots.ours` | SubPot | | "Door ons gespaard". | |
| `pots.theirs` | SubPot | | "Eigen geld". | |
| `bank` | Bank | | Reconciliation against the balance reported by the bank. | |
| `lastEntry` | LastEntry or `null` | | The most recent ledger entry across both sub-pots; `null` when the kid has no entries. | |
| `generatedAt` | string | RFC 3339, UTC | When the server built the response. | `"2026-11-08T13:16:15Z"` |

**SubPot**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `owner` | string | | `"ours"` or `"theirs"`. | `"ours"` |
| `balanceCents` | integer | cents | Current balance of this sub-pot (sum of all its entries). | `87500` |
| *(Flows fields)* | | | `openingBalanceCents`, `depositsCents`, `withdrawalsCents`, `adjustmentsCents`, `thisMonth`, `thisYear`, flattened into the sub-pot object. See Flows. | |
| `lastEntry` | LastEntry or `null` | | Most recent entry in this sub-pot; `null` when it has none. | |

**Flows**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `openingBalanceCents` | integer | cents | Sum of `opening_balance` entries. | `80000` |
| `depositsCents` | integer | cents | Sum of deposits (positive). | `7500` |
| `withdrawalsCents` | integer | cents | Sum of withdrawals as a **positive magnitude**. | `3500` |
| `adjustmentsCents` | integer | cents | Sum of adjustments. **Signed** (a correction can be negative). | `0` |
| `thisMonth` | Period | | Entries dated in the current calendar month (Europe/Amsterdam). | |
| `thisYear` | Period | | Entries dated in the current calendar year (Europe/Amsterdam). | |

Invariant: `openingBalanceCents + depositsCents - withdrawalsCents + adjustmentsCents == balanceCents`
(for `combined`, equal to `totalCents`).

**Period**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `depositsCents` | integer | cents | Deposits in the period (positive). | `2500` |
| `withdrawalsCents` | integer | cents | Withdrawals in the period, as a **positive magnitude**. | `0` |
| `adjustmentsCents` | integer | cents | Adjustments in the period (signed). | `0` |
| `netCents` | integer | cents | `depositsCents - withdrawalsCents + adjustmentsCents`. Can be negative. | `2500` |

The opening balance is never part of a period. See [Concepts](#5-concepts).

**Bank**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `reportedBalanceCents` | integer or `null` | cents | The balance last reported from the bank, per kid (not per sub-pot). `null` if none was recorded. | `103500` |
| `reportedBalanceDate` | string or `null` | `YYYY-MM-DD` | The date that reported balance applies to. | `"2026-11-01"` |
| `differenceCents` | integer or `null` | cents | `totalCents - reportedBalanceCents`. Positive means the ledger holds more than the bank reports. `null` when there is no reported balance. | `500` |

**LastEntry**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `date` | string | `YYYY-MM-DD` | The entry date. Latest by date, then by creation order. | `"2026-11-02"` |
| `type` | string | | `opening_balance`, `deposit`, `withdrawal` or `adjustment`. | `"deposit"` |
| `owner` | string | | `"ours"` or `"theirs"`. | `"ours"` |
| `amountCents` | integer | cents | **Signed** ledger amount: withdrawals are negative here (e.g. `-3500`), unlike the positive `withdrawalsCents` magnitudes elsewhere. | `2500` |

There is deliberately no description or id.

### GET /kids/{slug}/ours and GET /kids/{slug}/theirs

Scope: `kids:read`. A single sub-pot: `ours` is "door ons gespaard", `theirs` is "eigen geld".
The body is the SubPot object (see the table above) with `slug`, `name` and `generatedAt` added
at the top level.

```bash
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/kids/sam/theirs"
```

```json
{
  "slug": "sam",
  "name": "Sam",
  "owner": "theirs",
  "balanceCents": 16500,
  "openingBalanceCents": 15000,
  "depositsCents": 5000,
  "withdrawalsCents": 3500,
  "adjustmentsCents": 0,
  "thisMonth": {
    "depositsCents": 500,
    "withdrawalsCents": 0,
    "adjustmentsCents": 0,
    "netCents": 500
  },
  "thisYear": {
    "depositsCents": 1500,
    "withdrawalsCents": 3500,
    "adjustmentsCents": 0,
    "netCents": -2000
  },
  "lastEntry": {
    "date": "2026-11-01",
    "type": "deposit",
    "owner": "theirs",
    "amountCents": 500
  },
  "generatedAt": "2026-11-08T13:16:15Z"
}
```

These endpoints contain no `bank` object, because the reported balance belongs to the kid as a
whole. Use `GET /kids/{slug}` for that.

### GET /pots

Scope: `pots:read`. All active savings pots (ordered as in the app) with their balances and
savings goals, plus household totals. Use it for a household total and to discover slugs.

```bash
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/pots"
```

```json
{
  "pots": [
    {
      "slug": "vacation",
      "name": "Vacation",
      "kind": "normal",
      "balanceCents": 195000,
      "targetCents": 300000,
      "progressPercent": 65
    },
    {
      "slug": "new-car",
      "name": "New car",
      "kind": "normal",
      "balanceCents": 120000,
      "targetCents": 400000,
      "progressPercent": 30
    },
    {
      "slug": "buffer",
      "name": "Buffer",
      "kind": "normal",
      "balanceCents": 250000,
      "targetCents": null,
      "progressPercent": null
    },
    {
      "slug": "carryover",
      "name": "Carryover",
      "kind": "carryover",
      "balanceCents": 15000,
      "targetCents": null,
      "progressPercent": null
    }
  ],
  "totals": {
    "balanceCents": 580000,
    "targetCents": 700000
  },
  "generatedAt": "2026-11-08T14:00:59Z"
}
```

**Response**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `pots` | PotSummary[] | | Active pots, in app order. | |
| `totals` | PotTotals | | Sum over all listed pots. | |
| `generatedAt` | string | RFC 3339, UTC | When the server built the response. | `"2026-11-08T14:00:59Z"` |

**PotSummary**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `slug` | string | | URL key for the per-pot endpoint. See [Slugs](#slugs). | `"vacation"` |
| `name` | string | | Display name. | `"Vacation"` |
| `kind` | string | | `"normal"` or `"carryover"`. See [The carryover pot](#the-carryover-pot). | `"normal"` |
| `balanceCents` | integer | cents | Current balance (sum of every ledger entry of the pot). | `195000` |
| `targetCents` | integer or `null` | cents | The savings goal; `null` when the pot has none. | `300000` |
| `progressPercent` | number or `null` | percent | `balanceCents / targetCents * 100`, rounded to 1 decimal, **not capped** at 100. `null` when there is no target or it is zero or less. | `65` |

**PotTotals**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `balanceCents` | integer | cents | Sum of every listed pot's `balanceCents`. | `580000` |
| `targetCents` | integer | cents | Sum of `targetCents` over the pots that have a target (pots without one do not count). | `700000` |

### GET /pots/{slug}

Scope: `pots:read`. Everything about one pot: the balance broken down by entry type, this month
and this year, the savings goal, the split percentage and the most recent entry. This is the
endpoint to poll from Home Assistant.

```bash
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/pots/vacation"
```

```json
{
  "slug": "vacation",
  "name": "Vacation",
  "kind": "normal",
  "balanceCents": 195000,
  "flows": {
    "openingBalanceCents": 50000,
    "allocationsCents": 180000,
    "depositsCents": 20000,
    "withdrawalsCents": 50000,
    "carryoverOutCents": 0,
    "adjustmentsCents": -5000,
    "thisMonth": {
      "allocationsCents": 0,
      "depositsCents": 5000,
      "withdrawalsCents": 0,
      "carryoverOutCents": 0,
      "adjustmentsCents": 0,
      "netCents": 5000
    },
    "thisYear": {
      "allocationsCents": 150000,
      "depositsCents": 15000,
      "withdrawalsCents": 50000,
      "carryoverOutCents": 0,
      "adjustmentsCents": -5000,
      "netCents": 110000
    }
  },
  "target": {
    "targetCents": 300000,
    "targetDate": "2027-06-01",
    "remainingCents": 105000,
    "progressPercent": 65,
    "reached": false,
    "monthsLeft": 7,
    "neededPerMonthCents": 15000
  },
  "split": {
    "percentage": 25,
    "periodYear": 2026,
    "periodMonth": 11,
    "periodStatus": "open"
  },
  "lastEntry": {
    "date": "2026-11-02",
    "type": "deposit",
    "amountCents": 5000
  },
  "generatedAt": "2026-11-08T14:00:59Z"
}
```

Key order in a real response may differ (JSON objects are unordered); do not depend on it.

**PotDetail**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `slug` | string | | The slug you requested. | `"vacation"` |
| `name` | string | | Display name. | `"Vacation"` |
| `kind` | string | | `"normal"` or `"carryover"`. | `"normal"` |
| `balanceCents` | integer | cents | Current balance: the sum of every ledger entry of the pot. | `195000` |
| `flows` | PotFlows | | The balance broken down by entry type, plus this month and this year. | |
| `target` | Target or `null` | | The savings goal; `null` when the pot has no target. | |
| `split` | Split or `null` | | The pot's share of the monthly surplus; `null` when the pot has never had a split. | |
| `lastEntry` | LastEntry or `null` | | The most recent ledger entry; `null` when the pot has no entries. | |
| `generatedAt` | string | RFC 3339, UTC | When the server built the response. | `"2026-11-08T14:00:59Z"` |

**PotFlows**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `openingBalanceCents` | integer | cents | Sum of `opening_balance` entries. | `50000` |
| `allocationsCents` | integer | cents | Sum of allocations generated by month closes (positive). | `180000` |
| `depositsCents` | integer | cents | Sum of manual deposits (positive). | `20000` |
| `withdrawalsCents` | integer | cents | Sum of withdrawals as a **positive magnitude**. | `50000` |
| `carryoverOutCents` | integer | cents | Sum of `carryover_out` entries as a **positive magnitude**. Only ever non-zero for the carryover pot. | `0` |
| `adjustmentsCents` | integer | cents | Sum of adjustments. **Signed** (a correction can be negative). | `-5000` |
| `thisMonth` | PotPeriod | | Entries belonging to the current calendar month (Europe/Amsterdam). See [Period bucketing](#period-bucketing-for-pots). | |
| `thisYear` | PotPeriod | | Entries belonging to the current calendar year (Europe/Amsterdam). | |

Invariant: `openingBalanceCents + allocationsCents + depositsCents - withdrawalsCents - carryoverOutCents + adjustmentsCents == balanceCents`.

**PotPeriod**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `allocationsCents` | integer | cents | Allocations in the period (positive). | `0` |
| `depositsCents` | integer | cents | Deposits in the period (positive). | `5000` |
| `withdrawalsCents` | integer | cents | Withdrawals in the period, as a **positive magnitude**. | `0` |
| `carryoverOutCents` | integer | cents | Carryover-out in the period, as a **positive magnitude**. | `0` |
| `adjustmentsCents` | integer | cents | Adjustments in the period (signed). | `0` |
| `netCents` | integer | cents | `allocationsCents + depositsCents - withdrawalsCents - carryoverOutCents + adjustmentsCents`. Can be negative. | `5000` |

The opening balance is never part of a period.

**Target**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `targetCents` | integer | cents | The savings goal. | `300000` |
| `targetDate` | string or `null` | `YYYY-MM-DD` | The date the goal should be reached by; `null` when the goal has no date. | `"2027-06-01"` |
| `remainingCents` | integer | cents | `max(targetCents - balanceCents, 0)`. | `105000` |
| `progressPercent` | number or `null` | percent | As on PotSummary: 1 decimal, **not capped** (a pot at 120% of its goal shows `120`). `null` when `targetCents` is zero or less. | `65` |
| `reached` | boolean | | `balanceCents >= targetCents`. | `false` |
| `monthsLeft` | integer or `null` | months | Whole calendar months from the current month (Europe/Amsterdam) to the month of `targetDate`, never below 0. `null` without a date. | `7` |
| `neededPerMonthCents` | integer or `null` | cents | What must be saved per month to make it: `ceil(remainingCents / max(monthsLeft, 1))`, and `0` when `reached`. `null` without a date. | `15000` |

**Split**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `percentage` | number | percent | The share of the monthly surplus this pot receives in the reference period. | `25` |
| `periodYear` | integer | | Year of the reference period. | `2026` |
| `periodMonth` | integer | | Month (1-12) of the reference period. | `11` |
| `periodStatus` | string | | `"open"` or `"closed"`. | `"open"` |

Which period is used is explained under [Split reference period](#split-reference-period).

**LastEntry**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `date` | string | `YYYY-MM-DD` | The entry date. Latest by date, then by creation order. | `"2026-11-02"` |
| `type` | string | | `opening_balance`, `allocation`, `deposit`, `withdrawal`, `carryover_out` or `adjustment`. | `"deposit"` |
| `amountCents` | integer | cents | **Signed** ledger amount: withdrawals and carryover-out are negative here (e.g. `-45000`), unlike the positive `withdrawalsCents` and `carryoverOutCents` magnitudes elsewhere. | `5000` |

There is deliberately no description and no id.

The carryover pot has the same shape. This is what a closed October looks like on it (the
surplus share arrived as an allocation and the carryover into November left again):

```json
{
  "slug": "carryover",
  "name": "Carryover",
  "kind": "carryover",
  "balanceCents": 15000,
  "flows": {
    "openingBalanceCents": 0,
    "allocationsCents": 60000,
    "depositsCents": 0,
    "withdrawalsCents": 0,
    "carryoverOutCents": 45000,
    "adjustmentsCents": 0,
    "thisMonth": {
      "allocationsCents": 0,
      "depositsCents": 0,
      "withdrawalsCents": 0,
      "carryoverOutCents": 0,
      "adjustmentsCents": 0,
      "netCents": 0
    },
    "thisYear": {
      "allocationsCents": 60000,
      "depositsCents": 0,
      "withdrawalsCents": 0,
      "carryoverOutCents": 45000,
      "adjustmentsCents": 0,
      "netCents": 15000
    }
  },
  "target": null,
  "split": {
    "percentage": 15,
    "periodYear": 2026,
    "periodMonth": 11,
    "periodStatus": "open"
  },
  "lastEntry": {
    "date": "2026-11-03",
    "type": "carryover_out",
    "amountCents": -45000
  },
  "generatedAt": "2026-11-08T14:00:59Z"
}
```

Note that `thisMonth` is all zeros although the last entry is dated in November: that entry
belongs to the October period (see [Period bucketing](#period-bucketing-for-pots)).

### GET /periods/current

Scope: `periods:read`. The effective totals of "the current month". Which period that is when the
current month has none is explained under [Picking the current period](#picking-the-current-period);
`isCurrentMonth` tells you which case you got. This is the endpoint to poll from Home Assistant.

```bash
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/periods/current"
```

```json
{
  "year": 2026,
  "month": 11,
  "status": "open",
  "closedAt": null,
  "isCurrentMonth": true,
  "incomeCents": 485000,
  "expenseCents": 312500,
  "surplusCents": 172500,
  "generatedAt": "2026-11-08T14:30:12Z"
}
```

**PeriodTotals**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `year` | integer | | Year of the period. | `2026` |
| `month` | integer | | Month of the period, 1-12. | `11` |
| `status` | string | | `"open"` or `"closed"`. | `"open"` |
| `closedAt` | string or `null` | RFC 3339, UTC | When the month was closed; `null` while it is open. | `null` |
| `isCurrentMonth` | boolean | | `true` when the period is the current calendar month in Europe/Amsterdam. `false` means `/periods/current` fell back to another period. | `true` |
| `incomeCents` | integer | cents | Effective income of the month. See [Effective totals](#effective-totals). | `485000` |
| `expenseCents` | integer | cents | Effective expenses of the month. | `312500` |
| `surplusCents` | integer | cents | `incomeCents - expenseCents`. **Can be negative.** | `172500` |
| `generatedAt` | string | RFC 3339, UTC | When the server built the response. | `"2026-11-08T14:30:12Z"` |

There is deliberately no period id.

Errors: `404 not_found` when no periods exist at all, `403 insufficient_scope` without
`periods:read`.

### GET /periods/{yyyy-mm}

Scope: `periods:read`. The same PeriodTotals object for one specific month, for example
`2026-10`. The path must be exactly four digits, a dash and two digits, with the month in 01-12.

```bash
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/periods/2026-10"
```

```json
{
  "year": 2026,
  "month": 10,
  "status": "closed",
  "closedAt": "2026-11-03T08:12:44Z",
  "isCurrentMonth": false,
  "incomeCents": 480000,
  "expenseCents": 441250,
  "surplusCents": 38750,
  "generatedAt": "2026-11-08T14:30:12Z"
}
```

Fields are as in PeriodTotals above.

Errors: `404 not_found` for a malformed path (`2026-1`, `2026-10-01`, `latest`), a month outside
01-12 (`2026-13`) or a month that has no period; `403 insufficient_scope` without `periods:read`.

### GET /years

Scope: `years:read`. The totals of every year, oldest first: every year in the app's years list
plus any year that has periods. Use it for a multi-year overview.

```bash
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/years"
```

```json
{
  "years": [
    {
      "year": 2025,
      "locked": true,
      "periodsOpen": 0,
      "periodsClosed": 12,
      "incomeCents": 5640000,
      "expenseCents": 5128500,
      "surplusCents": 511500,
      "averageMonthlySurplusCents": 42625
    },
    {
      "year": 2026,
      "locked": false,
      "periodsOpen": 1,
      "periodsClosed": 10,
      "incomeCents": 5310000,
      "expenseCents": 4688750,
      "surplusCents": 621250,
      "averageMonthlySurplusCents": 56477
    },
    {
      "year": 2027,
      "locked": false,
      "periodsOpen": 0,
      "periodsClosed": 0,
      "incomeCents": 0,
      "expenseCents": 0,
      "surplusCents": 0,
      "averageMonthlySurplusCents": 0
    }
  ],
  "generatedAt": "2026-11-08T14:30:12Z"
}
```

**Response**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `years` | YearTotals[] | | Every known year, oldest first. | |
| `generatedAt` | string | RFC 3339, UTC | When the server built the response. | `"2026-11-08T14:30:12Z"` |

**YearTotals**

| Field | Type | Unit | Meaning | Example |
|---|---|---|---|---|
| `year` | integer | | The year. | `2026` |
| `locked` | boolean | | `true` when the year is locked in the app (no more edits). | `false` |
| `periodsOpen` | integer | | Number of open month periods in the year. | `1` |
| `periodsClosed` | integer | | Number of closed month periods in the year. | `10` |
| `incomeCents` | integer | cents | Effective income summed over all the year's periods, open ones included. | `5310000` |
| `expenseCents` | integer | cents | Effective expenses summed over all the year's periods. | `4688750` |
| `surplusCents` | integer | cents | `incomeCents - expenseCents`. **Can be negative.** | `621250` |
| `averageMonthlySurplusCents` | integer | cents | `surplusCents / (periodsOpen + periodsClosed)`, rounded half away from zero; `0` with no periods. See [Year totals](#year-totals). | `56477` |

Errors: `403 insufficient_scope` without `years:read`.

### GET /years/current

Scope: `years:read`. The YearTotals object of the current Europe/Amsterdam calendar year, with
`generatedAt` added at the top level. Unlike `/years/{yyyy}` it never returns 404: a year without
any data yet returns `200` with zeros. This is the endpoint to poll from Home Assistant.

```bash
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/years/current"
```

```json
{
  "year": 2026,
  "locked": false,
  "periodsOpen": 1,
  "periodsClosed": 10,
  "incomeCents": 5310000,
  "expenseCents": 4688750,
  "surplusCents": 621250,
  "averageMonthlySurplusCents": 56477,
  "generatedAt": "2026-11-08T14:30:12Z"
}
```

Fields are as in YearTotals above, plus `generatedAt`. On 1 January, before the new year has any
data, the body is the same shape with `periodsOpen`, `periodsClosed`, `incomeCents`,
`expenseCents`, `surplusCents` and `averageMonthlySurplusCents` all `0` and `locked` `false`.

Errors: `403 insufficient_scope` without `years:read`.

### GET /years/{yyyy}

Scope: `years:read`. The same object as `/years/current` for one specific year, for example
`2025`. The path must be exactly four digits in 2000-2100.

```bash
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/years/2025"
```

```json
{
  "year": 2025,
  "locked": true,
  "periodsOpen": 0,
  "periodsClosed": 12,
  "incomeCents": 5640000,
  "expenseCents": 5128500,
  "surplusCents": 511500,
  "averageMonthlySurplusCents": 42625,
  "generatedAt": "2026-11-08T14:30:12Z"
}
```

Errors: `404 not_found` for a malformed path (`25`, `latest`), a year outside 2000-2100 or a year
that is neither in the years list nor has periods; `403 insufficient_scope` without `years:read`.

## 5. Concepts

### Two sub-pots per kid

Every kid's money is split by who contributed it:

| Dutch term | Meaning | API owner / key |
|---|---|---|
| **eigen geld** | The kid's own money (pocket money, gifts) | `theirs` |
| **door ons gespaard** | Saved by us, the parents | `ours` |

The kid's total is the sum of both.

### Flows and the invariant

For each sub-pot (and for `combined`):

```
openingBalanceCents + depositsCents - withdrawalsCents + adjustmentsCents == balance
```

`withdrawalsCents` is reported as a **positive magnitude** and is subtracted. Adjustments are
signed and added. `combined` is `ours + theirs` field by field, and
`totalCents == pots.ours.balanceCents + pots.theirs.balanceCents`. The one place a withdrawal
shows up negative is `lastEntry.amountCents`, which is the raw signed ledger amount.

### This month and this year

`thisMonth` and `thisYear` cover the calendar month and year **in the Europe/Amsterdam time
zone**, matched on each entry's **entry date** (not when it was recorded). The **opening balance
is excluded**, so periods only describe real movement. They roll over at local midnight, so a
"deposited this month" sensor resets to 0 on the 1st.

### Bank difference

`bank.differenceCents = totalCents - reportedBalanceCents`, that is, the app's calculated total
minus the balance reported from the bank. Zero means ledger and bank agree. It is `null` (not
`0`) when no reported balance has been recorded, so treat `null` as "unknown".

### Pot flows and the invariant

For every pot:

```
openingBalanceCents + allocationsCents + depositsCents - withdrawalsCents - carryoverOutCents + adjustmentsCents == balanceCents
```

`withdrawalsCents` and `carryoverOutCents` are reported as **positive magnitudes** and are
subtracted; adjustments are signed and added. The one place they show up negative is
`lastEntry.amountCents`, the raw signed ledger amount. In `GET /pots`, the household
`totals.balanceCents` is the sum of the listed pots' balances.

### Period bucketing for pots

`thisMonth` and `thisYear` for pots are bucketed differently from kids. A kid's entries are
matched on their **entry date** only. A pot's ledger row that belongs to a **month period** counts
in that period's year and month; only a row without a period (a manual deposit or withdrawal)
falls back to its entry date. The practical effect: the allocation generated by closing October
counts in **October**, even when the close happens on 3 November (and the row's entry date is in
November). So in the first days of a month `thisMonth.allocationsCents` is typically `0`, and an
allocation shows up in the month it was *earned*, not the month it was booked. "This month" and
"this year" are the current Europe/Amsterdam calendar month and year, and the opening balance is
excluded. This is the same convention as the app's Trends page.

### Target maths

For a pot with a target:

- `remainingCents = max(targetCents - balanceCents, 0)`; `reached` is `balanceCents >= targetCents`.
- `progressPercent` is not capped, so an over-achieved goal reads above 100. It is `null` when
  the target is zero or less.
- `monthsLeft` counts calendar months from the **current Amsterdam month** to the month of
  `targetDate` (in November 2026, a goal dated `2027-06-01` has 7 months left). It is `0` in the
  target month and for any date in the past, and `null` without a date.
- `neededPerMonthCents = ceil(remainingCents / max(monthsLeft, 1))`: in the target month or past
  it, the whole remainder is "needed this month". It is `0` once `reached`, and `null` without
  a date.

### Split reference period

`split.percentage` is the pot's share of the monthly surplus. It is stored per month period, so
the API has to choose one: the **latest open period** if the pot has a split there, otherwise the
**latest period** (open or closed) in which the pot has a split. `periodYear`, `periodMonth` and
`periodStatus` tell you which one was used. `split` is `null` when the pot has never had a split
in any period.

### The carryover pot

At most one active pot has `kind` `carryover`. It receives its share of the surplus like any other
pot, but closing a month moves that money on as income of the next month, which is booked as a
`carryover_out` entry. That is why `carryoverOutCents` is only non-zero for this pot, and why its
balance is usually small. Its split percentage is not user-editable: it is always the remainder
after the other pots' percentages.

### Effective totals

`incomeCents` and `expenseCents` are *effective* amounts, the same numbers the app's month and
year screens show:

- An **itemized income source** counts the sum of its income transactions instead of the amount
  on its row.
- A **tracked budget line** counts the sum of its transactions instead of the amount on its row,
  with child categories rolled into the parent line.
- Every other line counts the amount entered on it. Budget targets never count.

`surplusCents` is `incomeCents - expenseCents`, and it can be **negative** when a month or year is
overspent. Do not clamp it, and do not assume it is positive in templates.

### Picking the current period

`GET /periods/current` returns the first match of:

1. the period for the current Europe/Amsterdam calendar month;
2. otherwise the latest **open** period;
3. otherwise the latest period of any status.

`isCurrentMonth` is `true` only in the first case. A `false` means the current month has no period
yet (for example before it is created in the app), so the numbers describe an earlier month; use
`year`, `month` and `status` to show which one. When no periods exist at all the endpoint returns
`404 not_found`.

### Year totals

- Year totals include **open** periods, so for the current year they are **year-to-date**.
- `averageMonthlySurplusCents` is `surplusCents` divided by the number of periods
  (`periodsOpen + periodsClosed`), rounded **half away from zero** (`-2.5` becomes `-3`, `2.5`
  becomes `3`). It is `0` when the year has no periods, rather than an error or `null`. Note that
  it divides by the periods that exist, not by 12.
- The `GET /years` list is the app's years list plus any year that has periods, oldest first, so a
  year that only exists through its periods still appears.
- `GET /years/current` returns zeros with `200` for a year with no data yet (instead of 404), so a
  Home Assistant sensor keeps working on 1 January. `GET /years/{yyyy}` does return 404 for a
  year the app does not know.

### Slugs

The `{slug}` in a kid's or pot's URL is derived from the name:

- lowercased, diacritics folded (`Zoë` becomes `zoe`, `ß` becomes `ss`, `ø` becomes `o`);
- every run of characters outside `a-z0-9` becomes a single `-`, leading and trailing dashes are
  trimmed, at most 64 characters (`Anna-Lisa` and `Anna Lisa` both give `anna-lisa`);
- **collisions:** names are not unique. If two or more active kids produce the same slug, every
  one of them gets `<slug>-<id>` (for example `sam-12` and `sam-31`). The suffix is the kid's
  internal id, so it is stable;
- the slug `balances` is reserved for kids (it is a static route), so a kid named "Balances" gets
  `balances-<id>`; pots have no reserved slugs;
- a name with no letters or digits at all gets `kid-<id>` (or `pot-<id>` for a pot).

Kid slugs and pot slugs are independent: a kid and a pot may share a slug, because they live
under different paths.

A slug is only valid if it matches `^[a-z0-9-]{1,64}$`; anything else is a 404.

**A rename changes the slug.** Renaming "Sam" to "Samuel" turns `/kids/sam` into a 404 and
`/kids/samuel` starts working (the same holds for pots). Adding a second kid with the same name also changes both slugs to
the suffixed form. **Archiving a kid or pot removes the slug (404).** Integrations should therefore
discover slugs from `GET /kids` or `GET /pots` rather than hard-coding them if renames are likely, and treat a
sudden 404 as "renamed or archived".

## 6. Home Assistant cookbook

All snippets are copy-paste ready for a standard Home Assistant install; replace
`budget.app.example.com` with your host. Entity IDs below are what Home Assistant generates from
the sensor names shown.

### Secret

`secrets.yaml`:

```yaml
home_finance_auth: "Bearer hf_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
```

The value includes the word `Bearer` and a space, because the secret is used as the whole header
value.

### One kid: one poll, many sensors

The `rest:` integration polls the resource once and every `sensor:` entry underneath derives its
state from that single response. Add to `configuration.yaml` (or `rest: !include rest.yaml`).
This is Sam; repeat the whole resource for each kid with its own slug, names and `unique_id`s.

```yaml
rest:
  - resource: https://budget.app.example.com/api/integrations/v1/kids/sam
    headers:
      Authorization: !secret home_finance_auth
      Accept: application/json
    scan_interval: 900
    timeout: 20
    sensor:
      - name: "Sam savings total"
        unique_id: home_finance_sam_total
        value_template: "{{ ((value_json.totalCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
        state_class: total

      - name: "Sam own money"
        unique_id: home_finance_sam_theirs
        value_template: "{{ ((value_json.pots.theirs.balanceCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
        state_class: total

      - name: "Sam saved by us"
        unique_id: home_finance_sam_ours
        value_template: "{{ ((value_json.pots.ours.balanceCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
        state_class: total

      - name: "Sam deposited this month"
        unique_id: home_finance_sam_deposited_month
        value_template: "{{ ((value_json.combined.thisMonth.depositsCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
        state_class: total

      - name: "Sam deposited this year"
        unique_id: home_finance_sam_deposited_year
        value_template: "{{ ((value_json.combined.thisYear.depositsCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
        state_class: total

      # bank.differenceCents is null until a reported balance is recorded, so this
      # sensor is unavailable (not 0) in that case.
      - name: "Sam bank difference"
        unique_id: home_finance_sam_bank_difference
        availability: "{{ value_json.bank.differenceCents is number }}"
        value_template: "{{ ((value_json.bank.differenceCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
        state_class: total
        json_attributes_path: "$.bank"
        json_attributes:
          - reportedBalanceCents
          - reportedBalanceDate

      # State is the date of the latest entry; the rest arrives as attributes.
      # lastEntry is null for a kid without any entries, hence the availability check.
      - name: "Sam last entry"
        unique_id: home_finance_sam_last_entry
        availability: "{{ value_json.lastEntry is mapping }}"
        value_template: "{{ value_json.lastEntry.date }}"
        device_class: date
        json_attributes_path: "$.lastEntry"
        json_attributes:
          - type
          - owner
          - amountCents

      # Signed: a withdrawal is negative. No state_class: a "last amount" is not a total.
      - name: "Sam last entry amount"
        unique_id: home_finance_sam_last_entry_amount
        availability: "{{ value_json.lastEntry is mapping }}"
        value_template: "{{ ((value_json.lastEntry.amountCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
```

Notes:

- `| float(0)` gives a default of `0` instead of an error if a field is ever missing.
- `device_class: monetary` only allows `state_class: total` in Home Assistant, which is why
  every money sensor uses `total` (including the month and year deposits, even though those
  reset; `total_increasing` is not permitted for monetary sensors).
- `unique_id` lets you rename and customise entities in the UI. Pick ids once and keep them.
- `availability` marks the sensor `unavailable` instead of showing a wrong number. Its template
  can use `value_json`, like `value_template`.
- Entity IDs: `sensor.sam_savings_total`, `sensor.sam_own_money`, `sensor.sam_saved_by_us`,
  `sensor.sam_deposited_this_month`, `sensor.sam_deposited_this_year`,
  `sensor.sam_bank_difference`, `sensor.sam_last_entry`, `sensor.sam_last_entry_amount`.
- If the slug changes (rename) the resource starts returning 404 and all of that kid's sensors
  become `unavailable`; update the URL.

### One pot: one poll, many sensors

Same pattern for a savings pot, on `/pots/<slug>`. This is the "Vacation" pot; repeat the whole
resource for each pot with its own slug, names and `unique_id`s. Add it to the same `rest:` list
as the kids (and use a token with `pots:read`).

```yaml
rest:
  - resource: https://budget.app.example.com/api/integrations/v1/pots/vacation
    headers:
      Authorization: !secret home_finance_auth
      Accept: application/json
    scan_interval: 900
    timeout: 20
    sensor:
      - name: "Vacation balance"
        unique_id: home_finance_pot_vacation_balance
        value_template: "{{ ((value_json.balanceCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
        state_class: total

      # A percentage, not money: no device_class. Unavailable for a pot without a target.
      - name: "Vacation progress"
        unique_id: home_finance_pot_vacation_progress
        availability: "{{ value_json.target is mapping and value_json.target.progressPercent is number }}"
        value_template: "{{ value_json.target.progressPercent | float(0) }}"
        unit_of_measurement: "%"

      - name: "Vacation remaining"
        unique_id: home_finance_pot_vacation_remaining
        availability: "{{ value_json.target is mapping }}"
        value_template: "{{ ((value_json.target.remainingCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
        state_class: total

      # neededPerMonthCents is null for a goal without a date, so unavailable (not 0) then.
      - name: "Vacation needed per month"
        unique_id: home_finance_pot_vacation_needed_per_month
        availability: "{{ value_json.target is mapping and value_json.target.neededPerMonthCents is number }}"
        value_template: "{{ ((value_json.target.neededPerMonthCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
        state_class: total

      - name: "Vacation allocated this month"
        unique_id: home_finance_pot_vacation_allocated_month
        value_template: "{{ ((value_json.flows.thisMonth.allocationsCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
        state_class: total

      # split is null for a pot that never had a split.
      - name: "Vacation split"
        unique_id: home_finance_pot_vacation_split
        availability: "{{ value_json.split is mapping }}"
        value_template: "{{ value_json.split.percentage | float(0) }}"
        unit_of_measurement: "%"

    binary_sensor:
      - name: "Vacation goal reached"
        unique_id: home_finance_pot_vacation_goal_reached
        availability: "{{ value_json.target is mapping }}"
        value_template: "{{ value_json.target.reached }}"
```

Notes:

- The same conventions as the kids sensors apply (`float(0)`, `unique_id`, `monetary` with
  `state_class: total`).
- `progressPercent` is a percentage and can exceed 100, so it has the unit `%` and **no**
  `device_class`; `availability` hides it for a pot without a target.
- "Allocated this month" is usually `0` until the month is closed, and the allocation then counts
  for the month it was earned (see [Period bucketing](#period-bucketing-for-pots)).
- Entity IDs: `sensor.vacation_balance`, `sensor.vacation_progress`, `sensor.vacation_remaining`,
  `sensor.vacation_needed_per_month`, `sensor.vacation_allocated_this_month`,
  `sensor.vacation_split`, `binary_sensor.vacation_goal_reached`.
- A renamed or archived pot makes the resource return 404 and its sensors `unavailable`.

### This month and this year (budget totals)

Same pattern for the monthly totals on `/periods/current` (token with `periods:read`). Add it to
the same `rest:` list. The amounts are in cents, hence the division by 100:

```yaml
rest:
  - resource: https://budget.app.example.com/api/integrations/v1/periods/current
    headers:
      Authorization: !secret home_finance_auth
      Accept: application/json
    scan_interval: 900
    timeout: 20
    sensor:
      - name: "Budget month income"
        unique_id: home_finance_month_income
        value_template: "{{ ((value_json.incomeCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
        state_class: total

      - name: "Budget month expenses"
        unique_id: home_finance_month_expenses
        value_template: "{{ ((value_json.expenseCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
        state_class: total

      # Can be negative when the month is overspent.
      - name: "Budget month surplus"
        unique_id: home_finance_month_surplus
        value_template: "{{ ((value_json.surplusCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
        state_class: total

      # State is "open" or "closed"; isCurrentMonth tells whether /periods/current had to
      # fall back to another month.
      - name: "Budget month status"
        unique_id: home_finance_month_status
        value_template: "{{ value_json.status }}"
        json_attributes:
          - year
          - month
          - isCurrentMonth
          - closedAt
```

And the year-to-date totals on `/years/current` (token with `years:read`):

```yaml
rest:
  - resource: https://budget.app.example.com/api/integrations/v1/years/current
    headers:
      Authorization: !secret home_finance_auth
      Accept: application/json
    scan_interval: 900
    timeout: 20
    sensor:
      - name: "Budget year income"
        unique_id: home_finance_year_income
        value_template: "{{ ((value_json.incomeCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
        state_class: total

      - name: "Budget year expenses"
        unique_id: home_finance_year_expenses
        value_template: "{{ ((value_json.expenseCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
        state_class: total

      - name: "Budget year surplus"
        unique_id: home_finance_year_surplus
        value_template: "{{ ((value_json.surplusCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
        state_class: total

      - name: "Budget year average monthly surplus"
        unique_id: home_finance_year_average_monthly_surplus
        value_template: "{{ ((value_json.averageMonthlySurplusCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
        state_class: total
```

Notes:

- The same conventions as above apply (`float(0)`, `unique_id`, `monetary` with
  `state_class: total`).
- The year figures are year-to-date because open periods count (see
  [Year totals](#year-totals)). On 1 January they read `0` instead of going `unavailable`.
- The month sensors follow `/periods/current`; check `Budget month status` and its
  `isCurrentMonth` attribute if the numbers look like an old month.
- Entity IDs: `sensor.budget_month_income`, `sensor.budget_month_expenses`,
  `sensor.budget_month_surplus`, `sensor.budget_month_status`, `sensor.budget_year_income`,
  `sensor.budget_year_expenses`, `sensor.budget_year_surplus`,
  `sensor.budget_year_average_monthly_surplus`.

### Household total

A separate resource for the household total from `GET /kids`:

```yaml
rest:
  - resource: https://budget.app.example.com/api/integrations/v1/kids
    headers:
      Authorization: !secret home_finance_auth
      Accept: application/json
    scan_interval: 900
    sensor:
      - name: "Kids savings total"
        unique_id: home_finance_kids_total
        value_template: "{{ ((value_json.totals.totalCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
        state_class: total
```

And the household total of all pots from `GET /pots`:

```yaml
rest:
  - resource: https://budget.app.example.com/api/integrations/v1/pots
    headers:
      Authorization: !secret home_finance_auth
      Accept: application/json
    scan_interval: 900
    sensor:
      - name: "Savings pots total"
        unique_id: home_finance_pots_total
        value_template: "{{ ((value_json.totals.balanceCents | float(0)) / 100) | round(2) }}"
        unit_of_measurement: EUR
        device_class: monetary
        state_class: total
```

`rest:` is a list; put these entries in the same list as the per-kid and per-pot resources rather
than declaring `rest:` twice (YAML would keep only one of two duplicate keys).

### Choosing scan_interval

`900` (15 minutes) is a good default. The data only changes when someone adds a ledger entry or
a reported balance, which happens at most a few times a month, so polling faster just adds load
without making anything fresher. Raising it to `3600` is fine too. With one resource per kid
or pot the number of requests is simply the number of resources per interval.

### Lovelace

An entities card per kid:

```yaml
type: entities
title: Sam
entities:
  - entity: sensor.sam_savings_total
    name: Total
  - entity: sensor.sam_own_money
    name: Own money
  - entity: sensor.sam_saved_by_us
    name: Saved by us
  - entity: sensor.sam_deposited_this_month
    name: Deposited this month
  - entity: sensor.sam_deposited_this_year
    name: Deposited this year
  - entity: sensor.sam_bank_difference
    name: Difference with bank
  - entity: sensor.sam_last_entry
    name: Last entry
```

A gauge card for the progress towards a savings goal (percent, so 0 to 100; a pot over its goal
simply pins the needle at the end):

```yaml
type: gauge
entity: sensor.vacation_progress
name: Vacation
min: 0
max: 100
needle: true
severity:
  red: 0
  yellow: 40
  green: 80
```

An entities card for the budget totals:

```yaml
type: entities
title: Budget
entities:
  - entity: sensor.budget_month_income
    name: Income this month
  - entity: sensor.budget_month_expenses
    name: Expenses this month
  - entity: sensor.budget_month_surplus
    name: Surplus this month
  - entity: sensor.budget_month_status
    name: Month status
  - entity: sensor.budget_year_surplus
    name: Surplus this year
  - entity: sensor.budget_year_average_monthly_surplus
    name: Average monthly surplus
```

Or a grid of tiles for a dashboard overview:

```yaml
type: grid
columns: 3
square: false
cards:
  - type: tile
    entity: sensor.kids_savings_total
    name: All kids
  - type: tile
    entity: sensor.sam_savings_total
    name: Sam
    icon: mdi:piggy-bank
  - type: tile
    entity: sensor.noor_savings_total
    name: Noor
    icon: mdi:piggy-bank
  - type: tile
    entity: sensor.savings_pots_total
    name: All pots
  - type: tile
    entity: sensor.vacation_balance
    name: Vacation
    icon: mdi:beach
```

### Automations

Notify when the ledger and the bank disagree. Newer Home Assistant (2024.10+) syntax; on older
versions use `trigger:` with `platform:` and `condition:`/`action:` keys as before:

```yaml
automation:
  - alias: "Sam savings differ from bank"
    triggers:
      - trigger: template
        value_template: "{{ states('sensor.sam_bank_difference') | float(0) != 0 }}"
    actions:
      - action: notify.notify
        data:
          title: "Savings mismatch"
          message: >-
            Sam's ledger is {{ states('sensor.sam_bank_difference') }} EUR off the
            bank balance (reported {{ state_attr('sensor.sam_bank_difference', 'reportedBalanceDate') }}).
```

Replace `notify.notify` with your own notifier, for example `notify.mobile_app_your_phone`.
An `unavailable` sensor evaluates as `0` here and never triggers.

Notify when the amount deposited this month goes up. The month rollover (back to 0) is a
decrease, so it does not fire, and the guard skips startup transitions from
`unknown`/`unavailable`:

```yaml
automation:
  - alias: "Sam received a deposit"
    triggers:
      - trigger: state
        entity_id: sensor.sam_deposited_this_month
    conditions:
      - condition: template
        value_template: >-
          {{ trigger.from_state is not none
             and trigger.from_state.state not in ['unknown', 'unavailable']
             and trigger.to_state.state not in ['unknown', 'unavailable']
             and trigger.to_state.state | float(0) > trigger.from_state.state | float(0) }}
    actions:
      - action: notify.notify
        data:
          title: "New deposit"
          message: >-
            Sam: {{ (trigger.to_state.state | float(0) - trigger.from_state.state | float(0)) | round(2) }} EUR deposited.
            Total is now {{ states('sensor.sam_savings_total') }} EUR.
```

Notify when a savings goal is reached, using a template trigger on the binary sensor defined
above (it turns `on` when `target.reached` becomes true):

```yaml
automation:
  - alias: "Vacation goal reached"
    triggers:
      - trigger: template
        value_template: "{{ is_state('binary_sensor.vacation_goal_reached', 'on') }}"
    actions:
      - action: notify.notify
        data:
          title: "Savings goal reached"
          message: >-
            The Vacation pot reached its goal. Balance is now {{ states('sensor.vacation_balance') }} EUR.
```

A template trigger fires on the change from false to true, so it does not repeat while the pot
stays above its goal, and `unavailable` never counts as `on`.

Notify when the current month's surplus drops below zero (the month is overspent). It fires on
the change from false to true, and `unavailable` evaluates as `0`, so it never triggers:

```yaml
automation:
  - alias: "Budget month overspent"
    triggers:
      - trigger: template
        value_template: "{{ states('sensor.budget_month_surplus') | float(0) < 0 }}"
    actions:
      - action: notify.notify
        data:
          title: "Budget overspent"
          message: >-
            This month's surplus is {{ states('sensor.budget_month_surplus') }} EUR
            (income {{ states('sensor.budget_month_income') }} EUR, expenses
            {{ states('sensor.budget_month_expenses') }} EUR).
```

All `automation:` blocks can live in one list in `automations.yaml`. A 15-minute poll means
notifications arrive up to 15 minutes after the entry is made.

## 7. Errors and troubleshooting

Errors use the usual body `{"error": {"code": "...", "message": "..."}}`.

| Status | Code | Cause | What to do |
|---|---|---|---|
| 401 | `unauthorized` | Header missing, not `Bearer <token>`, token unknown, **revoked** or **expired**. All look identical on purpose. The response carries `WWW-Authenticate: Bearer`. | Check the header and the secret; look at the token's status and expiry in Settings > Integrations; rotate if needed. |
| 403 | `insufficient_scope` | The token is valid but lacks the endpoint's scope (`kids:read` for `/kids...`, `pots:read` for `/pots...`, `periods:read` for `/periods...`, `years:read` for `/years...`). | Create a new token with the scope (scopes cannot be added to an existing token). |
| 404 | `not_found` | Unknown path, or `{slug}` unknown, malformed or belonging to an **archived** kid or pot; often a renamed one. For totals: a malformed or out-of-range `{yyyy-mm}` / `{yyyy}`, a missing period or year, or `/periods/current` when no periods exist. | List slugs with `GET /kids` or `GET /pots`; update the URL. For totals, check the path format and that the period exists in the app. |
| 405 | `method_not_allowed` | Not a GET. | The API is read-only. |
| 500 | `db_error` | Server-side failure such as an unreachable database. Not a token problem, and deliberately not a 401 so an integration does not think its token was revoked. | Retry later; check the app's logs. |

Every response, including errors, has `Cache-Control: no-store`.

### Testing a token with curl

```bash
# 1. Is the token valid? Expect 200 and your token name.
curl -si -H "Authorization: Bearer $TOKEN" "$BASE/info"

# 2. Does it have the scope? Expect 200 (403 means no kids:read / pots:read /
#    periods:read / years:read).
curl -si -H "Authorization: Bearer $TOKEN" "$BASE/kids"
curl -si -H "Authorization: Bearer $TOKEN" "$BASE/pots"
curl -si -H "Authorization: Bearer $TOKEN" "$BASE/periods/current"
curl -si -H "Authorization: Bearer $TOKEN" "$BASE/years/current"

# 3. Which slugs exist?
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/kids" | jq -r '.kids[].slug'
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/pots" | jq -r '.pots[].slug'
```

A `401` with `WWW-Authenticate: Bearer` on step 1 means the token itself is wrong, revoked or
expired. A timeout rather than any HTTP status points at networking (see
[section 3](#3-reaching-the-api)).

In Home Assistant, a failing resource is logged by the `rest` integration, and its sensors show
`unavailable`. Developer Tools > Template with `{{ states('sensor.sam_savings_total') }}` is a
quick way to see the value, and a template-sensor value that does not appear usually means the
resource returned an error body instead of the expected JSON.

## 8. Security notes

- **Hash-only storage.** The server keeps only a SHA-256 hash and a short display prefix of each
  token. The raw value is shown once at creation.
- **Treat the token as a password.** Keep it in `secrets.yaml` or a secret manager. Never commit
  it, paste it in an issue, or put it in a URL (it is only accepted in the `Authorization`
  header).
- **Least privilege.** Grant only the scopes an integration needs, and use one token per
  integration so each can be revoked on its own.
- **Always expiring.** Tokens live 1 hour to 2 years (default 1 year). Rotate before expiry.
- **Revocation is immediate.** Revoke from Settings > Integrations; the next request fails with 401.
- **Audit log.** Creating and revoking a token is recorded in the audit log. Creating one also
  needs a fresh step-up reauthentication.
- **Read-only.** A leaked token can read kids' balances and names (`kids:read`) pot balances, goals and names (`pots:read`), or monthly and yearly income, expense and surplus totals (`periods:read`, `years:read`); it cannot change
  anything, cannot reach the session API and cannot read ledger descriptions.
- **Transport.** Use HTTPS for anything outside the cluster.

## 9. Roadmap

Done: phase 1 and 2 (kids, `kids:read`, and savings pots, `pots:read`) and phases 3 and 4 (monthly
totals, `periods:read`, and yearly totals, `years:read`).

Planned, not yet available:

- A Home Assistant custom integration (HACS) with a config flow: enter the base URL and token,
  validate it against `GET /info`, then create the kid, pot and budget sensors automatically from
  `GET /kids`, `GET /pots`, `GET /periods/current` and `GET /years/current`.
  Until then, the `rest:` cookbook above is the supported way.

All of this arrives additively within `/v1`, per the versioning promise.
