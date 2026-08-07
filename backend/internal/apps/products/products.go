package products

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/appregistry"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/tenant"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Product struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	SKU       string    `json:"sku"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
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

func (m *Module) ID() string      { return "io.example.products" }
func (m *Module) Name() string    { return "Products" }
func (m *Module) Version() string { return "1.0.0" }

func (m *Module) Dependencies() []internal.Dependency {
	return nil
}

func (m *Module) Permissions() []internal.PermissionDefinition {
	return []internal.PermissionDefinition{
		{Code: "products.read", Name: "Read Products", Description: "View product catalog"},
		{Code: "products.manage", Name: "Manage Products", Description: "Create and edit products"},
	}
}

func (m *Module) Menus() []internal.MenuDefinition {
	return []internal.MenuDefinition{
		{ID: "products", ParentID: "master_data", Label: "Products", Path: "/products", Icon: "package", Order: 20, Labels: map[string]string{"mn": "Бараа бүтээгдэхүүн"}},
	}
}

func (m *Module) RegisterRoutes(r chi.Router, tenantAuthMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/products", func(pr chi.Router) {
		pr.Use(tenantAuthMiddleware)
		pr.Get("/", m.listProductsHandler)
		pr.Post("/", m.createProductHandler)
		pr.Put("/{id}", m.updateProductHandler)
	})
}

func (m *Module) listProductsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	rows, err := m.db.Query(r.Context(),
		`SELECT id, tenant_id, sku, name, price, active, created_at, updated_at 
		 FROM products WHERE tenant_id = $1 ORDER BY name ASC`, tenantID)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	list := make([]Product, 0)
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.TenantID, &p.SKU, &p.Name, &p.Price, &p.Active, &p.CreatedAt, &p.UpdatedAt); err != nil {
			http.Error(w, `{"error":"scan error"}`, http.StatusInternalServerError)
			return
		}
		list = append(list, p)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (m *Module) createProductHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		SKU    string  `json:"sku"`
		Name   string  `json:"name"`
		Price  float64 `json:"price"`
		Active bool    `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SKU == "" || req.Name == "" {
		http.Error(w, `{"error":"invalid product payload, sku and name required"}`, http.StatusBadRequest)
		return
	}

	id := uuid.New().String()
	now := time.Now()

	_, err = m.db.Exec(r.Context(),
		`INSERT INTO products (id, tenant_id, sku, name, price, active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id, claims.TenantID, req.SKU, req.Name, req.Price, req.Active, now, now)
	if err != nil {
		http.Error(w, `{"error":"sku already exists or DB error"}`, http.StatusConflict)
		return
	}

	p := Product{
		ID:        id,
		TenantID:  claims.TenantID,
		SKU:       req.SKU,
		Name:      req.Name,
		Price:     req.Price,
		Active:    req.Active,
		CreatedAt: now,
		UpdatedAt: now,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(p)
}

func (m *Module) updateProductHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	var req struct {
		SKU    string  `json:"sku"`
		Name   string  `json:"name"`
		Price  float64 `json:"price"`
		Active bool    `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
		return
	}

	now := time.Now()
	res, err := m.db.Exec(r.Context(),
		`UPDATE products SET sku = $1, name = $2, price = $3, active = $4, updated_at = $5
		 WHERE id = $6 AND tenant_id = $7`,
		req.SKU, req.Name, req.Price, req.Active, now, id, claims.TenantID)
	if err != nil || res.RowsAffected() == 0 {
		http.Error(w, `{"error":"product not found or unauthorized"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}
