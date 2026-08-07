# Security Policy

<p>
  <a href="../SECURITY.md"><img src="assets/icons/flag-mn.png" width="18" height="18" alt=""> Монгол</a>
  &nbsp;·&nbsp;
  <img src="assets/icons/flag-en.png" width="18" height="18" alt=""> <b>English</b>
</p>

---

## Supported versions

| Version | Security updates |
| --- | --- |
| 0.1.x | Supported |
| < 0.1.0 | Not supported |

---

## Reporting a vulnerability

We take the security of `open-gerege-nexus` seriously. If you believe you have
found a vulnerability, please disclose it responsibly.

### How to report

**Please do not report security vulnerabilities through public GitHub issues.**

Contact the security team directly:

- **Email**: `security@gerege.mn`
- **Maintainer**: Gerege Systems Development Team

### What to include

1. The class of issue (SQL injection, XSS, broken authentication, rate limiter
   bypass, and so on).
2. Full reproduction steps, including sample HTTP requests or code snippets.
3. The potential impact and blast radius.
4. Any suggested remediation.

### Response and disclosure process

1. **Acknowledgement** — we confirm receipt within 24–48 hours.
2. **Investigation** — the engineering team reproduces and verifies the issue.
3. **Patch** — once confirmed, a fix is implemented, tested and shipped as a
   security release.
4. **Disclosure** — a public advisory is published alongside the release notes.

---

## Controls implemented in the platform

| Control | Description |
| --- | --- |
| Tenant isolation | Every query is scoped by `tenant_id`, preventing cross-tenant leaks |
| App gating | Uninstalled or disabled modules reject access with `403 Forbidden` |
| Session tokens | 256-bit random values; only the SHA-256 digest is stored; revoked on logout |
| Passwords | Hashed with `bcrypt` |
| Rate limiting | Per-IP throttling on login (`golang.org/x/time/rate`) |
| Proxy headers | `X-Forwarded-For` is trusted only when `TRUST_PROXY_HEADERS=true` |
| Security headers | `X-Content-Type-Options`, `X-Frame-Options`, `Content-Security-Policy`, and `HSTS` in production |
| Path traversal guards | App slugs accept only `a-z`, `0-9`, `-` and `_` |
| OAuth2 client auth | Mandatory, using constant-time comparison |
| National integration mocks | Disabled automatically when `ENVIRONMENT=production` |
| Administrator rights | Installing, enabling or disabling apps and registering integrations require a tenant administrator |

---

## Continuous checks

CI runs the following on every push and pull request:

- `govulncheck` — matches dependencies and the standard library against the Go
  vulnerability database.
- `gosec` — static security analysis of the source.
- `golangci-lint` — correctness and static analysis.

A scheduled vulnerability scan also runs every Monday.
