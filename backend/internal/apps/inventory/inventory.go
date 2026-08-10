package inventory

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/appregistry"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/httpx"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/tenant"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Warehouse struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	CreatedAt time.Time `json:"created_at"`
}

type StockLevel struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	WarehouseID string    `json:"warehouse_id"`
	ProductID   string    `json:"product_id"`
	Quantity    float64   `json:"quantity"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type StockMovement struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	WarehouseID    string    `json:"warehouse_id"`
	ProductID      string    `json:"product_id"`
	QuantityChange float64   `json:"quantity_change"`
	Reference      string    `json:"reference"`
	CreatedAt      time.Time `json:"created_at"`
}

type Module struct {
	db                 *pgxpool.Pool
	allowNegativeStock bool
}

func New(db *pgxpool.Pool, allowNegativeStock bool) *Module {
	m := &Module{db: db, allowNegativeStock: allowNegativeStock}
	appregistry.Register(m)
	return m
}

func (m *Module) ID() string      { return "io.example.inventory" }
func (m *Module) Name() string    { return "Inventory" }
func (m *Module) Version() string { return "1.0.0" }

func (m *Module) Dependencies() []internal.Dependency {
	return []internal.Dependency{
		{ID: "io.example.contacts", VersionConstraint: "^1.0.0"},
		{ID: "io.example.products", VersionConstraint: "^1.0.0"},
	}
}

func (m *Module) Permissions() []internal.PermissionDefinition {
	return []internal.PermissionDefinition{
		{Code: "inventory.read", Name: "Read Inventory", Description: "View stock levels and warehouses"},
		{Code: "inventory.manage", Name: "Manage Inventory", Description: "Create warehouses and stock adjustments"},
	}
}

func (m *Module) Menus() []internal.MenuDefinition {
	return []internal.MenuDefinition{
		{ID: "inventory", ParentID: "operations", Label: "Inventory", Path: "/inventory", Icon: "boxes", Order: 10, Labels: map[string]string{"mn": "Агуулах", "ar": "المخزون", "zh": "库存", "fr": "Inventaire", "ru": "Склад", "es": "Inventario"}},
	}
}

func (m *Module) RegisterRoutes(r chi.Router, tenantAuthMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/inventory", func(ir chi.Router) {
		ir.Use(tenantAuthMiddleware)
		ir.Get("/warehouses", m.listWarehousesHandler)
		ir.Post("/warehouses", m.createWarehouseHandler)
		ir.Get("/stock-levels", m.listStockLevelsHandler)
		ir.Get("/movements", m.listStockMovementsHandler)
		ir.Post("/adjustments", m.adjustStockHandler)
	})
}

func (m *Module) listWarehousesHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}

	rows, err := m.db.Query(r.Context(),
		`SELECT id, tenant_id, code, name, address, created_at FROM warehouses WHERE tenant_id = $1 ORDER BY name ASC`, tenantID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	list := make([]Warehouse, 0)
	for rows.Next() {
		var wh Warehouse
		if err := rows.Scan(&wh.ID, &wh.TenantID, &wh.Code, &wh.Name, &wh.Address, &wh.CreatedAt); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "scan error")
			return
		}
		list = append(list, wh)
	}
	// A broken stream ends the loop exactly like a complete one; without this
	// a truncated list goes out under a 200.
	if err := rows.Err(); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "scan error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (m *Module) createWarehouseHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Code    string `json:"code"`
		Name    string `json:"name"`
		Address string `json:"address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" || req.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid warehouse payload, code and name required")
		return
	}

	id := uuid.New().String()
	now := time.Now()

	_, err = m.db.Exec(r.Context(),
		`INSERT INTO warehouses (id, tenant_id, code, name, address, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		id, claims.TenantID, req.Code, req.Name, req.Address, now)
	if err != nil {
		httpx.Error(w, http.StatusConflict, "warehouse code already exists or DB error")
		return
	}

	wh := Warehouse{
		ID:        id,
		TenantID:  claims.TenantID,
		Code:      req.Code,
		Name:      req.Name,
		Address:   req.Address,
		CreatedAt: now,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(wh)
}

func (m *Module) listStockLevelsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}

	rows, err := m.db.Query(r.Context(),
		`SELECT id, tenant_id, warehouse_id, product_id, quantity, updated_at FROM stock_levels WHERE tenant_id = $1`, tenantID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	list := make([]StockLevel, 0)
	for rows.Next() {
		var sl StockLevel
		if err := rows.Scan(&sl.ID, &sl.TenantID, &sl.WarehouseID, &sl.ProductID, &sl.Quantity, &sl.UpdatedAt); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "scan error")
			return
		}
		list = append(list, sl)
	}
	// Stock levels above all: a short list here reads as stock that is not
	// there, and somebody reorders against it.
	if err := rows.Err(); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "scan error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (m *Module) listStockMovementsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}

	rows, err := m.db.Query(r.Context(),
		`SELECT id, tenant_id, warehouse_id, product_id, quantity_change, reference, created_at 
		 FROM stock_movements WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	list := make([]StockMovement, 0)
	for rows.Next() {
		var sm StockMovement
		if err := rows.Scan(&sm.ID, &sm.TenantID, &sm.WarehouseID, &sm.ProductID, &sm.QuantityChange, &sm.Reference, &sm.CreatedAt); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "scan error")
			return
		}
		list = append(list, sm)
	}
	// The movement ledger is what a stock figure is reconciled against, so a
	// silently short one is worse than an error.
	if err := rows.Err(); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "scan error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (m *Module) adjustStockHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		WarehouseID    string  `json:"warehouse_id"`
		ProductID      string  `json:"product_id"`
		QuantityChange float64 `json:"quantity_change"`
		Reference      string  `json:"reference"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.WarehouseID == "" || req.ProductID == "" || req.QuantityChange == 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid adjustment payload")
		return
	}

	ctx := r.Context()
	tx, err := m.db.Begin(ctx)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "tx begin error")
		return
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Fetch current stock level
	var currentQty float64
	var stockID string
	err = tx.QueryRow(ctx,
		`SELECT id, quantity FROM stock_levels WHERE tenant_id = $1 AND warehouse_id = $2 AND product_id = $3 FOR UPDATE`,
		claims.TenantID, req.WarehouseID, req.ProductID).Scan(&stockID, &currentQty)

	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusInternalServerError, "failed to fetch current stock")
		return
	}

	newQty := currentQty + req.QuantityChange
	if newQty < 0 && !m.allowNegativeStock {
		httpx.Error(w, http.StatusBadRequest, "stock adjustment rejected: negative stock is not allowed")
		return
	}

	now := time.Now()
	if errors.Is(err, pgx.ErrNoRows) {
		stockID = uuid.New().String()
		_, err = tx.Exec(ctx,
			`INSERT INTO stock_levels (id, tenant_id, warehouse_id, product_id, quantity, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			stockID, claims.TenantID, req.WarehouseID, req.ProductID, newQty, now, now)
	} else {
		_, err = tx.Exec(ctx,
			`UPDATE stock_levels SET quantity = $1, updated_at = $2 WHERE id = $3`,
			newQty, now, stockID)
	}

	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to update stock level")
		return
	}

	// Record append-only stock movement
	movID := uuid.New().String()
	_, err = tx.Exec(ctx,
		`INSERT INTO stock_movements (id, tenant_id, warehouse_id, product_id, quantity_change, reference, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		movID, claims.TenantID, req.WarehouseID, req.ProductID, req.QuantityChange, req.Reference, now)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to record stock movement")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to commit stock adjustment")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":          "success",
		"previous_qty":    currentQty,
		"new_qty":         newQty,
		"quantity_change": req.QuantityChange,
	})
}
