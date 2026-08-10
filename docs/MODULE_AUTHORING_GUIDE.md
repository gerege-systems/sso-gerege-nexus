# Module Authoring Guide

<p>
  <img src="assets/icons/flag-en.png" width="18" height="18" alt=""> <b>English</b>
</p>

[Back to the documentation hub](README.md)

Welcome to the **sso-gerege-nexus** Module Authoring Guide! This guide explains how external developers can write, register, and distribute custom business application modules for the platform.

---

## Module architecture overview

In `sso-gerege-nexus`, business modules are written in Go as compile-time packages under `backend/internal/apps/`. 

Every module MUST implement the `Module` interface defined in [`backend/internal/module.go`](../backend/internal/module.go):

```go
type Module interface {
    ID() string
    Name() string
    Version() string
    Dependencies() []Dependency
    Permissions() []PermissionDefinition
    Menus() []MenuDefinition
    RegisterRoutes(r chi.Router, gateMiddleware func(http.Handler) http.Handler)
}
```

---

## Step by step: creating a new module

### Step 1: Define Module Struct & Register in `appregistry`
Create a new directory `backend/internal/apps/invoices/invoices.go`:

```go
package invoices

import (
    "net/http"
    "github.com/go-chi/chi/v5"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/gerege-systems/sso-gerege-nexus/backend/internal"
    "github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/appregistry"
)

type Module struct {
    db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Module {
    m := &Module{db: db}
    appregistry.Register(m)
    return m
}

func (m *Module) ID() string      { return "io.example.invoices" }
func (m *Module) Name() string    { return "Invoicing & Billing" }
func (m *Module) Version() string { return "1.0.0" }

func (m *Module) Dependencies() []internal.Dependency {
    return []internal.Dependency{
        {ID: "io.example.contacts", VersionConstraint: "^1.0.0"},
        {ID: "io.example.products", VersionConstraint: "^1.0.0"},
    }
}
```

### Step 2: Define Permissions and Menus
```go
func (m *Module) Permissions() []internal.PermissionDefinition {
    return []internal.PermissionDefinition{
        {Code: "invoices.read", Name: "View Invoices"},
        {Code: "invoices.manage", Name: "Create & Edit Invoices"},
    }
}

func (m *Module) Menus() []internal.MenuDefinition {
    return []internal.MenuDefinition{
        {ID: "menu_invoices", Label: "Invoices", Path: "/invoices", Icon: "file-text", Order: 30},
    }
}
```

### Step 3: Register HTTP Routes with App Gate Middleware
```go
func (m *Module) RegisterRoutes(r chi.Router, gateMiddleware func(http.Handler) http.Handler) {
    r.Route("/api/v1/invoices", func(sub chi.Router) {
        sub.Use(gateMiddleware)
        sub.Get("/", m.handleListInvoices)
        sub.Post("/", m.handleCreateInvoice)
    })
}
```

### Using platform services

Anything more than one app needs lives in `internal/platform/` and reaches a
module through its constructor, not through a package-level singleton. The
server builds one instance in `NewServer` and passes it in, the way
`gov_services` receives the integration manager.

Email verification is one of these. Do not grow your own token table:

```go
type Module struct {
    db          *pgxpool.Pool
    emailVerify *emailverify.Service
}

func New(db *pgxpool.Pool, emailVerify *emailverify.Service) *Module { /* … */ }

// Somewhere in a handler, with the tenant taken from the request context:
_, err := m.emailVerify.Send(ctx, tenantID, emailverify.Request{
    Email:       invitee.Email,
    Source:      m.ID(),          // kept on the row, so the audit trail names you
    Purpose:     "invoice_portal_invite",
    RedirectURL: "https://portal.example/invited", // optional; HTTPS only
})
```

The mail is sent by the hosted verification service — this platform holds no
mailbox credential and composes no message. `Send` asks for the link, records
the request, and enforces the local sending limits. When the recipient follows
the link they come back to `/api/v1/verify/landed`, the verification is marked
confirmed exactly once, and they are forwarded to the `RedirectURL` you named.

Map the errors rather than reporting them all as server failures:

| Error | Meaning | Answer |
| --- | --- | --- |
| `*emailverify.InvalidError` | a bad address or destination | `400` |
| `*emailverify.RateLimitedError` | carries `RetryAfter` | `429` |
| `ErrNotConfigured`, `ErrOriginNotHTTPS`, `ErrUnauthorizedKey` | this deployment's configuration, not the request | `503` |
| `ErrUpstream` | the service could not send or could not be reached; retryable | `502` |

There is no webhook yet, so a verification is recorded only when the person
returns here. Treat `PENDING` as "we have not seen them come back", not as
"they ignored it".

### Step 4: Create App Manifest JSON
Add a manifest file in `catalog/manifests/invoices.json`:

```json
{
  "id": "io.example.invoices",
  "name": "Invoices",
  "version": "1.0.0",
  "platform": ">=0.1.0 <2.0.0",
  "dependencies": [
    { "id": "io.example.contacts", "version_constraint": "^1.0.0" },
    { "id": "io.example.products", "version_constraint": "^1.0.0" }
  ],
  "permissions": [
    {
      "code": "invoices.read",
      "name": "Read Invoices",
      "description": "Allows viewing invoices"
    },
    {
      "code": "invoices.manage",
      "name": "Manage Invoices",
      "description": "Allows issuing and editing invoices"
    }
  ],
  "menus": [
    {
      "id": "invoices",
      "label": "Invoices",
      "path": "/invoices",
      "icon": "receipt",
      "order": 70
    }
  ]
}
```

The field names must match `appcatalog.Manifest` exactly:

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | Must equal the `id` of the matching `catalog/apps.json` entry |
| `version` | string | Valid semver |
| `platform` | string | Semver constraint checked against the platform version (`1.0.0`) |
| `dependencies` | **array** of `{id, version_constraint}` | Not an object — `{}` fails to parse |
| `permissions` | **array of objects** `{code, name, description}` | Not an array of strings |
| `menus` | array of `{id, label, path, icon, order}` | `label`/`path`/`order`, not `name`/`action`/`sequence` |

The file name must be `catalog/manifests/<slug>.json`, where `<slug>` is the
slug used in `catalog/apps.json` (lowercase letters, digits, `-` and `_`).
A manifest that fails to load or whose `id` disagrees with the catalog entry is
a **startup error** — the server refuses to boot rather than silently
installing the app with an empty dependency, permission and menu set.

And update `catalog/apps.json` to index the new app in the App Store! The
`apps` database table is synchronised from that file on every boot, so no
manual SQL is required.

---

## Maintainers

- **Gerege Systems Development Team** ([@gerege-systems](https://github.com/gerege-systems))
- **Gemini AI**, **Claude AI**
