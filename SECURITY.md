# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest (main) | ✅ |
| Older releases | ❌ |

Only the latest release receives security fixes.

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

This repository is private, so GitHub's Security Advisories form is not reachable from the
outside. Report privately by email to `<SECURITY_CONTACT_EMAIL>` instead.

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (optional)

This is a single-maintainer hobby project, not a staffed product — there is no response SLA.
Reports are read and acknowledged as soon as the maintainer picks them up, and a confirmed issue
is fixed in the next release; severity decides how far that release gets pulled forward.

## Scope

This is a self-hosted personal finance application. Key security considerations:

- **Authentication**: OIDC/BFF — session tokens never reach the browser; HttpOnly cookies only
- **CSRF**: SameSite=Lax + required `X-Requested-With` header on all mutations
- **Authorization**: `ALLOWED_EMAILS` env var restricts who can sign in at all; Authentik handles IdP-level gating in prod. Within the app, role-based access (`admin`/`user`, bootstrapped via `INITIAL_ADMIN_EMAILS`) gates structural/destructive actions — a `user` account can enter transactions but can't manage categories/pots/years/users, lock a year, or close/reopen/delete a month
- **Destructive actions**: period deletion, year lock/unlock, and destructive Excel import (wipe/reset) require a fresh (≤5 minute) Authentik re-authentication on top of role checks — a popup redirects through Authentik with `prompt=login`, and the callback verifies the returned identity matches the currently logged-in session before granting the action; no separate password is stored or handled by this app
- **Audit log**: close, reopen, delete, pot entry, import, and role-change actions are logged with user and timestamp; viewable in Settings (admin-only) and optionally exported periodically to S3-compatible storage
- **Avatars**: uploaded photos live as `bytea` rows in Postgres (`users.avatar_data`, migration 0013), not in object storage, and are streamed by a session-auth-gated proxy endpoint (`GET /api/users/:id/avatar`) — never from a public URL. The stored content type is derived by decoding the bytes, not from the client's `Content-Type` header, so only a genuine PNG/JPEG is ever persisted and served back
- **Object storage**: used by exactly one feature, the optional audit-log exporter; S3 credentials are only ever read from the server's own environment, never exposed to the frontend
- **Data**: all amounts stored as integer cents — no floating point

## Out of Scope

- Vulnerabilities requiring physical access to the server
- Denial-of-service attacks (single-household app, no rate-limiting SLA)
- Issues in dependencies already tracked by the automated dependency updates below

## Dependency Updates

Renovate opens dependency-update PRs for Go modules and npm packages. Security-relevant ones are merged promptly.
