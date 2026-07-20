# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest (main) | ✅ |
| Older releases | ❌ |

Only the latest release receives security fixes.

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Report privately via [GitHub Security Advisories](https://github.com/wouterdamman/home-finance/security/advisories/new).

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
- **Authorization**: `ALLOWED_EMAILS` env var restricts who can sign in at all; Authentik handles IdP-level gating in prod. Within the app, role-based access (`admin`/`user`, bootstrapped via `INITIAL_ADMIN_EMAILS`) gates structural/destructive actions — a `user` account can enter transactions but can't manage categories/pots/years/users, lock a year, or close/reopen/delete a month
- **Destructive actions**: period deletion, year lock/unlock, and destructive Excel import (wipe/reset) require a fresh (≤5 minute) Authentik re-authentication on top of role checks — a popup redirects through Authentik with `prompt=login`, and the callback verifies the returned identity matches the currently logged-in session before granting the action; no separate password is stored or handled by this app
- **Audit log**: close, reopen, delete, pot entry, import, and role-change actions are logged with user and timestamp; viewable in Settings (admin-only) and optionally exported periodically to S3-compatible storage
- **Object storage**: avatar images are never served from a public bucket URL — a session-auth-gated proxy endpoint streams them; S3 credentials are only ever read from the server's own environment, never exposed to the frontend
- **Data**: all amounts stored as integer cents — no floating point

## Out of Scope

- Vulnerabilities requiring physical access to the server
- Denial-of-service attacks (single-household app, no rate-limiting SLA)
- Issues in dependencies already tracked by Dependabot

## Dependency Updates

Dependabot is enabled for Go modules and npm packages. Security PRs are merged promptly.
