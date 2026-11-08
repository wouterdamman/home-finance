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

Example data in this guide uses invented kids ("Sam", "Noor", "Mila"). Amounts are always
**integer cents**, dates are `YYYY-MM-DD`, timestamps are RFC 3339 in UTC.

## 1. Overview

The Integration API lives under `/api/integrations/v1`. It is:

- **Read-only.** There are no write endpoints.
- **Token-authenticated.** Every request carries `Authorization: Bearer hf_...`. It never accepts
  a session cookie and needs no CSRF header.
- **Scoped.** Each token has one or more scopes. Today there is one: `kids:read`, which grants
  access to all kids' savings data (there is no per-kid restriction).
- **Privacy-minimal.** Responses contain no internal ids and no ledger descriptions (those are
  free text). Kid names are included.
- **Uncached.** Every response carries `Cache-Control: no-store`.

**Versioning promise.** Within `/v1` only *additive* changes happen: new endpoints, new fields in
existing objects, new scopes. Existing fields keep their name, type, unit and meaning. Clients
must ignore unknown fields. A breaking change will be published as `/v2`, and `/v1` keeps working
alongside it for a transition period.

## 2. Token setup

1. Sign in to the app as an **admin** and open **Settings > Integrations**.
2. Give the token a recognisable **name** (for example `Home Assistant`). The name is shown in the
   token list and in the audit log.
3. Tick the **scope(s)**. For everything in this guide that is `kids:read`.
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
/api/integrations/v1                      (Bearer token; scope kids:read on everything below)
├── GET /info                             token name + scopes (any valid token)
└── /kids
    ├── GET /kids                         all active kids (summary) + household totals
    ├── GET /kids/balances                legacy summary without slugs (kept unchanged)
    └── /kids/{slug}
        ├── GET /kids/{slug}              one kid, full detail (both sub-pots + combined)
        ├── GET /kids/{slug}/ours         sub-pot "door ons gespaard"
        └── GET /kids/{slug}/theirs       sub-pot "eigen geld"
```

`{slug}` is the kid's name slug, see [Slugs](#slugs). An unknown or archived slug returns
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
  "scopes": ["kids:read"]
}
```

| Field | Type | Meaning | Example |
|---|---|---|---|
| `apiVersion` | integer | Major version of this API. | `1` |
| `tokenName` | string | The name given to the token in Settings. | `"Home Assistant"` |
| `scopes` | string[] | Scopes the token holds. | `["kids:read"]` |

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

### Slugs

The `{slug}` in a kid's URL is derived from the kid's name:

- lowercased, diacritics folded (`Zoë` becomes `zoe`, `ß` becomes `ss`, `ø` becomes `o`);
- every run of characters outside `a-z0-9` becomes a single `-`, leading and trailing dashes are
  trimmed, at most 64 characters (`Anna-Lisa` and `Anna Lisa` both give `anna-lisa`);
- **collisions:** names are not unique. If two or more active kids produce the same slug, every
  one of them gets `<slug>-<id>` (for example `sam-12` and `sam-31`). The suffix is the kid's
  internal id, so it is stable;
- the slug `balances` is reserved (it is a static route), so a kid named "Balances" gets
  `balances-<id>`;
- a name with no letters or digits at all gets `kid-<id>`.

A slug is only valid if it matches `^[a-z0-9-]{1,64}$`; anything else is a 404.

**A rename changes the slug.** Renaming "Sam" to "Samuel" turns `/kids/sam` into a 404 and
`/kids/samuel` starts working. Adding a second kid with the same name also changes both slugs to
the suffixed form. **Archiving a kid removes the slug (404).** Integrations should therefore
discover slugs from `GET /kids` rather than hard-coding them if renames are likely, and treat a
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

`rest:` is a list; put this entry in the same list as the per-kid resources rather than
declaring `rest:` twice (YAML would keep only one of two duplicate keys).

### Choosing scan_interval

`900` (15 minutes) is a good default. The data only changes when someone adds a ledger entry or
a reported balance, which happens at most a few times a month, so polling faster just adds load
without making anything fresher. Each authenticated request also touches the token's "last used"
timestamp, which the server throttles to once a minute at most, so there is no benefit in
polling more often than that. Raising it to `3600` is fine too. With one resource per kid the
number of requests is simply the number of kids per interval.

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

Both `automation:` blocks can live in one list in `automations.yaml`. A 15-minute poll means
notifications arrive up to 15 minutes after the entry is made.

## 7. Errors and troubleshooting

Errors use the usual body `{"error": {"code": "...", "message": "..."}}`.

| Status | Code | Cause | What to do |
|---|---|---|---|
| 401 | `unauthorized` | Header missing, not `Bearer <token>`, token unknown, **revoked** or **expired**. All look identical on purpose. The response carries `WWW-Authenticate: Bearer`. | Check the header and the secret; look at the token's status and expiry in Settings > Integrations; rotate if needed. |
| 403 | `insufficient_scope` | The token is valid but lacks the endpoint's scope (`kids:read`). | Create a token with the scope. |
| 404 | `not_found` | Unknown path, or `{slug}` unknown, malformed or belonging to an **archived** kid; often a renamed kid. | List slugs with `GET /kids`; update the URL. |
| 405 | `method_not_allowed` | Not a GET. | The API is read-only. |
| 500 | `db_error` | Server-side failure such as an unreachable database. Not a token problem, and deliberately not a 401 so an integration does not think its token was revoked. | Retry later; check the app's logs. |

Every response, including errors, has `Cache-Control: no-store`.

### Testing a token with curl

```bash
# 1. Is the token valid? Expect 200 and your token name.
curl -si -H "Authorization: Bearer $TOKEN" "$BASE/info"

# 2. Does it have the scope? Expect 200 (403 means no kids:read).
curl -si -H "Authorization: Bearer $TOKEN" "$BASE/kids"

# 3. Which slugs exist?
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/kids" | jq -r '.kids[].slug'
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
- **Read-only.** A leaked `kids:read` token can read kids' balances and names; it cannot change
  anything, cannot reach the session API and cannot read ledger descriptions.
- **Transport.** Use HTTPS for anything outside the cluster.

## 9. Roadmap

Planned, not yet available (they are deliberately not accepted as scopes until their endpoints
exist):

- Scopes `pots:read`, `periods:read` and `years:read`, for savings pots, monthly overviews and
  yearly summaries, under the same `/api/integrations/v1` prefix.
- A Home Assistant custom integration (HACS) with a config flow: enter the base URL and token,
  validate it against `GET /info`, then create the kid sensors automatically from `GET /kids`.
  Until then, the `rest:` cookbook above is the supported way.

All of this arrives additively within `/v1`, per the versioning promise.
