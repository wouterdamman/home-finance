# home-finance Helm chart

Ships with two supported modes, selected purely by values — no template forking
needed.

## Default: Gateway API + External Secrets Operator + external Postgres

The chart's own `values.yaml` defaults assume a cluster with Gateway API,
[external-secrets](https://external-secrets.io/) and an existing Postgres instance
(shared or dedicated) reachable over the network:

- `httproutes` creates a Gateway API `HTTPRoute` (`ingress.enabled: false`)
- `externalSecrets.enabled: true` renders an `ExternalSecret` that builds the
  `secrets.name` Secret (`DATABASE_URL`, `OIDC_CLIENT_SECRET`) from a `SecretStore`
  (`externalSecrets.secretStoreRef`, default `onepassword-connect` — override for
  Vault, Doppler, AWS Secrets Manager, etc., anything ESO supports)
- `postgres.cnpg.enabled: false` — the chart does not provision its own database;
  point `externalSecrets.database.remoteKey` at a secret-store item exposing
  `host`/`port`/`database`/`username`/`password`

## Classic / standalone mode

For clusters without Gateway API, ESO, or an existing Postgres — everything the
chart needs is bundled and hand-configurable:

```yaml
ingress:
  enabled: true
  host: finance.example.com
httproutes: []

postgres:
  cnpg:
    enabled: true    # chart provisions its own CloudNativePG Cluster

externalSecrets:
  enabled: false

networkPolicy:
  cilium:
    enabled: false   # no Cilium on this cluster

oidc:
  existingSecret: my-oidc-secret   # Secret you create by hand, key: clientSecret
```

With `externalSecrets.enabled: false`, create the `secrets.name` Secret
(default `home-finance-secrets`) yourself:

```bash
kubectl create secret generic home-finance-secrets \
  --from-literal=DATABASE_URL='postgres://user:pass@host:5432/db?sslmode=disable' \
  --from-literal=OIDC_CLIENT_SECRET='...'
```

(`DATABASE_URL` here is only read if `postgres.cnpg.enabled` is also `false` — with
CNPG enabled the chart uses the Cluster's own auto-generated `<clusterName>-app`
Secret instead, and this key is ignored.)

## Cilium network policy

On by default (`networkPolicy.cilium.enabled: true`) — renders a `CiliumNetworkPolicy`
(DNS egress, app-port + Prometheus-scrape ingress), with `extraEgress`/`extraIngress`
escape hatches for anything else (e.g. an OIDC provider reachable only via a specific
CIDR/entity). Set to `false` on a cluster without Cilium (classic mode).

## Required environment / admin bootstrap

- `env.INITIAL_ADMIN_EMAILS` — comma-separated, granted `role=admin` on first login
- `env.ALLOWED_EMAILS` — comma-separated allowlist (unset = anyone the IdP authenticates)
