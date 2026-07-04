# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest (main) | ✅ |
| Older releases | ❌ |

Only the latest release receives security fixes.

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Report privately via [GitHub Security Advisories](https://github.com/TheIronRock95/home-finance/security/advisories/new).

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (optional)

You will receive a response within **72 hours**. If confirmed, a fix will be released as soon as possible (target: within 14 days for critical issues).

## Scope

This is a self-hosted personal finance application. Key security considerations:

- **Authentication**: OIDC/BFF — session tokens never reach the browser; HttpOnly cookies only
- **CSRF**: SameSite=Lax + required `X-Requested-With` header on all mutations
- **Authorization**: `ALLOWED_EMAILS` env var restricts access; Authentik handles IdP-level gating in prod
- **Destructive actions**: period deletion requires a separate `DELETE_PASSWORD` PIN
- **Audit log**: close, reopen, delete, and pot entry actions are logged with user and timestamp
- **Data**: all amounts stored as integer cents — no floating point

## Out of Scope

- Vulnerabilities requiring physical access to the server
- Denial-of-service attacks (single-household app, no rate-limiting SLA)
- Issues in dependencies already tracked by Dependabot

## Dependency Updates

Dependabot is enabled for Go modules and npm packages. Security PRs are merged promptly.
