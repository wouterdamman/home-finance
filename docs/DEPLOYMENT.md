# Deployment reference

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | — | PostgreSQL connection string |
| `PORT` | `8080` | HTTP listen port |
| `ENV` | `production` | Deployment environment. Only `development` unlocks `DEV_FAKE_AUTH` — a missing or misspelled value fails secure |
| `AUTO_MIGRATE` | `false` | Run goose migrations on startup |
| `DEV_FAKE_AUTH` | `false` | Skip OIDC, inject dev user |
| `OIDC_ISSUER_URL` | — | OIDC provider discovery URL |
| `OIDC_CLIENT_ID` | — | OIDC client ID |
| `OIDC_CLIENT_SECRET` | — | OIDC client secret |
| `OIDC_REDIRECT_URL` | — | OAuth callback URL |
| `ALLOWED_EMAILS` | — | Comma-separated allowlist (empty = all) |
| `INITIAL_ADMIN_EMAILS` | — | Comma-separated emails granted `role=admin` on first login (upsert only — doesn't override a role changed later in Settings > Users) |
| `STATIC_DIR` | — | Path to frontend dist (empty = no SPA serving) |
| `SESSION_SECURE` | `true` | `Secure` flag on the session cookie — `false` only for plain-HTTP local dev |
| `S3_ENDPOINT` | — | S3-compatible object storage endpoint (audit-log export only). Empty disables it |
| `S3_BUCKET` | — | Bucket name |
| `S3_ACCESS_KEY_ID` | — | Access key |
| `S3_SECRET_ACCESS_KEY` | — | Secret key |
| `S3_REGION` | — | Region (optional, provider-dependent) |
| `S3_USE_SSL` | `true` | Use HTTPS to reach the S3 endpoint |
| `AUDIT_EXPORT_INTERVAL` | — | Go duration string (`"10s"`, `"1h"`, `"12h"`) for periodic audit-log export to S3 — the only feature that uses S3. Empty disables it; requires S3 to also be configured |

## Releasing

Push a `v*` tag → GitHub Actions builds a multi-arch image, pushes to `ghcr.io/wouterdamman/home-finance`, creates a GitHub release, and bumps the Helm chart version on `main`.

```bash
git tag v1.0.0
git push origin v1.0.0
```

Version bumps and changelog entries are otherwise handled by [Release Please](https://github.com/googleapis/release-please) off conventional commit messages — merging its release PR is what actually cuts the tag.

## Kubernetes deploy

```bash
# Install / upgrade
helm upgrade --install home-finance deploy/helm/home-finance \
  --set oidc.issuerURL=https://auth.example.com/application/o/home-finance/ \
  --set oidc.clientID=home-finance \
  --set ingress.host=finance.example.com \
  --set env.INITIAL_ADMIN_EMAILS=you@example.com \
  --set oidc.existingSecret=home-finance-oidc

# One-off Excel import
kubectl run importer --rm -it --restart=Never \
  --image=ghcr.io/wouterdamman/home-finance:latest \
  --env="DATABASE_URL=$(kubectl get secret home-finance-pg-app -o jsonpath='{.data.uri}' | base64 -d)" \
  -- /app/importer --xlsx /tmp/Finance.xlsx --year 2026 --wipe --close-through 6
```

See `deploy/helm/home-finance/values.yaml` for the full set of chart options — network policy (Cilium), Gateway API vs classic Ingress, CNPG-managed vs external Postgres, pod security context, and more, each documented inline.

### Object storage (audit-log export)

Optional and off by default — the audit-log exporter is the only feature that uses S3. Avatar
photos are stored as `bytea` rows in Postgres, so no object storage is involved there. Enable by
pointing `s3.existingSecret` at a Secret with `ENDPOINT` / `ACCESS_KEY_ID` /
`SECRET_ACCESS_KEY` keys (any S3-compatible provider):

```bash
kubectl create secret generic home-finance-s3 \
  --from-literal=ENDPOINT=s3.eu-central-1.example.com \
  --from-literal=ACCESS_KEY_ID=... \
  --from-literal=SECRET_ACCESS_KEY=...

helm upgrade --install home-finance deploy/helm/home-finance \
  --set s3.enabled=true \
  --set s3.existingSecret=home-finance-s3 \
  --set s3.bucket=home-finance \
  --set auditExport.interval=1h \
  # ... plus the oidc/ingress flags above
```

`auditExport.interval` is a Go duration string (`10s`, `5m`, `1h`, `12h`) and runs as a goroutine
inside the app pod, not a separate CronJob — that's why sub-minute intervals work. Leave it unset
and S3 is never touched at all.

## Database migrations

Migrations live in `backend/migrations/` and run automatically on startup when `AUTO_MIGRATE=true`. To run manually:

```bash
make migrate
```
