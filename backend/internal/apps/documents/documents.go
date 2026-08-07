/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Package documents implements Digital Documents & E-Signatures Go module (io.example.documents).
 */

package documents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/appregistry"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/dan"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/eid"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/security"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/tenant"
	"golang.org/x/time/rate"
)

// DocTypes enumerates the accepted document classifications. The column has no
// CHECK constraint, so validation happens here.
var DocTypes = []string{"CONTRACT", "REQUEST", "APPROVAL"}

// Document status lifecycle: DRAFT -> PENDING_APPROVAL (RouteDocument) ->
// APPROVED once the approval chain is complete, or REJECTED. A document can only
// be signed or rejected while it is pending.
//
// CreateDocument writes PENDING_APPROVAL directly, so nothing the app creates is
// a draft; DRAFT is the column default and RouteDocument is what moves a row that
// arrived any other way.
const (
	StatusDraft    = "DRAFT"
	StatusPending  = "PENDING_APPROVAL"
	StatusApproved = "APPROVED"
	StatusRejected = "REJECTED"
)

// SignerMethod is the national identity channel used to apply the signature.
const (
	SignerEID = "EID" // eidmongolia.mn / sso.gov.mn digital signature
	SignerDAN = "DAN" // dan.gerege.mn SSO gateway
)

// ErrNotSignable is returned when a sign/reject is attempted on a document that
// is missing, belongs to another tenant, or is no longer pending. The three
// cases are deliberately indistinguishable so the endpoint cannot be used to
// discover which documents exist in someone else's tenant.
var ErrNotSignable = errors.New("document not found or is not awaiting approval")

// ErrSignatureRejected is returned when the signature was refused: an identity
// the provider would not verify, a channel the type's policy does not allow, or a
// signer its approval chain does not name. It is the caller's problem to fix, so
// the wrapped message is safe to show them — unlike a storage failure, which is
// reported as a plain server error.
var ErrSignatureRejected = errors.New("signature rejected")

// ErrInvalidDocument is returned when what the caller sent cannot make a
// document: an empty or over-long title, an unknown type. It is theirs to fix, so
// the message is safe to show them — a storage failure is not, and is reported as
// a plain server error instead.
var ErrInvalidDocument = errors.New("invalid document")

// ErrInvalidConfiguration is returned when a template, approval chain, signature
// policy or retention rule the caller sent cannot be stored as asked. Like
// ErrInvalidDocument its message is meant to be read by whoever sent it.
var ErrInvalidConfiguration = errors.New("invalid configuration")

// ErrProviderUnavailable is returned when the identity provider could not be
// reached or answered in a way we could not use. It is nobody's mistake and it may
// work in a moment, so it is reported as a temporary server-side failure — a
// polling client must retry it rather than abandon a ceremony the citizen may be
// about to complete.
var ErrProviderUnavailable = errors.New("identity provider unavailable")

type Document struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	Title    string `json:"title"`
	DocType  string `json:"doc_type"` // CONTRACT, REQUEST, APPROVAL
	Status   string `json:"status"`   // DRAFT, PENDING_APPROVAL, APPROVED, REJECTED
	// The newest signature, mirrored onto the record so the list stays one query.
	// The full history is in document_signatures.
	SignedBy        string     `json:"signed_by,omitempty"`
	SignatureHash   string     `json:"signature_hash,omitempty"`
	SignerRegNumber string     `json:"signer_reg_number,omitempty"`
	SignerMethod    string     `json:"signer_method,omitempty"`
	SignedAt        *time.Time `json:"signed_at,omitempty"`
	// Progress through the approval chain: how many signatures the document carries
	// and how many it asked for.
	SignatureCount     int `json:"signature_count"`
	RequiredSignatures int `json:"required_signatures"`
	// OutstandingSteps is how many steps of the document's own chain no signature has
	// filled. Completion needs this to be zero as well as the count to be met, so a
	// screen that only compared the two numbers could paint "complete" over a document
	// whose named step was still owed — a signature from somebody no step names counts
	// toward the total and satisfies none of the chain.
	OutstandingSteps int       `json:"outstanding_steps"`
	CreatedAt        time.Time `json:"created_at"`
}

type DocumentsModule struct {
	db     *pgxpool.Pool
	eidSvc *eid.EIDService
	danSvc *dan.DANService

	// Whether this cluster has an ICU collation for the title search to lean on. See
	// titleMatch: without one, a Cyrillic search is case-sensitive on a database
	// initialised with LC_CTYPE=C, which is what the postgres:16-alpine image this
	// stack deploys produces.
	// signLimiter budgets the requests that reach a citizen's phone. See New.
	signLimiter *security.IPRateLimiter

	collationMu sync.Mutex
	// collationKnown is set only when the answer is definitive. A failed probe must not
	// latch: it would leave a case-sensitive Cyrillic search for the life of the
	// process, on the strength of one transient error.
	collationKnown bool
	hasICU         bool
}

// New builds the module and registers it in the compile-time app registry so
// the app store can resolve and install io.example.documents. The module owns
// its own E-ID and DAN clients so signing stays self-contained; both read their
// configuration from the environment and default to mock mode.
// Starting an E-ID signature pushes a notification to a real person's phone, so it is
// budgeted the way the platform budgets its own sign-in push rather than left open.
// Measured before this existed: thirty starts in a row against one document all went
// through, which in production is thirty buzzes on one citizen's phone from one
// operator's session — while /auth/eid/start-id, which does the same thing, refused
// eight of twelve.
//
// A little more generous than sign-in, because several clerks in one office share an
// address and each of them getting a document signed is legitimate; still tight,
// because each request reaches somebody's pocket. Polling and reading are not budgeted:
// they trouble nobody.
//
// The burst is the length of the longest chain a tenant may configure, because a chain
// really is signed in one sitting sometimes — ten people at one counter, one after
// another, all from the office's address. A burst of five turned that into a refusal
// half way through, which I found by running one. The sustained rate is what matters
// against guessing a six-digit DAN code, and ten a minute leaves that hopeless.
const (
	signPushRatePerMinute = 10
	signPushBurst         = maxChainSteps
)

func New(db *pgxpool.Pool) *DocumentsModule {
	m := &DocumentsModule{
		db:          db,
		eidSvc:      eid.NewEIDService(),
		danSvc:      dan.NewDANService(),
		signLimiter: security.NewIPRateLimiter(rate.Limit(float64(signPushRatePerMinute)/60.0), signPushBurst),
	}
	appregistry.Register(m)
	return m
}

func (m *DocumentsModule) ID() string      { return "io.example.documents" }
func (m *DocumentsModule) Name() string    { return "Digital Documents & Signatures" }
func (m *DocumentsModule) Version() string { return "1.0.0" }

func (m *DocumentsModule) Dependencies() []internal.Dependency { return nil }

func (m *DocumentsModule) Permissions() []internal.PermissionDefinition {
	return []internal.PermissionDefinition{
		{Code: "documents.read", Name: "Read Documents", Description: "View documents and signature status"},
		{Code: "documents.manage", Name: "Manage Documents", Description: "Create documents, route them for approval, and configure templates, approval chains, signature policies and retention rules"},
		{Code: "documents.sign", Name: "Sign Documents", Description: "Apply an E-ID / DAN digital signature or reject a document"},
	}
}

func (m *DocumentsModule) Menus() []internal.MenuDefinition {
	return []internal.MenuDefinition{
		{ID: "documents", ParentID: "operations", Label: "Documents & E-Sign", Path: "/documents", Icon: "file-text", Order: 30, Labels: map[string]string{"mn": "Баримт ба цахим гарын үсэг"}},
	}
}

// bodyLimit is the most any request to this module legitimately carries. The largest
// is an approval chain: ten steps, each a name of at most 255 characters and a
// registration number of at most 64 — about four kilobytes. Everything else is a title,
// a note or a registration number.
//
// Measured before this existed: a 143 MB body took the API's resident memory from 86 MB
// to 444 MB in six tenths of a second, and was then refused for having more than ten
// steps. The refusal was correct and far too late — the whole thing had already been
// read and parsed into Go objects. A few of those at once would take the box down, and
// on this stack the database is on the same box.
const bodyLimit = 64 << 10 // 64 KB, sixteen times the largest real request

// limitBody refuses an over-large request before it is read, and caps what a
// chunked one can stream. It sits on the module's whole route group so no handler
// can forget it.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > bodyLimit {
			writeError(w, http.StatusRequestEntityTooLarge,
				fmt.Sprintf("the request body is larger than %d bytes", bodyLimit))
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, bodyLimit)
		next.ServeHTTP(w, r)
	})
}

func (m *DocumentsModule) RegisterRoutes(r chi.Router, tenantAuthMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/documents", func(dr chi.Router) {
		dr.Use(tenantAuthMiddleware)
		dr.Use(limitBody)
		dr.Get("/", m.listDocumentsHandler)
		dr.Post("/", m.createDocumentHandler)

		// Templates a document is started from.
		dr.Get("/templates", m.listTemplatesHandler)
		dr.Post("/templates", m.createTemplateHandler)
		dr.Put("/templates/{id}", m.updateTemplateHandler)
		dr.Delete("/templates/{id}", m.deleteTemplateHandler)
		dr.Post("/templates/{id}/use", m.useTemplateHandler)

		// How a document type may be signed.
		dr.Get("/policies", m.listSignaturePoliciesHandler)
		dr.Put("/policies/{docType}", m.saveSignaturePolicyHandler)

		// Who must sign it, in order.
		dr.Get("/workflows", m.listWorkflowsHandler)
		dr.Put("/workflows/{docType}", m.saveWorkflowHandler)

		// How long it is kept.
		dr.Get("/retention", m.listRetentionRulesHandler)
		dr.Put("/retention/{docType}", m.saveRetentionRuleHandler)

		// A single document. Static segments above win over {id} in chi's trie,
		// so "templates" and "policies" are never read as document ids.
		dr.Get("/{id}/signatures", m.listSignaturesHandler)
		dr.Get("/{id}/steps", m.listDocumentStepsHandler)
		dr.Post("/{id}/route", m.routeDocumentHandler)
		dr.Post("/{id}/reject", m.rejectDocumentHandler)

		// Signing is per channel, because the channels are not the same shape.
		// E-ID is an approval the citizen gives on their own device, so it takes
		// two calls; DAN is a code they read out, so it takes one.
		//
		// The start is budgeted: it is the one that reaches a citizen's phone. The poll
		// is not — a citizen takes as long as they take to find it, and throttling the
		// watching would only lose approvals. DAN is a code the citizen reads out, so it
		// pushes nothing, but it is budgeted with the same bucket because it is still an
		// authentication attempt against a real person's credentials.
		if m.signLimiter != nil {
			dr.With(security.RateLimitMiddleware(m.signLimiter)).
				Post("/{id}/sign/eid/start", m.startEIDSignatureHandler)
			dr.With(security.RateLimitMiddleware(m.signLimiter)).
				Post("/{id}/sign/dan", m.signWithDANHandler)
		} else {
			// A module built by hand in a test has no limiter; the routes still work.
			dr.Post("/{id}/sign/eid/start", m.startEIDSignatureHandler)
			dr.Post("/{id}/sign/dan", m.signWithDANHandler)
		}
		dr.Post("/{id}/sign/eid/poll", m.pollEIDSignatureHandler)
	})
}

func (m *DocumentsModule) listDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	query := r.URL.Query()
	filter := DocumentFilter{
		Status:  query.Get("status"),
		DocType: query.Get("doc_type"),
		Search:  query.Get("q"),
		Order:   query.Get("order"),
		AfterID: strings.TrimSpace(query.Get("after_id")),
	}
	if raw := strings.TrimSpace(query.Get("after_at")); raw != "" {
		at, parseErr := time.Parse(time.RFC3339Nano, raw)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "after_at must be an RFC3339 timestamp")
			return
		}
		filter.AfterAt = at
	}
	// Both halves name one row, so one without the other would silently page from
	// somewhere nobody asked for.
	if (filter.AfterID == "") != filter.AfterAt.IsZero() {
		writeError(w, http.StatusBadRequest, "after_at and after_id go together")
		return
	}
	page, err := m.ListDocuments(r.Context(), tenantID, filter, limit, offset)
	if errors.Is(err, ErrInvalidDocument) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to fetch documents", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch documents")
		return
	}

	writeJSON(w, http.StatusOK, page)
}

func (m *DocumentsModule) createDocumentHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Title   string `json:"title"`
		DocType string `json:"doc_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		writeError(w, http.StatusBadRequest, "invalid document parameters: title is required")
		return
	}

	doc, err := m.CreateDocument(r.Context(), tenantID, req.Title, req.DocType)
	if err != nil {
		writeWriteFailure(r.Context(), w, err, "failed to create document")
		return
	}

	writeJSON(w, http.StatusCreated, doc)
}

func (m *DocumentsModule) signWithDANHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	docID := chi.URLParam(r, "id")
	var req struct {
		RegNumber string `json:"reg_number"` // Регистрийн дугаар
		OTPCode   string `json:"otp_code"`   // Нэг удаагийн нууц код
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RegNumber == "" {
		writeError(w, http.StatusBadRequest, "invalid signature request: reg_number is required")
		return
	}

	doc, err := m.SignWithDAN(r.Context(), tenantID, docID, req.RegNumber, req.OTPCode)
	switch {
	case errors.Is(err, ErrNotSignable):
		writeError(w, http.StatusConflict, err.Error())
		return
	case errors.Is(err, ErrAlreadySigned):
		writeError(w, http.StatusConflict, err.Error())
		return
	case errors.Is(err, ErrProviderUnavailable):
		// The gateway, not the caller. Same 503 the E-ID routes give, and for the same
		// reason: it is worth trying again, and it is not something the operator did.
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	case errors.Is(err, ErrSignatureRejected):
		writeError(w, http.StatusBadRequest, err.Error())
		return
	case err != nil:
		// A storage failure is ours, not the caller's: report it as one and keep
		// the driver's message out of the response.
		slog.ErrorContext(r.Context(), "failed to sign document through DAN", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to sign document")
		return
	}

	writeJSON(w, http.StatusOK, doc)
}

func (m *DocumentsModule) rejectDocumentHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	docID := chi.URLParam(r, "id")
	doc, err := m.RejectDocument(r.Context(), tenantID, docID)
	switch {
	case errors.Is(err, ErrNotSignable):
		writeError(w, http.StatusConflict, err.Error())
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "failed to reject document")
		return
	}

	writeJSON(w, http.StatusOK, doc)
}

// textFault says why a string cannot be stored as Postgres text, or "" when it can.
//
// Postgres text holds neither a NUL character nor a byte sequence that is not UTF-8, so
// a value carrying either is refused by the driver — and every field in this module
// answered that with 500 "we could not save it", which is untrue twice over: nothing
// broke, and it is the caller's input. A 400 that names the fault is the honest answer,
// and it keeps a log line for something nobody can fix out of the error log.
func textFault(value string) string {
	if strings.ContainsRune(value, 0) {
		return "it contains a NUL character"
	}
	if !utf8.ValidString(value) {
		return "it is not valid UTF-8"
	}
	return ""
}

// TitleLimit is what document_records.title holds. Checking it here turns a
// storage error the caller cannot read into one they can act on.
const TitleLimit = 255

// RegNumberLimit is the shortest thing that can be a registration number. Both
// national channels refuse anything shorter, so naming one in an approval chain
// would create a step no citizen could fill.
const RegNumberLimit = 8

func (m *DocumentsModule) CreateDocument(ctx context.Context, tenantID, title, docType string) (*Document, error) {
	title = strings.TrimSpace(title)
	docType = strings.ToUpper(strings.TrimSpace(docType))

	if title == "" {
		return nil, fmt.Errorf("%w: title cannot be empty", ErrInvalidDocument)
	}
	if len([]rune(title)) > TitleLimit {
		return nil, fmt.Errorf("%w: a title is at most %d characters, this one is %d",
			ErrInvalidDocument, TitleLimit, len([]rune(title)))
	}
	if fault := textFault(title); fault != "" {
		return nil, fmt.Errorf("%w: the title cannot be stored — %s", ErrInvalidDocument, fault)
	}
	if docType == "" {
		docType = DocTypes[0]
	}
	if !slices.Contains(DocTypes, docType) {
		return nil, fmt.Errorf("%w: unsupported doc_type %q", ErrInvalidDocument, docType)
	}

	// A document and the chain it will be approved under are created together: a
	// document that started waiting keeps the rules it started under, whatever the
	// type's chain becomes later.
	tx, err := m.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create document: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// The old code answered a failed INSERT with a canned "doc_demo_200"
	// record, reporting success for a document that was never stored.
	var id string
	if err := tx.QueryRow(ctx,
		`INSERT INTO document_records (tenant_id, title, doc_type, status)
		      VALUES ($1, $2, $3, $4) RETURNING id`,
		tenantID, title, docType, StatusPending).Scan(&id); err != nil {
		return nil, fmt.Errorf("create document: %w", err)
	}

	if err := m.snapshotApprovalChain(ctx, tx, tenantID, id, docType); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create document: %w", err)
	}

	// Recorded before anything else that can fail. Creating a document is what puts
	// it into the approval queue and pins the chain it will be approved under, and
	// document_records holds no created_by — so this is the only place "who drafted
	// this contract" is ever answered. Ordering it after the read below would lose the
	// record for a document that is already committed, on a read that has nothing to
	// do with whether it was created.
	//
	// The chain is read best-effort for the same reason it is recorded at all: the
	// type's chain may be edited later without reaching this document, so what it was
	// given is only knowable from here.
	given, err := m.stepsForDocumentTx(ctx, m.db, tenantID, id)
	if err != nil {
		slog.WarnContext(ctx, "created the document but could not read back its chain for the record",
			"document_id", id, "error", err)
	}
	audit.Record(ctx, tenantID, actorFor(ctx), "documents.created", id, map[string]any{
		"doc_type": docType, "title": title, "status": StatusPending, "chain": given,
	})

	return m.getDocument(ctx, tenantID, id)
}

// verifiedSignature is what a national identity channel hands back once it has
// vouched for the signer.
type verifiedSignature struct {
	SignerName string
	RegNumber  string
	// Hash references the act of approval: the DAN session for DAN, the eID
	// session for E-ID.
	Hash string
	// CertificateSerial and CertificateIssuer are the citizen's eID certificate,
	// when the provider returned one. They say whose key gave the approval, which
	// is what a dispute turns on; a session id alone only says one happened.
	CertificateSerial string
	CertificateIssuer string
}

// signaturePreflight is what the document's own configuration decides before a
// citizen is asked to approve anything.
type signaturePreflight struct {
	DocType  string
	Policy   SignaturePolicy
	Required int
	Position *approvalPosition
}

// preflightSignature checks that the document is still awaiting approval, that
// the tenant's policy allows this channel, and which step of the document's own
// chain comes next. Both signing paths run it: E-ID before pushing a request to
// somebody's phone, DAN before asking for a code — there is no point troubling a
// citizen for a document their signature cannot land on.
func (m *DocumentsModule) preflightSignature(ctx context.Context, tenantID, docID, method string) (*signaturePreflight, error) {
	// document_records.id is a uuid column, so a malformed path parameter would
	// otherwise surface as a storage error. It is simply not a document we hold.
	if uuid.Validate(docID) != nil {
		return nil, ErrNotSignable
	}

	var docType string
	var required int
	err := m.db.QueryRow(ctx,
		`SELECT doc_type, required_signatures FROM document_records
		  WHERE id = $1 AND tenant_id = $2 AND status = $3`,
		docID, tenantID, StatusPending).Scan(&docType, &required)
	if isNoRows(err) {
		return nil, ErrNotSignable
	}
	if err != nil {
		return nil, fmt.Errorf("load document for signing: %w", err)
	}

	policy, err := m.SignaturePolicyFor(ctx, tenantID, docType)
	if err != nil {
		return nil, err
	}
	if !policy.allows(method) {
		return nil, fmt.Errorf("%w: %s documents may not be signed through %s under this tenant's policy",
			ErrSignatureRejected, docType, method)
	}

	position, err := m.nextApprovalStep(ctx, m.db, tenantID, docID)
	if err != nil {
		return nil, err
	}
	return &signaturePreflight{DocType: docType, Policy: policy, Required: required, Position: position}, nil
}

// alreadySigned reports whether this citizen has already signed this document. The
// unique constraint would catch it either way, but only after the citizen had been
// asked to approve on their own device and their approval discarded.
func (m *DocumentsModule) alreadySigned(ctx context.Context, q querier, tenantID, docID, regNumber string) (bool, error) {
	var signed bool
	if err := q.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM document_signatures
		                 WHERE tenant_id = $1 AND document_id = $2 AND signer_reg_number = $3)`,
		tenantID, docID, regNumber).Scan(&signed); err != nil {
		return false, fmt.Errorf("check whether the signer already signed: %w", err)
	}
	return signed, nil
}

// checkSigner holds the signature to whoever the document's next step names, and
// holds back anyone a later step names. The check runs against the identity a
// provider vouched for, never against what the caller typed.
func checkSigner(pos *approvalPosition, docType, regNumber string) error {
	if pos == nil || pos.Next == nil {
		return nil // no chain: one open signature approves
	}

	if named := pos.Next.SignerRegNumber; named != "" {
		if named == regNumber {
			return nil
		}
		return fmt.Errorf("%w: step %d of the %s chain (%s) is reserved for %s, not %s",
			ErrSignatureRejected, pos.Next.Order, docType, pos.Next.Name, named, regNumber)
	}

	// The step is open — but not to somebody the chain still needs further along.
	// A citizen signs a document once, so letting them spend their signature here
	// would leave their own step unfillable and the document unapprovable.
	if slices.Contains(pos.Reserved, regNumber) {
		return fmt.Errorf("%w: %s is named by a later step of the %s chain, so their signature is held for it — step %d (%s) is open to anyone else",
			ErrSignatureRejected, regNumber, docType, pos.Next.Order, pos.Next.Name)
	}
	return nil
}

// SignWithDAN signs through dan.gerege.mn's registration-number and one-time-code
// gateway. DAN exposes no approval-push flow in this platform, so unlike E-ID it
// is still a single call — and it only succeeds while DAN_MOCK_MODE is on, because
// the live DAN client has no implementation for it.
func (m *DocumentsModule) SignWithDAN(ctx context.Context, tenantID, docID, regNumber, otpCode string) (*Document, error) {
	// Refused before the gateway is troubled, on the same terms as the E-ID path: a
	// number this module knows cannot name a step is not worth a round trip, and the
	// caller gets told what is wrong with it rather than what DAN thought of it.
	if fault := textFault(regNumber); fault != "" {
		return nil, fmt.Errorf("%w: that registration number cannot be stored — %s",
			ErrSignatureRejected, fault)
	}
	if !plausibleRegNumber(normaliseRegNumber(regNumber)) {
		return nil, fmt.Errorf("%w: %q is not a registration number", ErrSignatureRejected, regNumber)
	}

	pre, err := m.preflightSignature(ctx, tenantID, docID, SignerDAN)
	if err != nil {
		return nil, err
	}

	profile, err := m.danSvc.AuthenticateDANCitizen(ctx, regNumber, otpCode)
	if errors.Is(err, dan.ErrUnavailable) {
		// The channel is not there, which is not the same as the citizen being refused.
		// Answering 400 told an operator their signature had been rejected every time
		// they picked DAN on a deployment without a live gateway — which is every
		// deployment today, since DAN_MOCK_MODE is off in production and the live OTP
		// client does not exist. This is the same 503 the E-ID path gives for the same
		// reason, and it is the answer a client may retry.
		return nil, fmt.Errorf("%w: DAN could not be reached: %w", ErrProviderUnavailable, err)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: DAN verification failed: %w", ErrSignatureRejected, err)
	}
	// Normalised here, not trusted from the provider. Three decisions compare this
	// string — whether a named step belongs to this citizen, whether they have already
	// signed, and the ledger's one-signature-per-signer constraint — and the E-ID path
	// normalises its own answer for the same reason. A provider returning 'aa90010111'
	// where a step names 'AA90010111' would lock the named signer out of their own step,
	// and let them sign a second time in the other case.
	signature := &verifiedSignature{
		SignerName: strings.TrimSpace(profile.FirstName+" "+profile.LastName) + " (DAN баталгаажсан)",
		RegNumber:  normaliseRegNumber(profile.RegNumber),
		Hash:       "dan_sig_" + profile.DANSessionID,
	}

	if err := checkSigner(pre.Position, pre.DocType, signature.RegNumber); err != nil {
		return nil, err
	}
	signed, err := m.alreadySigned(ctx, m.db, tenantID, docID, signature.RegNumber)
	if err != nil {
		return nil, err
	}
	if signed {
		return nil, ErrAlreadySigned
	}
	return m.recordSignature(ctx, tenantID, docID, SignerDAN, signature, "")
}

// recordSignature writes the signature and decides whether the chain is complete.
//
// Everything the decision rests on is read inside one transaction with the
// document row locked: the requirement from the row itself, the count of
// signatures already applied, and which step comes next. An earlier version took
// the requirement from a pool read before the lock, which let a chain edited in
// between approve a document on fewer signatures than it asked for.
//
// sessionID, when given, is the eID session this signature came from; it is
// marked spent in the same transaction so a recorded signature and an unspent
// session can never disagree.
func (m *DocumentsModule) recordSignature(ctx context.Context, tenantID, docID, method string, signature *verifiedSignature, sessionID string) (*Document, error) {
	// The citizen has already approved by the time we get here, so an operator
	// closing the dialog must not destroy their signature. The write runs on a
	// context the caller cannot cancel — with its own deadline, so a stalled
	// database still lets go.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()

	tx, err := m.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin signature: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var docType string
	var required int
	err = tx.QueryRow(ctx,
		`SELECT doc_type, required_signatures FROM document_records
		  WHERE id = $1 AND tenant_id = $2 AND status = $3 FOR UPDATE`,
		docID, tenantID, StatusPending).Scan(&docType, &required)
	if isNoRows(err) {
		return nil, ErrNotSignable
	}
	if err != nil {
		return nil, fmt.Errorf("lock document for signing: %w", err)
	}

	position, err := m.nextApprovalStep(ctx, tx, tenantID, docID)
	if err != nil {
		return nil, err
	}
	// The authority check that counts is this one: the preflight's was for a good
	// error message before a citizen was troubled, this one is under the lock.
	if err := checkSigner(position, docType, signature.RegNumber); err != nil {
		return nil, err
	}

	// The signature fills the step it was checked against — not "the next number",
	// which is not the same thing once a signature can sit on a later step than an
	// unfilled earlier one.
	//
	// With no step left to fill, the signature is parked PAST THE END of the chain,
	// the way migration 00017 §4 parks a legacy one. Counting instead — applied + 1 —
	// collides with a real step number as soon as a chain is not numbered 1..n: on a
	// hand-numbered chain of steps 2 and 3, both filled, the third signature would be
	// written as step 3 as well, and the trail would show somebody the chain never
	// named as having given the director's approval.
	var step int
	if position.Next != nil {
		step = position.Next.Order
	} else {
		if err := tx.QueryRow(ctx,
			`SELECT GREATEST(
			          COALESCE((SELECT max(s.step_order) FROM document_signatures s
			                     WHERE s.document_id = $1), 0),
			          COALESCE((SELECT max(st.step_order) FROM document_approval_steps st
			                     WHERE st.document_id = $1), 0)) + 1`,
			docID).Scan(&step); err != nil {
			return nil, fmt.Errorf("find a step number past the chain: %w", err)
		}
	}
	signedAt := time.Now()
	_, err = tx.Exec(ctx,
		`INSERT INTO document_signatures
		        (tenant_id, document_id, signer_name, signer_reg_number,
		         signer_method, signature_hash, signed_at,
		         certificate_serial, certificate_issuer, step_order)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), NULLIF($9, ''), $10)`,
		tenantID, docID, signature.SignerName, signature.RegNumber,
		method, signature.Hash, signedAt,
		signature.CertificateSerial, signature.CertificateIssuer, step)
	// Only the one-per-signer constraint means "you have already signed". The
	// one-per-approval constraint means this module chose a step number another
	// signature already holds, which is a bug in here, not something the caller did
	// — and telling them they have already signed would send them away satisfied
	// while an approval went unrecorded.
	if isConstraintViolation(err, "document_signatures_once_per_signer") {
		return nil, ErrAlreadySigned
	}
	if err != nil {
		return nil, fmt.Errorf("record signature on step %d: %w", step, err)
	}

	// Complete means every step of the document's own chain is filled AND it carries
	// at least as many signatures as it asked for. A document with no chain has no
	// steps to fill, so the count alone decides — which is how a type without a
	// chain has always behaved.
	left, err := m.unfilledSteps(ctx, tx, docID)
	if err != nil {
		return nil, err
	}
	status := StatusPending
	if left == 0 && position.Applied+1 >= required {
		status = StatusApproved
	}

	if _, err := tx.Exec(ctx,
		`UPDATE document_records
		    SET status = $1, signed_by = $2, signature_hash = $3,
		        signer_reg_number = $4, signer_method = $5, signed_at = $6
		  WHERE id = $7 AND tenant_id = $8`,
		status, signature.SignerName, signature.Hash,
		signature.RegNumber, method, signedAt, docID, tenantID); err != nil {
		return nil, fmt.Errorf("apply signature to document: %w", err)
	}

	if sessionID != "" {
		// Spend it, or do not sign. The session row is the pairing that makes this
		// approval a signature on THIS document, so if it is not there to be spent the
		// approval is not one to record: it means another poll declared the session past
		// its deadline and removed it, or the stale sweep did, between the read that let
		// this signature through and this write. The whole transaction goes back, the
		// citizen keeps their signature, and they can be asked again.
		tag, err := tx.Exec(ctx,
			`UPDATE document_eid_sign_sessions SET consumed_at = NOW()
			  WHERE session_id = $1 AND tenant_id = $2 AND consumed_at IS NULL`,
			sessionID, tenantID)
		if err != nil {
			return nil, fmt.Errorf("mark signature session spent: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return nil, fmt.Errorf("%w: the approval session is no longer redeemable", ErrSignSessionUnknown)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit signature: %w", err)
	}

	// Both channels land here, so one record covers every signature the module
	// applies. It carries what the signature was, not only that one happened:
	// which citizen, through which channel, on whose certificate, which step of
	// the chain it filled, and whether it completed it.
	audit.Record(ctx, tenantID, actorFor(ctx), "documents.signed", docID, map[string]any{
		"method":              method,
		"signer_reg_number":   signature.RegNumber,
		"certificate_serial":  signature.CertificateSerial,
		"certificate_issuer":  signature.CertificateIssuer,
		"step_order":          step,
		"signatures_required": required,
		"status":              status,
	})

	return m.getDocument(ctx, tenantID, docID)
}

// RejectDocument moves a pending document to REJECTED. Like signing, it is a
// no-op on anything that is not currently pending.
func (m *DocumentsModule) RejectDocument(ctx context.Context, tenantID, docID string) (*Document, error) {
	if uuid.Validate(docID) != nil {
		return nil, ErrNotSignable
	}

	// What the rejection interrupted is recorded with it. A document rejected with
	// two of three approvals already given is a different event from one rejected
	// untouched, and the row itself keeps no memory of which it was.
	var docType, title string
	var held int
	err := m.db.QueryRow(ctx,
		`UPDATE document_records SET status = $1
		  WHERE id = $2 AND tenant_id = $3 AND status = $4
		  RETURNING doc_type, title,
		            (SELECT count(*) FROM document_signatures s WHERE s.document_id = document_records.id)`,
		StatusRejected, docID, tenantID, StatusPending).Scan(&docType, &title, &held)
	if isNoRows(err) {
		return nil, ErrNotSignable
	}
	if err != nil {
		return nil, fmt.Errorf("reject document: %w", err)
	}

	audit.Record(ctx, tenantID, actorFor(ctx), "documents.rejected", docID, map[string]any{
		"doc_type": docType, "title": title, "signatures_held": held,
	})

	return m.getDocument(ctx, tenantID, docID)
}

// documentColumns is the shape every document read returns. The progress is how
// many signatures the document carries against how many it asked for — read from
// the document's own row, not counted from the type's current chain, so a later
// configuration change cannot rewrite what a decided document required.
const documentColumns = `
	d.id, d.tenant_id, d.title, d.doc_type, d.status,
	COALESCE(d.signed_by, ''), COALESCE(d.signature_hash, ''),
	COALESCE(d.signer_reg_number, ''), COALESCE(d.signer_method, ''),
	d.signed_at, d.created_at,
	(SELECT count(*) FROM document_signatures s WHERE s.document_id = d.id),
	GREATEST(1, d.required_signatures),
	(SELECT count(*) FROM document_approval_steps st
	  WHERE st.document_id = d.id
	    AND NOT EXISTS (SELECT 1 FROM document_signatures s
	                     WHERE s.document_id = st.document_id
	                       AND s.step_order = st.step_order))`

// DocumentPage is what the list endpoint answers with: a bounded slice of a tenant's
// documents, and how many there are in total.
//
// The total is not decoration. A screen that shows the most recent two hundred of four
// thousand documents and does not say so is a screen an operator searches in vain — and
// this list is per-row expensive (each row counts its signatures and its outstanding
// steps), so it cannot simply be unbounded either. Twenty thousand documents came to
// 5.9 MB on every page load before this.
type DocumentPage struct {
	Documents []Document `json:"documents"`
	// Total counts every document the filter matches, not just the ones returned.
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// ListLimitDefault and ListLimitMax bound one page of documents. The default is
// generous enough that most tenants never see a partial list, and the maximum keeps a
// caller from asking for the whole table by hand.
const (
	ListLimitDefault = 200
	ListLimitMax     = 500
)

// ListOrderOldest asks for the oldest documents first. The approval queue wants it:
// a queue is worked from its oldest end, and with the newest first the document that
// has waited longest sits on the last page — where a screen showing one page cannot
// see it, and would report the longest wait as whatever happened to be on page one.
const ListOrderOldest = "oldest"

// titleMatch is the case-insensitive title comparison, spelled so that it actually is
// case-insensitive for Mongolian.
//
// ILIKE folds case according to the DATABASE's ctype. On a cluster initialised with
// LC_CTYPE=C — which is what the postgres:16-alpine image this stack deploys produces,
// because musl has no locales to initdb with — Latin still folds and Cyrillic does not:
// 'ГЭРЭЭ 2026' ILIKE '%гэрээ%' is false. An operator searching for гэрээ would not find
// their own contracts, on the half of the product that is written in Cyrillic.
//
// An explicit ICU collation folds it regardless of how the cluster was initialised
// (measured: true on exactly that C-ctype database). ICU is not guaranteed to be built
// in, though, and naming a collation that does not exist is an error rather than a
// degraded search — so it is asked for once and only used if it is there.
func (m *DocumentsModule) titleMatch(ctx context.Context) string {
	m.collationMu.Lock()
	defer m.collationMu.Unlock()

	// Asked once and remembered — but only a definitive answer is remembered. A probe
	// that fails is retried on the next request, because latching it would trade a
	// transient error for a search that is case-sensitive for Mongolian until the
	// process restarts.
	if !m.collationKnown {
		var present bool
		if err := m.db.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM pg_collation WHERE collname = 'und-x-icu')`).Scan(&present); err != nil {
			slog.WarnContext(ctx, "could not tell whether this database has an ICU collation; "+
				"searching case-sensitively for now and asking again on the next request",
				"error", err)
			return `d.title ILIKE $4 ESCAPE '!'`
		}
		m.hasICU, m.collationKnown = present, true
		if !present {
			slog.WarnContext(ctx, "this database has no ICU collation, so a Cyrillic title "+
				"search is case-sensitive; build Postgres with ICU or initialise the cluster "+
				"with a UTF-8 ctype")
		}
	}
	if m.hasICU {
		return `d.title COLLATE "und-x-icu" ILIKE $4 ESCAPE '!'`
	}
	return `d.title ILIKE $4 ESCAPE '!'`
}

// DocumentFilter narrows a page of documents to what somebody is looking for. Paging
// alone is not enough on a tenant with thousands of them: "Load more" is not a way to
// find a particular contract, it is a way to read the whole list.
type DocumentFilter struct {
	// Status, when given, must be one of the statuses a document can hold.
	Status string
	// DocType, when given, must be one of DocTypes.
	DocType string
	// Search matches anywhere in the title, case-insensitively.
	Search string
	// Order is ListOrderOldest, or empty for newest first.
	Order string
	// AfterAt and AfterID continue from a row already seen: the answer holds only the
	// documents that come after it in this order.
	//
	// This is what a screen reading a long list should use, rather than Offset. Offset
	// counts from the start of a set that other people are changing: approve one
	// document between two requests and the rows shift up, so the next request skips
	// one — and the skipped document is then on no screen at all, which on the approval
	// queue means a signature nobody can give. A cursor names a place instead of a
	// distance, so nothing between here and there can move it.
	AfterAt time.Time
	AfterID string
}

// ListDocuments returns one page of a tenant's documents matching the filter, newest
// first unless asked for ListOrderOldest.
//
// An unknown status or type is refused rather than answered with an empty page: a
// screen cannot tell "nothing matches" from "you asked for something that does not
// exist", and the second is a bug worth surfacing. The approvals queue passes
// PENDING_APPROVAL rather than filtering a capped page in the browser: a document
// waiting for a signature must not fall off the end of a page full of approved ones.
func (m *DocumentsModule) ListDocuments(ctx context.Context, tenantID string, filter DocumentFilter, limit, offset int) (*DocumentPage, error) {
	status := strings.ToUpper(strings.TrimSpace(filter.Status))
	if status != "" && !slices.Contains([]string{StatusDraft, StatusPending, StatusApproved, StatusRejected}, status) {
		return nil, fmt.Errorf("%w: unknown status %q", ErrInvalidDocument, status)
	}
	docType := strings.ToUpper(strings.TrimSpace(filter.DocType))
	if docType != "" && !slices.Contains(DocTypes, docType) {
		return nil, fmt.Errorf("%w: unknown doc_type %q", ErrInvalidDocument, docType)
	}
	// The search is a substring of the title, so the two characters Postgres reads as
	// wildcards are escaped: somebody typing "100%" is looking for a document called
	// that, not for every document.
	//
	// The escape character is declared explicitly in the query (ESCAPE '!') rather than
	// relying on the backslash default, because what a backslash means in a pattern
	// depends on standard_conforming_strings and on how the driver sends the parameter.
	// With '!' there is nothing to reason about: the escape character is escaped by
	// doubling, and the reasoning cannot be broken by a setting somewhere else.
	search := strings.TrimSpace(filter.Search)
	if len([]rune(search)) > TitleLimit {
		return nil, fmt.Errorf("%w: a search is at most %d characters", ErrInvalidDocument, TitleLimit)
	}
	if fault := textFault(search); fault != "" {
		return nil, fmt.Errorf("%w: the search cannot be run — %s", ErrInvalidDocument, fault)
	}
	pattern := ""
	if search != "" {
		escaped := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(search)
		pattern = "%" + escaped + "%"
	}
	if limit <= 0 {
		limit = ListLimitDefault
	}
	if limit > ListLimitMax {
		limit = ListLimitMax
	}
	if offset < 0 {
		offset = 0
	}
	if filter.AfterID != "" && uuid.Validate(filter.AfterID) != nil {
		return nil, fmt.Errorf("%w: after_id is not a document id", ErrInvalidDocument)
	}

	where := `d.tenant_id = $1
		   AND ($2 = '' OR d.status = $2)
		   AND ($3 = '' OR d.doc_type = $3)
		   AND ($4 = '' OR ` + m.titleMatch(ctx) + `)`

	// The count and the rows are two statements, so a document created or deleted
	// between them can leave the total off by one or two. That is deliberate: both ask
	// the same question of the same filter, and a read-only snapshot to make them agree
	// would buy nothing an operator can see — the total exists to say "there is more
	// than this page", not to be reconciled against the rows.
	page := &DocumentPage{Documents: make([]Document, 0), Limit: limit, Offset: offset}
	if err := m.db.QueryRow(ctx,
		`SELECT count(*) FROM document_records d WHERE `+where,
		tenantID, status, docType, pattern).Scan(&page.Total); err != nil {
		return nil, fmt.Errorf("count documents: %w", err)
	}

	// The id breaks ties, so paging cannot show one document twice and skip another
	// when several share a timestamp. The cursor comparison below is the same pair in
	// the same order, which is what makes it exact.
	oldestFirst := strings.EqualFold(strings.TrimSpace(filter.Order), ListOrderOldest)
	sort, after := "d.created_at DESC, d.id DESC", "(d.created_at, d.id) < ($7::timestamptz, $8::uuid)"
	if oldestFirst {
		sort, after = "d.created_at ASC, d.id ASC", "(d.created_at, d.id) > ($7::timestamptz, $8::uuid)"
	}
	// The arguments follow the SQL: with no cursor the comparison is not in the query,
	// so its parameters must not be passed either.
	cursor := "TRUE"
	args := []any{tenantID, status, docType, pattern, limit, offset}
	if filter.AfterID != "" {
		cursor = after
		args = append(args, filter.AfterAt, filter.AfterID)
	}
	rows, err := m.db.Query(ctx,
		`SELECT `+documentColumns+`
		   FROM document_records d
		  WHERE `+where+` AND `+cursor+`
		  ORDER BY `+sort+`
		  LIMIT $5 OFFSET $6`, args...)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		doc, err := m.scanDocument(rows)
		if err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		page.Documents = append(page.Documents, *doc)
	}
	return page, rows.Err()
}

// getDocument re-reads one document through the same column list as the list, so
// a write endpoint answers with exactly what a later refresh will show.
func (m *DocumentsModule) getDocument(ctx context.Context, tenantID, docID string) (*Document, error) {
	doc, err := m.scanDocument(m.db.QueryRow(ctx,
		`SELECT `+documentColumns+`
		   FROM document_records d
		  WHERE d.tenant_id = $1 AND d.id = $2`, tenantID, docID))
	if isNoRows(err) {
		return nil, ErrNotSignable
	}
	if err != nil {
		return nil, fmt.Errorf("read document: %w", err)
	}
	return doc, nil
}

// rowScanner is satisfied by both pgx.Row (QueryRow) and pgx.Rows (Query), so
// the column layout lives in exactly one place.
type rowScanner interface {
	Scan(dest ...any) error
}

func (m *DocumentsModule) scanDocument(row rowScanner) (*Document, error) {
	var doc Document
	err := row.Scan(
		&doc.ID, &doc.TenantID, &doc.Title, &doc.DocType, &doc.Status,
		&doc.SignedBy, &doc.SignatureHash, &doc.SignerRegNumber, &doc.SignerMethod,
		&doc.SignedAt, &doc.CreatedAt, &doc.SignatureCount, &doc.RequiredSignatures,
		&doc.OutstandingSteps,
	)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// actorFor names who did it in the audit log, preferring the email an
// administrator reading the log would recognise. Calls that arrive outside a
// request — a migration, a test — have no claims and are recorded as the system.
func actorFor(ctx context.Context) string {
	claims, err := auth.UserFromContext(ctx)
	if err != nil {
		return "system"
	}
	if claims.Email != "" {
		return claims.Email
	}
	return claims.UserID
}

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// isUniqueViolation reports a Postgres 23505, which is how a duplicate reaches Go.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// isConstraintViolation reports whether err is a violation of one NAMED constraint.
// Which constraint was hit decides what the caller is told: "you have already signed"
// is the truth for one of them and a lie for the others.
func isConstraintViolation(err error, name string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == name
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// writeWriteFailure sorts a failed write into the class its caller can act on:
// what they sent (400 with the reason), a collision with somebody else's change
// (409, retryable), or ours (500 with a fixed message, the driver's own text
// logged rather than returned). Answering everything with 400 and the wrapped
// error told operators their request was invalid when the database was down, and
// put connection strings and SQLSTATEs on screen.
func writeWriteFailure(ctx context.Context, w http.ResponseWriter, err error, whatFailed string) {
	switch {
	case errors.Is(err, ErrInvalidDocument), errors.Is(err, ErrInvalidConfiguration):
		writeError(w, http.StatusBadRequest, err.Error())
	case isUniqueViolation(err):
		writeError(w, http.StatusConflict, "this was changed by someone else at the same time — reload and try again")
	default:
		slog.ErrorContext(ctx, whatFailed, "error", err)
		writeError(w, http.StatusInternalServerError, whatFailed)
	}
}
