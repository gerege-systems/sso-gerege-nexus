# Changelog

All notable changes to **sso-gerege-nexus** (Gerege SSO) will be documented in
this file.

Entries below the rebrands keep the names that were true when they shipped — the
`open-gerege-mn-erp` repository, the ERP framing, and the `openerp.gerege.mn`
deployment, which moved on to `nexus.gerege.mn`; and the Gerege Nexus naming this
fork carried before it became Gerege SSO on `sso.gerege.mn`. A changelog edited to
match the present tense stops being a record.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added — Switching between the organisations you belong to

- **The membership table always allowed several; the runtime allowed one.**
  Which tenant a session acted for was decided at login by whichever membership
  was oldest (`internal/platform/auth_handlers.go`), and nothing could change it
  afterwards — signing out and back in landed the same person in the same
  tenant, deliberately, so somebody working for two organisations could reach
  only the first. `GET /api/v1/auth/tenants` lists the ones they may act for and
  `POST /api/v1/auth/switch-tenant` moves the session to one of them.
- **The token is rotated, not the row updated.** A session token is the
  authority to act inside one tenant, and the tenant is what changes; the new
  session inherits the old one's expiry, so moving between two organisations
  cannot be used to keep a session alive without signing in again. The
  membership check lives in the store, where no route can reach the insert
  without it, and a tenant the caller is not in answers 403 rather than 404 —
  whether it exists is not their business.
- **Both queries deliberately leave the tenant behind** (`tenant.Without`):
  `memberships` carries a `tenant_id` and is under the row-level policy, so a
  request bound to the current tenant would answer "which tenants may you act
  for" with the one the caller is already in.
- **The brand mark is the control**, and the account menu carries the same list
  — the mobile shell hides the header brand below 900px, and a phone is exactly
  where somebody moves between two organisations. Both render one component
  over one cached answer, so the two cannot drift and opening the second does
  not re-ask the server. The mark used to link to `/apps`, which the Platform
  tile beneath it in the rail still does. Choosing another organisation reloads
  the shell rather than patching state: the menus, the permissions and every
  list on screen belonged to the tenant just left.
- **The demo seed now has two organisations** (`cmd/api/seed.go`): Demo
  Corporation, with contacts, products, inventory and documents, and Demo Trade
  LLC, with contacts and billing. One tenant exercises nothing — the switcher,
  the row-level isolation and the per-tenant permission set all behave
  identically on a single-tenant deployment and identically wrongly if they are
  broken. The seeder runs after the platform server is built rather than before
  it, because an installation row references the `apps` table that the catalogue
  sync fills, and it installs through the installer so a demo tenant cannot
  claim an app whose Go module is not in the binary.

### Removed — The Swift macOS client (`desktop-mac/`)

- The bundle was a WKWebView pointed at the web client, plus a menu bar, Touch
  ID and a preferences window for the two server URLs. It was built by a shell
  script outside CI, so nothing compiled it on a pull request and nothing
  caught a Swift file that no longer built until somebody ran `make build-mac`
  by hand. `make build` ran it, which meant a build of this repository failed
  on any machine without Xcode.
- What it offered over the browser, the browser now offers: the web client is
  installable as a PWA and gets its own dock icon and window from
  `/manifest.webmanifest`, with no download and no store. The README section
  that documented `make build-mac` / `make run-mac` says that instead.
- The API keeps the path a native client would use — bearer tokens, no ambient
  cookie — so this is a client leaving, not the platform closing a door. Anyone
  wanting a native macOS app can build one against the same API in its own
  repository, where it can have a real build and a real signing identity.

### Added — Email verification as a platform capability

- **One flow instead of one per app**
  ([`internal/platform/emailverify`](backend/internal/platform/emailverify)):
  proving that somebody controls an address is not one module's business.
  Contacts wants it before it trusts an address, Documents before a signing link
  leaves for an outsider, Gov Services before it answers a citizen at one. Each
  is the same act, so it lives in the platform: an app module takes the service
  in its constructor the way `gov_services` takes the integration manager and
  calls `emailverify.Service.Send` with its own app id as the source.
- **The mail is sent by a hosted service, deliberately.** Delivering mail that
  arrives is not a matter of holding an SMTP password: it is SPF, DKIM, DMARC,
  reverse DNS and a sending reputation, maintained continuously. enigma.mn runs
  that, so this platform holds no mailbox credential, composes no message and
  owns no sender address. What stays here is what only this platform can know —
  which module asked, for whom, why, and whether the person came back.
- **The return is good exactly once** (migrations `00026`, `00027`): the request
  carries a single-use reference in the return address, stored as a SHA-256, and
  claimed by one conditional `UPDATE`. A browser reloading the landing page
  races itself, and a reference that travelled through a mailbox and a browser's
  history must not be replayable. A spent, expired or invented reference is
  `410` alike.
- **The platform is not an open redirector**: the onward destination is
  validated when the request is made — HTTPS only (HTTP tolerated for localhost
  outside production), no embedded credentials — not when somebody arrives, by
  which time the mail has gone.
- **Mail bombing has a cost**: a per-tenant hourly allowance in front of the
  shared key and a one-minute pause per recipient, answered `429` with a
  `Retry-After` somebody can obey — a limit we can avoid provoking upstream is
  one we do not have to explain. A request the service refuses withdraws its own
  row, so the Overview screen never shows a verification nobody was asked for.
- **Errors say who has to act**: a bad address is `400`, a missing key or an
  HTTP `PUBLIC_ORIGIN` or a rejected key is `503` (this deployment, not the
  request), and a failure at the service is `502` and retryable. An answer
  nobody documented is never read as success.
- **Settings → Email verification**: whether the service is reachable, what has
  been asked for and by whom, the verified rate, and a test send. No key
  management — keys belong to the sending service and are administered there,
  and this platform's copy is a server-side environment variable that never
  reaches a browser.
- **The page shown after a click exists in all seven platform languages.** It is
  read outside the product, by somebody who may never have seen it.
- **Known limitation, stated on the screen**: the service has no webhook yet, so
  a verification is recorded only when the person returns here. Somebody who
  confirms on another device and never comes back stays `PENDING`. That is the
  honest reading — this platform did not see it happen — and it is what the
  Overview screen says rather than something the code knows and the operator
  does not.

### Added — PDF E-Sign v2: eID Mongolia qualified remote signing

- **eID Mongolia signature client ([`internal/platform/eidsign`](backend/internal/platform/eidsign))**:
  a real relying-party client for the v3 signature API. The citizen's own device
  holds the private key and approves with PIN2, so nothing here ever touches a
  signing key: we hash the PDF, eID pushes that digest to the phone, and the
  signed document is assembled by eID's own doc-signer
  (`POST /v3/signature/stamp/{sessionId}`), which embeds the PKCS#7 together with
  OCSP and CRL data. Certificate level defaults to `QUALIFIED` — accepting
  `ADVANCED` would silently downgrade every document the ERP produces.
- **Asynchronous signing ceremony** (`/api/v1/esign/sign/init`, `/sign/{id}`,
  `/sign/{id}/download`, `/sign/{id}/cancel`): upload → verification code →
  PIN2 on the phone → long-poll → PAdES-signed PDF. Sessions carry the exact
  bytes whose digest was approved, so a document edited mid-ceremony cannot
  produce a signature that fails to verify.
- **Signing on behalf of an organisation**: representation rights are read live
  from the national registry rather than from a certificate, because a director
  who resigned yesterday still holds yesterday's certificate.
- **eID identity linkage** (`user_eid_identities`, migration `00010`): sign-in
  now records who a user is to eID. Without it every signature would make the
  citizen retype the registration number they had just authenticated with, and a
  typo would push the PIN2 prompt at somebody else's phone.
- **The five module screens are now real** — signature log (filters, pagination,
  CSV export), batch signing, stamp placement (with an A4 preview), HSM
  connection (read-only, with a connection probe) and signing policy — replacing
  the `/module/esign/*` coming-soon placeholders.
- **Signing policy**: a tenant can require qualified eID signatures and disable
  the HSM rail outright, including for callers hitting the API directly.

### Fixed — PDF E-Sign

- **The app's permissions were declared but never enforced.** `io.example.esign`
  is absent from the platform's blanket app gate
  ([`server.go`](backend/internal/platform/server.go)) and its handlers only
  checked the tenant, so anyone in a tenant could sign. Every route now asserts
  `esign.read`, `esign.sign` or `esign.manage` explicitly. Migration `00010`
  backfills the grants existing roles should already have had, so no current
  user loses access.
- **`esign.sign` was ungrantable by the installer**: the default-role rules key
  off a `.read`/`.manage` suffix, so only administrators would ever have
  received it.
- **The signature log recorded only successes**, so a refused or expired
  ceremony left no trace — exactly the event an auditor looks for. Failures,
  refusals, expiries and downloads are now recorded with an outcome.
- **Non-ASCII download filenames were mangled**: signed PDFs are now served with
  an RFC 5987 `filename*`, so a Cyrillic document keeps its name.
- **Truncated PDFs were accepted**: a valid `%PDF-` header on a truncated body
  was passed to the signing service, which returned something that would not
  open. Uploads are now checked for a trailer as well as a header.
- **Sidebar sub-menus rendered as identical grey boxes**: the icons named by the
  server's menu blueprints were never mapped in the frontend
  ([`Layout.tsx`](frontend/components/Layout.tsx)).

### Added

- **PDF E-Sign App Module ([`io.example.esign`](backend/internal/apps/esign))**:
  - PDF document upload with tenant-scoped storage (migration `00009`), page-count detection, and original/signed download endpoints (`/api/v1/esign`).
  - Digital signature (тоон гарын үсэг) certificate validation and PKCS#7 PDF signing via the Gerege eSign HSM platform client ([`internal/platform/gerege/esign.go`](backend/internal/platform/gerege/esign.go)) — the private signing key never leaves the HSM.
  - Visible signature stamp placement with last-page auto-targeting and signature audit log (`esign_signature_logs`).
  - Frontend signing flow (`/esign`): certificate check → canvas signature pad → HSM signing → signed PDF download.
  - Mock mode by default (`ESIGN_MOCK_MODE`); configure `ESIGN_LOGIN_URL`, `ESIGN_SIGN_URL`, and `ESIGN_TOKEN` for live signing.

### Fixed — CI/CD pipeline

- **Go toolchain mismatch broke every job**: `backend/go.mod` requires `go 1.25.7`
  while the workflows pinned `go-version: "1.24"` and both Dockerfiles used
  `golang:1.24-alpine` (which sets `GOTOOLCHAIN=local`, so the build hard-fails).
  All jobs now resolve the version from `backend/go.mod` and the image builder
  sets `GOTOOLCHAIN=auto`.
- **Security workflow could never pass**: `govulncheck ./...` and `gosec ./...`
  ran at the repository root, which contains no `go.mod`. They now run against
  `backend/`. The workflow also only triggered on PRs to `master` while the
  default branch is `main`.
- **GHCR push lacked `packages: write`**, so deployment failed with `denied:
  installation not allowed to Write the repository`.
- **Removed the `swag-drift` job**: the sources carry no swagger annotations and
  `backend/docs/` is untracked, so it could only fail or pass vacuously.
- `deploy.yml` no longer duplicates lint/test from `ci.yml`; it runs migrations
  before swapping the API over, uses `docker compose`, lower-cases the GHCR
  image path, and skips cleanly when deployment secrets are absent.
- Added a **frontend CI job** (`npm ci` + `tsc --noEmit` + `next build`) — the
  Next.js app was never built by CI.
- Added `.dockerignore`, a pinned `backend/.golangci.yml` (v2), deleted the
  duplicate `backend/Dockerfile`, untracked committed `.DS_Store` files, and
  `gofmt`-ed the 11 files that had drifted.
- `docker-compose.yml`: added a one-shot **migration service** (the API used to
  start against an empty schema), health checks, and build-time
  `NEXT_PUBLIC_API_URL`; `frontend/Dockerfile` now uses `npm ci` with the lock
  file. Database credentials are consistent across compose, `.env.example` and
  the Makefile.

### Fixed — Security

- **Session tokens were the user's UUID**, the same value returned by
  `/api/v1/auth/me`. Replaced with opaque 256-bit `crypto/rand` tokens stored as
  SHA-256 digests in a new `sessions` table, with expiry and real revocation on
  logout (logout previously only dropped the cookie).
- **Mock national-identity mode was on by default** (`os.Getenv(...) != "false"`),
  so in production `/auth/eid/login` and `/auth/dan/login` accepted any
  registration number and logged the caller in as the first user in the table
  with `is_admin: true`. Mock mode is now refused in production unless requested
  explicitly, and identities are matched against a real ERP user.
- **OAuth2 token endpoint accepted any known `client_id` with no secret**
  (`clientSecret != "" && ...` skipped the check entirely). Client
  authentication is now mandatory and constant-time, supports HTTP Basic,
  validates the grant type, and is also enforced on `/oauth2/introspect` and
  `/oauth2/revoke`.
- Removed the **hard-coded client secret** `secret_gerege_dev_2026`;
  `ListClients` no longer discloses client secrets.
- App install/enable/disable and integration registration now require a **tenant
  administrator** — any authenticated user could previously reconfigure the
  tenant.
- Login rate limiting no longer trusts `X-Forwarded-For` unless
  `TRUST_PROXY_HEADERS=true`.
- Halved-entropy `generateRandomString` (hex output truncated back to `n`) fixed.
- `/metrics` no longer labels unmatched routes with the raw request path —
  unbounded Prometheus cardinality driven by unauthenticated requests.

### Fixed — App store & modules

- **Billing, Documents and the Developer Portal could not be installed**: their
  modules were never registered in `appregistry`, their rows were missing from
  the `apps` table (foreign-key violation), and `developer_portal` was rejected
  by the slug validator, which forbade underscores.
- The `apps` table is now **synchronised from `catalog/apps.json` on boot**
  instead of a hand-maintained INSERT that listed three of six apps.
- **Three manifests were malformed** (`"dependencies": {}` instead of an array,
  permissions as plain strings, `depends`/`sequence`/`action` keys). They parsed
  into a silent stub with no dependencies, permissions or menus. Manifests are
  fixed, and a manifest that fails to load is now a startup error.
- Billing and Documents no longer create their tables at boot with the error
  discarded; the schema moved into migration `00004`. Both stop answering failed
  writes with **fabricated demo records** (`inv_demo_100`, `doc_demo_200`).
- The app store reported disabled apps as "not installed" because `installed`
  and `enabled` were both derived from the enabled-only query.
- Fixed a nil-interface panic in `InstallApp` when a module was missing from the
  registry, and a nil-pointer dereference in the E-ID/DAN login handlers that
  called `err.Error()` on a nil error.

### Fixed — Reliability

- `AsyncOTPMailer.Shutdown` closed the queue while workers and retries could
  still send on it (**panic: send on closed channel**) and dropped already-queued
  mail; it now drains, is idempotent, and refuses post-shutdown enqueues.
- AI Copilot intent classification was case-sensitive against a lowercase
  keyword table, so "Stock" never matched.
- Restored demo-data seeding (dropped from `cmd/api`), now idempotent and
  disabled in production unless `SEED_DEMO_DATA` is set.

---

## [0.1.0] - 2026-08-05

### Added
- **Modular Monolith Core Architecture**:
  - Pure Go compile-time `Module` interface and global module registry (`appregistry`).
  - Tenant-level app installation, enablement, and menu visibility engine (`appinstaller`).
  - **ORY Hydra-Grade OAuth2 & OpenID Connect (OIDC) SSO Provider ([`internal/platform/ssoprovider`](backend/internal/platform/ssoprovider))**:
  - OpenID Connect Discovery (`/.well-known/openid-configuration`), JWKS URI (`/.well-known/jwks.json`), and OAuth2 Authorization Server (`/oauth2/token`, `/oauth2/introspect`, `/oauth2/revoke`).
  - Supports `authorization_code`, `client_credentials`, and `refresh_token` grant flows.
- **Developer Portal App Module ([`io.example.developer_portal`](backend/internal/apps/developer_portal))**:
  - Developer portal interface (`/developer/apps`) to register third-party OAuth2 client applications, issue Client IDs and Client Secrets, and manage redirect URIs.
- **Automated Production Deployment & CI/CD Pipeline ([`openerp.gerege.mn`](.github/workflows/deploy.yml))**:
  - Continuous Integration & Automated Deployment pipeline building GHCR Docker images and deploying to `openerp.gerege.mn`.
  - Production Multi-Stage Dockerfile ([`deploy/Dockerfile`](deploy/Dockerfile)) and Nginx SSL Reverse Proxy config (then `deploy/nginx/openerp.gerege.mn.conf`, since renamed to [`deploy/nginx/nexus.gerege.mn.conf`](deploy/nginx/nexus.gerege.mn.conf)).
  - Recursive dependency resolution algorithm with cycle detection and semver validation.
- **Shared-Schema Multi-Tenancy**:
  - Context-scoped `tenant_id` isolation across all business entities and repositories.
  - Tenant app gating middleware returning `403 Forbidden` for disabled modules.
- **Business Modules (Vertical Slices)**:
  - **Contacts (`io.example.contacts`)**: Business contacts directory with full CRUD.
  - **Products (`io.example.products`)**: Product catalog management with unique tenant-scoped SKUs.
  - **Inventory (`io.example.inventory`)**: Warehouse management, live stock levels, append-only stock movement log, and transactional stock adjustments with negative stock protection.
- **Next.js App Router Admin Shell**:
  - Top navigation bar with tenant badge (`Demo Corporation`), user profile menu, and logout.
  - Dynamic sidebar navigation driven by `/api/v1/menus`.
  - App Store (`/apps`) with search, categories, dependency badges, and Install/Enable/Disable controls.
  - Installed Apps Settings (`/settings/apps`).
  - Dedicated business UIs for `/contacts`, `/products`, and `/inventory`.
- **High-Performance Resilience Engine (go-zero Inspired)**:
  - **Adaptive Circuit Breaker (`resilience/breaker.go`)**: Google SRE style sliding window adaptive circuit breaker.
  - **Adaptive Load Shedding (`resilience/loadshedder.go`)**: In-flight HTTP request concurrency shedder returning `503 Service Unavailable` under heavy traffic spikes.
  - **Singleflight Coalescing (`resilience/singleflight.go`)**: Duplicate query suppressor preventing thundering herd cache stampedes.
  - **Exponential Backoff Retry (`resilience/retry.go`)**: `DoWithRetry` execution helper for resilient DB/network operations.
- **Observability & Async Messaging**:
  - Prometheus metrics endpoint (`/metrics`) recording HTTP request rates and latency histograms (`github.com/prometheus/client_golang`).
  - OpenTelemetry tracing initialization (`SetupTracing`).
  - Async OTP Mailer queue with worker pool, retry logic, and graceful shutdown (`internal/platform/mailer`).
- **Public Billing & e-Barimt Module ([`io.example.billing`](backend/internal/apps/billing))**:
  - Public service fee invoices, 10% VAT calculation for Mongolia e-Barimt, and status tracking (`/billing`).
- **Gerege DAN SSO Gateway System ([`dan.gerege.mn`](backend/internal/platform/dan))**:
  - Official Gerege Systems DAN SSO Gateway integration service (`POST /api/v1/auth/dan/login`).
  - Citizen identity verification and session token validation against `https://dan.gerege.mn/api/v1`.
- **E-ID Digital Identity & DAN SSO Authentication ([`internal/platform/eid`](backend/internal/platform/eid))**:
  - Aligned 100% with official **[eidmongolia.mn](https://eidmongolia.mn)** & **[developer.gerege.mn](https://developer.gerege.mn)** OAuth2 and OpenID Connect (OIDC) specifications.
  - Supports 4 official Mongolian authentication channels: PKI Digital Signature (Тоон гарын үсэг), Mobile OTP, Bank SSO, and Biometric Face Verification.
- **External System Integrations & Webhook Engine ([`internal/platform/integration`](backend/internal/platform/integration))**:
  - Event Dispatcher & Connector Manager supporting HMAC-SHA256 signature signing, asynchronous webhooks, and third-party REST connectors.
  - Dedicated Integration Settings Manager UI (`/settings/integrations`) with real-time status & health tracking.
- **XYP State Data Exchange System ([`xyp.gerege.mn`](backend/internal/platform/gerege/xyp.go))**:
  - Official Mongolian State Data Exchange (ХУР Төрийн мэдээлэл солилцооны систем) integration service.
  - Citizen civil registration (`POST /api/v1/xyp/citizen`) & company legal entity verification (`POST /api/v1/xyp/company`).
  - Interactive "ХУР / XYP Auto-fill" button integration on Contacts page.
- **Database & Migrations**:
  - Goose SQL migrations (`00001_platform_core.sql`, `00002_app_store.sql`, `00003_business_apps.sql`).
  - Automated initial demo data seeder (`admin@example.com` / `Password123!`).

---

### Inspirations & Acknowledgements
- **[snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)** by [@snykk](https://github.com/snykk): Initial Go REST API structure.
- **[Odoo](https://github.com/odoo/odoo)**: Modular app ecosystem, App Store dependency resolver, and dynamic menu architecture.
- **[go-zero](https://github.com/zeromicro/go-zero)**: High-performance cloud-native resilience engine (Adaptive Circuit Breaker, Load Shedder, Singleflight, Exponential Retry).

### Authors & Contributors
- **Gerege Systems Development Team**
- **Gemini AI**
- **Claude AI**
