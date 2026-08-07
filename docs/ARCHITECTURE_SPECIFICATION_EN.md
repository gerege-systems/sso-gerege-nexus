# Architecture Specification

System architecture, layering and technical decisions behind the
**Gerege Nexus**.

<p>
  <a href="ARCHITECTURE_SPECIFICATION.md"><img src="assets/icons/flag-mn.png" width="18" height="18" alt=""> Монгол</a>
  &nbsp;·&nbsp;
  <img src="assets/icons/flag-en.png" width="18" height="18" alt=""> <b>English</b>
</p>

[Back to the documentation hub](README.md)

---

## 1. System overview

**Gerege Nexus** is a high-performance **modular monolith platform** that
connects services, operations, systems, and data across public and private
organizations, wired directly into Mongolia's national digital infrastructure.

### 1.1 High-performance modular monolith

- **Zero-latency execution** — business modules (`contacts`, `products`,
  `inventory`, `billing`, `documents`, `developer_portal`) implement the Go
  `Module` contract and compile into a single binary.
- **Tenant app store** — whether a module is active for a tenant is decided
  dynamically from PostgreSQL (`app_installations`).
- **DAG dependency resolution** — a directed acyclic graph plus semver
  constraints resolve module dependencies without cycles.
- **Catalog sync** — `catalog/apps.json` is the single source of truth and the
  `apps` table is reconciled from it on every boot.

### 1.2 Cloud-native resilience (inspired by go-zero)

- **Adaptive circuit breaker** (`resilience/breaker.go`) — Google SRE sliding
  window error-rate rejection.
- **Adaptive load shedding** (`resilience/loadshedder.go`) — returns
  `503 Service Unavailable` once in-flight concurrency is exceeded.
- **Singleflight coalescing** (`resilience/singleflight.go`) — collapses
  duplicate queries and absorbs cache stampedes.
- **Exponential backoff retry** (`resilience/retry.go`) — retries transient
  failures.

### 1.3 State data exchange and identity

- **XYP state exchange** — citizen civil registration (`WS100101`) and legal
  entity data (`WS100201`).
- **DAN and E-ID** ([`eidmongolia.mn`](https://eidmongolia.mn),
  [`developer.gerege.mn`](https://developer.gerege.mn)) — PKI digital signature,
  mobile OTP, bank SSO and biometric face verification.
- **OAuth2 / OIDC provider** (`/.well-known/openid-configuration`) — the
  platform's own authorisation server.

> Mock mode is a development convenience only; it is disabled automatically when
> `ENVIRONMENT=production`.

---

## 2. Architecture diagram

```
+-----------------------------------------------------------------------------------+
|                              Gerege Nexus                             |
+-----------------------------------------------------------------------------------+
                                          |
                +-------------------------+-------------------------+
                |                                                   |
      +-------------------+                               +-------------------+
      | Next.js 15 Client |                               |  Go 1.25 Backend  |
      |   (App Router)    |                               |   (Chi Router)    |
      +-------------------+                               +-------------------+
                |                                                   |
        +-------+-------+                                   +-------+-------+
        |               |                                   |               |
+---------------+ +---------------+                 +---------------+ +---------------+
| AI Copilot UI | | E-ID / DAN    |                 | Cloud-Native  | | State Exchange|
|  Drawer Panel | | SSO Provider  |                 | Resilience    | | (xyp.gerege)  |
+---------------+ +---------------+                 +---------------+ +---------------+
                                                            |
                                                    +---------------+
                                                    | Shared-Schema |
                                                    |  PostgreSQL   |
                                                    +---------------+
```

---

## 3. Request pipeline

1. **Shared middleware** — logging, panic recovery, load shedding, Prometheus
   metrics, security headers, CORS.
2. **Authentication** — the session token is read from the cookie or the
   `Authorization: Bearer` header and resolved against the `sessions` table.
   Only the SHA-256 digest of the token is stored.
3. **Tenant context** — `tenant_id` is placed in the Go context and scopes every
   query.
4. **App gate** — each module route checks `app_installations`; an uninstalled
   or disabled app returns `403 Forbidden`.
5. **Module handler** — business logic and database transactions.

---

## 4. Core data model

| Table | Purpose |
| --- | --- |
| `tenants`, `users`, `memberships` | Multi-tenancy and user membership |
| `roles`, `permissions`, `role_permissions`, `membership_roles` | RBAC model |
| `sessions` | Server-side session tokens (SHA-256 digests) |
| `apps`, `app_versions`, `app_installations`, `installation_events` | App store and installation history |
| `contacts`, `products`, `warehouses`, `stock_levels`, `stock_movements` | Core business data |
| `billing_invoices`, `document_records` | Invoices and digital documents |
| `oauth2_clients` | OAuth2 client applications |

All schema changes go through goose migrations in `backend/db/migrations/`.
Runtime DDL is not allowed.

---

## 5. Architectural decisions

| Decision | Rationale |
| --- | --- |
| Modular monolith over microservices | In-process calls avoid network latency; module boundaries are enforced by Go interfaces |
| No ORM (`pgx` plus hand-written SQL) | Keeps queries explicit and tunable, avoids hidden N+1 |
| Shared-schema multi-tenancy | Isolation via `tenant_id` without duplicating schemas |
| Catalog file as source of truth | Adding an app needs no manual SQL; the `apps` table syncs automatically |
| Opaque session tokens | Avoids the revocation problem of stateless JWTs |

---

## 6. Maintainers

- **Gerege Systems Development Team** ([@gerege-systems](https://github.com/gerege-systems))
- **Gemini AI**, **Claude AI**
