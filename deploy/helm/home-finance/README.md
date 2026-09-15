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
- `postgres.sslMode: require` — appended to the `DATABASE_URL` the `ExternalSecret`
  assembles. Default Postgres connections in the stack cross namespaces, so TLS is
  on by default; use `verify-full` if you also want the server certificate checked,
  or `disable` only for a Postgres with no TLS at all

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
  --from-literal=DATABASE_URL='postgres://user:pass@host:5432/db?sslmode=require' \
  --from-literal=OIDC_CLIENT_SECRET='...'
```

(In classic mode you write the whole URL yourself, so `postgres.sslMode` does not
apply — pick the `sslmode` here.)

(`DATABASE_URL` here is only read if `postgres.cnpg.enabled` is also `false` — with
CNPG enabled the chart uses the Cluster's own auto-generated `<clusterName>-app`
Secret instead, and this key is ignored.)

## Cilium network policy

On by default (`networkPolicy.cilium.enabled: true`) — renders two
`CiliumNetworkPolicy` objects, with `extraEgress`/`extraIngress` escape hatches for
anything else (e.g. an OIDC provider reachable only via a specific CIDR/entity).
Set to `false` on a cluster without Cilium (classic mode).

- `<release>` — the server: egress to kube-dns, Postgres and the OIDC issuer's
  FQDN; ingress on `:8080` from the Gateway data plane
  (`networkPolicy.cilium.gatewayIngress`) and from Prometheus
  (`networkPolicy.cilium.prometheusNamespace`).
- `<release>-migrate` — the migration Job, whose pod carries a different label and
  so matched no policy at all before: egress to kube-dns and Postgres only, no
  ingress.

`gatewayIngress` has the same shape as `dbEgress` and **must** be filled in when
Cilium policy is on — with `namespaceLabel` empty no `:8080` ingress rule is
rendered and only Prometheus can reach the app. Defaults point at the Cilium
Gateway named in `httproutes[0].gateway`; in classic-Ingress mode point it at the
nginx controller's namespace/pod labels instead.

## Pod hardening

`podSecurityContext`/`securityContext` (applied to the Deployment *and* the migrate
Job) meet the Pod Security `restricted` profile that `templates/namespace.yaml`
enforces: non-root UID 65532, `seccompProfile: RuntimeDefault`, no privilege
escalation, all capabilities dropped, read-only root filesystem, and no
ServiceAccount token mounted (`serviceAccount.*`, a dedicated zero-permission
account per workload).

Because the root filesystem is read-only, an `emptyDir` is mounted at `/tmp`
(`tmpVolume.sizeLimit`, default `256Mi`) — xlsx import and avatar upload both write
there via `os.CreateTemp`/`ParseMultipartForm`, and break without it.

## HTTP → HTTPS redirect

Each entry in `httproutes` can carry an optional `httpRedirect` block that renders a
second `HTTPRoute` on the Gateway's plaintext listener with a `RequestRedirect`
filter (`scheme: https`, 301). Off by default because the HTTP listener's
`sectionName` is cluster-specific. Classic-Ingress mode gets the same behaviour from
nginx's `force-ssl-redirect` annotation instead.

## Required environment / admin bootstrap

- `env.INITIAL_ADMIN_EMAILS` — comma-separated, granted `role=admin` on first login
- `env.ALLOWED_EMAILS` — comma-separated allowlist (unset = anyone the IdP authenticates)

## Avatar storage

Avatar photos are stored as bytea rows in Postgres — no PVC, no object storage,
no extra chart config. `replicas: 1` with `strategy.type: Recreate` is safe by
default because the app has no ReadWriteOnce volume to fight over during rollout.

Older installs used a filesystem backend on a Longhorn RWO PVC
(`avatarStorage.filesystem.*`), which caused Multi-Attach rollout races and has
been removed from this chart. If you're upgrading from that setup: run
`kubectl exec -it deploy/<release> -- /app/migrate-avatars` against the
currently-running pod (it still has both the PVC mount and `DATABASE_URL`) to
copy existing avatar files into Postgres *before* deploying a chart version
without the PVC template — otherwise those files become unreachable.
