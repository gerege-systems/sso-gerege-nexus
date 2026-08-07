/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Package billing implements Public Billing & e-Barimt tax receipt Go module (io.example.billing).
 */

package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/appregistry"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/tenant"
)

// VATRate is the Mongolian value added tax rate applied to e-Barimt invoices.
const VATRate = 0.10

type Invoice struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	InvoiceNumber string    `json:"invoice_number"`
	ContactName   string    `json:"contact_name"`
	Amount        float64   `json:"amount"`
	VatAmount     float64   `json:"vat_amount"`
	EBarimtStatus string    `json:"ebarimt_status"`
	Status        string    `json:"status"` // PENDING, PAID, CANCELLED
	CreatedAt     time.Time `json:"created_at"`
}

type BillingModule struct {
	db *pgxpool.Pool
}

// New builds the module and registers it in the compile-time app registry.
// Without registration the app store refused to install io.example.billing
// ("module is not present in binary registry") and its menu never appeared.
func New(db *pgxpool.Pool) *BillingModule {
	m := &BillingModule{db: db}
	appregistry.Register(m)
	return m
}

func (m *BillingModule) ID() string      { return "io.example.billing" }
func (m *BillingModule) Name() string    { return "Public Billing & e-Barimt" }
func (m *BillingModule) Version() string { return "1.0.0" }

func (m *BillingModule) Dependencies() []internal.Dependency {
	return []internal.Dependency{
		{ID: "io.example.contacts", VersionConstraint: "^1.0.0"},
	}
}

func (m *BillingModule) Permissions() []internal.PermissionDefinition {
	return []internal.PermissionDefinition{
		{Code: "billing.read", Name: "Read Invoices", Description: "View invoices and e-Barimt receipts"},
		{Code: "billing.manage", Name: "Manage Invoices", Description: "Issue invoices and submit e-Barimt receipts"},
	}
}

func (m *BillingModule) Menus() []internal.MenuDefinition {
	return []internal.MenuDefinition{
		{ID: "billing", ParentID: "operations", Label: "Public Billing", Path: "/billing", Icon: "credit-card", Order: 20, Labels: map[string]string{"mn": "Нэхэмжлэх"}},
	}
}

func (m *BillingModule) RegisterRoutes(r chi.Router, tenantAuthMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/billing", func(br chi.Router) {
		br.Use(tenantAuthMiddleware)
		br.Get("/invoices", m.listInvoicesHandler)
		br.Post("/invoices", m.createInvoiceHandler)
	})
}

func (m *BillingModule) listInvoicesHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	list, err := m.ListInvoices(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch invoices")
		return
	}

	writeJSON(w, http.StatusOK, list)
}

func (m *BillingModule) createInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		ContactName string  `json:"contact_name"`
		Amount      float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ContactName == "" || req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "invalid invoice parameters: contact_name and a positive amount are required")
		return
	}

	inv, err := m.CreateInvoice(r.Context(), tenantID, req.ContactName, req.Amount)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, inv)
}

func (m *BillingModule) CreateInvoice(ctx context.Context, tenantID, contactName string, amount float64) (*Invoice, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("invoice amount must be positive")
	}

	vat := amount * VATRate
	invNo := fmt.Sprintf("INV-%d", time.Now().UnixNano()/1e6)

	var inv Invoice
	const query = `
		INSERT INTO billing_invoices (tenant_id, invoice_number, contact_name, amount, vat_amount, ebarimt_status, status)
		VALUES ($1, $2, $3, $4, $5, 'SENT_TO_ETAX', 'PENDING')
		RETURNING id, tenant_id, invoice_number, contact_name, amount, vat_amount, ebarimt_status, status, created_at
	`
	// A failed INSERT used to be swallowed and answered with a hard-coded
	// "inv_demo_100" invoice, so callers were told an invoice existed when
	// nothing had been written.
	err := m.db.QueryRow(ctx, query, tenantID, invNo, contactName, amount, vat).Scan(
		&inv.ID, &inv.TenantID, &inv.InvoiceNumber, &inv.ContactName, &inv.Amount,
		&inv.VatAmount, &inv.EBarimtStatus, &inv.Status, &inv.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create invoice: %w", err)
	}

	return &inv, nil
}

func (m *BillingModule) ListInvoices(ctx context.Context, tenantID string) ([]Invoice, error) {
	const query = `SELECT id, tenant_id, invoice_number, contact_name, amount, vat_amount, ebarimt_status, status, created_at
	                 FROM billing_invoices WHERE tenant_id = $1 ORDER BY created_at DESC`
	rows, err := m.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list invoices: %w", err)
	}
	defer rows.Close()

	list := make([]Invoice, 0)
	for rows.Next() {
		var inv Invoice
		if err := rows.Scan(&inv.ID, &inv.TenantID, &inv.InvoiceNumber, &inv.ContactName, &inv.Amount,
			&inv.VatAmount, &inv.EBarimtStatus, &inv.Status, &inv.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan invoice: %w", err)
		}
		list = append(list, inv)
	}
	return list, rows.Err()
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
