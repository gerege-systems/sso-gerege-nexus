package contacts

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/appregistry"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/httpx"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/tenant"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Contact struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	Company   string    `json:"company"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Module struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Module {
	m := &Module{db: db}
	appregistry.Register(m)
	return m
}

func (m *Module) ID() string      { return "io.example.contacts" }
func (m *Module) Name() string    { return "Contacts" }
func (m *Module) Version() string { return "1.0.0" }

func (m *Module) Dependencies() []internal.Dependency {
	return nil
}

func (m *Module) Permissions() []internal.PermissionDefinition {
	return []internal.PermissionDefinition{
		{Code: "contacts.read", Name: "Read Contacts", Description: "View contacts list"},
		{Code: "contacts.manage", Name: "Manage Contacts", Description: "Create and edit contacts"},
	}
}

func (m *Module) Menus() []internal.MenuDefinition {
	return []internal.MenuDefinition{
		{ID: "contacts", ParentID: "master_data", Label: "Contacts", Path: "/contacts", Icon: "users", Order: 10, Labels: map[string]string{"mn": "Харилцагчид", "ar": "جهات الاتصال", "zh": "联系人", "fr": "Contacts", "ru": "Контакты", "es": "Contactos"}},
	}
}

func (m *Module) RegisterRoutes(r chi.Router, tenantAuthMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/contacts", func(cr chi.Router) {
		cr.Use(tenantAuthMiddleware)
		cr.Get("/", m.listContactsHandler)
		cr.Post("/", m.createContactHandler)
		cr.Put("/{id}", m.updateContactHandler)
	})
}

func (m *Module) listContactsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}

	rows, err := m.db.Query(r.Context(),
		`SELECT id, tenant_id, name, email, phone, company, active, created_at, updated_at 
		 FROM contacts WHERE tenant_id = $1 ORDER BY name ASC`, tenantID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	list := make([]Contact, 0)
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.ID, &c.TenantID, &c.Name, &c.Email, &c.Phone, &c.Company, &c.Active, &c.CreatedAt, &c.UpdatedAt); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "scan error")
			return
		}
		list = append(list, c)
	}
	// A stream that breaks partway through ends the loop the same way a
	// complete one does, so without this the caller receives a short list
	// under a 200 and has no way to tell it apart from the whole set.
	if err := rows.Err(); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "scan error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (m *Module) createContactHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Name    string `json:"name"`
		Email   string `json:"email"`
		Phone   string `json:"phone"`
		Company string `json:"company"`
		Active  bool   `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid contact payload, name is required")
		return
	}

	id := uuid.New().String()
	now := time.Now()

	_, err = m.db.Exec(r.Context(),
		`INSERT INTO contacts (id, tenant_id, name, email, phone, company, active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		id, claims.TenantID, req.Name, req.Email, req.Phone, req.Company, req.Active, now, now)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to create contact")
		return
	}

	contact := Contact{
		ID:        id,
		TenantID:  claims.TenantID,
		Name:      req.Name,
		Email:     req.Email,
		Phone:     req.Phone,
		Company:   req.Company,
		Active:    req.Active,
		CreatedAt: now,
		UpdatedAt: now,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(contact)
}

func (m *Module) updateContactHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	var req struct {
		Name    string `json:"name"`
		Email   string `json:"email"`
		Phone   string `json:"phone"`
		Company string `json:"company"`
		Active  bool   `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid payload")
		return
	}

	now := time.Now()
	res, err := m.db.Exec(r.Context(),
		`UPDATE contacts SET name = $1, email = $2, phone = $3, company = $4, active = $5, updated_at = $6
		 WHERE id = $7 AND tenant_id = $8`,
		req.Name, req.Email, req.Phone, req.Company, req.Active, now, id, claims.TenantID)
	if err != nil || res.RowsAffected() == 0 {
		httpx.Error(w, http.StatusNotFound, "contact not found or unauthorized")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}
