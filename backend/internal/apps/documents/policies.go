package documents

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/audit"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/httpx"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/tenant"
	"github.com/go-chi/chi/v5"
)

// SignaturePolicy says how a document type may be signed. Every type has an
// effective policy: a type nobody has configured allows both national channels
// and names no signer, which is how the app behaved before the table existed.
type SignaturePolicy struct {
	DocType            string     `json:"doc_type"`
	AllowEID           bool       `json:"allow_eid"`
	AllowDAN           bool       `json:"allow_dan"`
	RequireNamedSigner bool       `json:"require_named_signer"`
	Configured         bool       `json:"configured"`
	UpdatedAt          *time.Time `json:"updated_at,omitempty"`
}

// defaultSignaturePolicy is the policy a type falls back to while no row exists.
func defaultSignaturePolicy(docType string) SignaturePolicy {
	return SignaturePolicy{DocType: docType, AllowEID: true, AllowDAN: true}
}

func (p SignaturePolicy) allows(method string) bool {
	switch method {
	case SignerEID:
		return p.AllowEID
	case SignerDAN:
		return p.AllowDAN
	default:
		return false
	}
}

// SignaturePolicyFor reads the stored policy for a type, or the default when the
// tenant has not configured one.
func (m *DocumentsModule) SignaturePolicyFor(ctx context.Context, tenantID, docType string) (SignaturePolicy, error) {
	policy := defaultSignaturePolicy(docType)
	var updatedAt time.Time

	err := m.db.QueryRow(ctx,
		`SELECT allow_eid, allow_dan, require_named_signer, updated_at
		   FROM document_signature_policies
		  WHERE tenant_id = $1 AND doc_type = $2`, tenantID, docType).
		Scan(&policy.AllowEID, &policy.AllowDAN, &policy.RequireNamedSigner, &updatedAt)
	if err != nil {
		if isNoRows(err) {
			return policy, nil
		}
		return policy, fmt.Errorf("load signature policy: %w", err)
	}

	policy.Configured = true
	policy.UpdatedAt = &updatedAt
	return policy, nil
}

// ListSignaturePolicies returns the effective policy for every document type, so
// the screen shows what actually applies rather than only the rows that exist.
func (m *DocumentsModule) ListSignaturePolicies(ctx context.Context, tenantID string) ([]SignaturePolicy, error) {
	stored := map[string]SignaturePolicy{}

	rows, err := m.db.Query(ctx,
		`SELECT doc_type, allow_eid, allow_dan, require_named_signer, updated_at
		   FROM document_signature_policies WHERE tenant_id = $1`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("query signature policies: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var policy SignaturePolicy
		var updatedAt time.Time
		if err := rows.Scan(&policy.DocType, &policy.AllowEID, &policy.AllowDAN,
			&policy.RequireNamedSigner, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan signature policy: %w", err)
		}
		policy.Configured = true
		policy.UpdatedAt = &updatedAt
		stored[policy.DocType] = policy
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	list := make([]SignaturePolicy, 0, len(DocTypes))
	for _, docType := range DocTypes {
		if policy, ok := stored[docType]; ok {
			list = append(list, policy)
			continue
		}
		list = append(list, defaultSignaturePolicy(docType))
	}
	return list, nil
}

// SaveSignaturePolicy upserts the policy for one document type.
func (m *DocumentsModule) SaveSignaturePolicy(ctx context.Context, tenantID string, policy SignaturePolicy) (*SignaturePolicy, error) {
	docType := strings.ToUpper(strings.TrimSpace(policy.DocType))
	if !slices.Contains(DocTypes, docType) {
		return nil, fmt.Errorf("%w: invalid doc_type %q", ErrInvalidConfiguration, docType)
	}
	if !policy.AllowEID && !policy.AllowDAN {
		return nil, fmt.Errorf("%w: a policy must allow at least one of E-ID or DAN, otherwise the type cannot be signed", ErrInvalidConfiguration)
	}

	// The policy and the chain are one setting in practice: requiring a named signer
	// means every step has to name one, and name a different one — a step left open
	// could never be filled, and two steps naming the same citizen could never both
	// be, because one citizen signs a document once. Either way the type would
	// become unapprovable by anybody.
	//
	// So the check and the write share a transaction holding the same lock the
	// chain screen takes. Read outside one, the two screens could each pass their
	// guard on a stale view of the other and commit exactly the state both forbid:
	// one saving a chain that names nobody while the other turns the requirement on.
	tx, err := m.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin signature policy update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2, 0))`,
		tenantID, docType); err != nil {
		return nil, fmt.Errorf("lock approval chain: %w", err)
	}

	if policy.RequireNamedSigner {
		steps, err := m.workflowStepsTx(ctx, tx, tenantID, docType)
		if err != nil {
			return nil, err
		}
		if err := stepsCanRequireNamedSigners(docType, steps); err != nil {
			return nil, err
		}
	}

	saved := SignaturePolicy{DocType: docType, Configured: true}
	var updatedAt time.Time
	err = tx.QueryRow(ctx,
		`INSERT INTO document_signature_policies
		        (tenant_id, doc_type, allow_eid, allow_dan, require_named_signer, updated_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())
		 ON CONFLICT (tenant_id, doc_type) DO UPDATE
		    SET allow_eid = EXCLUDED.allow_eid,
		        allow_dan = EXCLUDED.allow_dan,
		        require_named_signer = EXCLUDED.require_named_signer,
		        updated_at = NOW()
		 RETURNING allow_eid, allow_dan, require_named_signer, updated_at`,
		tenantID, docType, policy.AllowEID, policy.AllowDAN, policy.RequireNamedSigner).
		Scan(&saved.AllowEID, &saved.AllowDAN, &saved.RequireNamedSigner, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("save signature policy: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit signature policy: %w", err)
	}

	saved.UpdatedAt = &updatedAt

	audit.Record(ctx, tenantID, actorFor(ctx), "documents.signature_policy_changed", docType, map[string]any{
		"allow_eid":            saved.AllowEID,
		"allow_dan":            saved.AllowDAN,
		"require_named_signer": saved.RequireNamedSigner,
	})

	return &saved, nil
}

func (m *DocumentsModule) listSignaturePoliciesHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}

	list, err := m.ListSignaturePolicies(r.Context(), tenantID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to fetch signature policies")
		return
	}
	httpx.JSON(w, http.StatusOK, list)
}

func (m *DocumentsModule) saveSignaturePolicyHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}

	var req struct {
		AllowEID           bool `json:"allow_eid"`
		AllowDAN           bool `json:"allow_dan"`
		RequireNamedSigner bool `json:"require_named_signer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid signature policy payload")
		return
	}

	saved, err := m.SaveSignaturePolicy(r.Context(), tenantID, SignaturePolicy{
		DocType:            chi.URLParam(r, "docType"),
		AllowEID:           req.AllowEID,
		AllowDAN:           req.AllowDAN,
		RequireNamedSigner: req.RequireNamedSigner,
	})
	if err != nil {
		writeWriteFailure(r.Context(), w, err, "failed to save the signature policy")
		return
	}
	httpx.JSON(w, http.StatusOK, saved)
}
