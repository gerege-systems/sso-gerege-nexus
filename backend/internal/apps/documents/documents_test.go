package documents

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"
	"time"
	"unicode/utf8"
)

// The HTTP handlers turn these error classes into 409, 404, 400 and 500, so the
// class a failure lands in decides whether the caller is told to fix something or
// told the server broke. A nil pool is safe here: every case is settled before a
// query, and before any citizen is troubled.
func TestSigningRefusesAMalformedID(t *testing.T) {
	ctx := context.Background()
	module := &DocumentsModule{}

	if _, err := module.SignWithDAN(ctx, "tenant", "not-a-uuid", "AA90010111", "123456"); !errors.Is(err, ErrNotSignable) {
		t.Errorf("dan: got %v, want ErrNotSignable", err)
	}
	if _, err := module.StartEIDSignature(ctx, "tenant", "not-a-uuid", "AA90010111"); !errors.Is(err, ErrNotSignable) {
		t.Errorf("eid start: got %v, want ErrNotSignable", err)
	}
	if _, err := module.PollEIDSignature(ctx, "tenant", "not-a-uuid", "some-session"); !errors.Is(err, ErrSignSessionUnknown) {
		t.Errorf("eid poll: got %v, want ErrSignSessionUnknown", err)
	}
	if _, err := module.RejectDocument(ctx, "tenant", "not-a-uuid"); !errors.Is(err, ErrNotSignable) {
		t.Errorf("reject: got %v, want ErrNotSignable", err)
	}
	if _, err := module.RouteDocument(ctx, "tenant", "not-a-uuid"); !errors.Is(err, ErrNotRoutable) {
		t.Errorf("route: got %v, want ErrNotRoutable", err)
	}
	if _, err := module.ListSignatures(ctx, "tenant", "not-a-uuid"); !errors.Is(err, ErrNotSignable) {
		t.Errorf("signatures: got %v, want ErrNotSignable", err)
	}
	if err := module.DeleteTemplate(ctx, "tenant", "not-a-uuid"); !errors.Is(err, ErrTemplateNotFound) {
		t.Errorf("delete template: got %v, want ErrTemplateNotFound", err)
	}
	if _, err := module.CreateDocumentFromTemplate(ctx, "tenant", "not-a-uuid"); !errors.Is(err, ErrTemplateNotFound) {
		t.Errorf("use template: got %v, want ErrTemplateNotFound", err)
	}
}

// What the citizen reads on their own device is the only thing telling them what
// they are approving, so the words saying a signature is being given have to
// survive — and they only survive if we cut the text ourselves. eID's field is
// displayText60 and the core client slices it to 60 BYTES, which for Cyrillic is
// about 30 letters and can land mid-letter.
func TestSignatureDisplayText(t *testing.T) {
	short := signatureDisplayText("Гэрээ 2026")
	if !strings.Contains(short, "Гэрээ 2026") {
		t.Errorf("got %q, want a title that fits to be shown whole", short)
	}
	if !strings.Contains(short, "Гарын үсэг") {
		t.Errorf("got %q, want it to say a signature is being asked for", short)
	}

	// The case that used to break: a long Cyrillic title. The purpose has to
	// survive it, because the purpose is the whole point of the prompt.
	long := signatureDisplayText("Улаанбаатар хотын Захирагчийн ажлын албатай хамтран ажиллах гурван талт гэрээ, 2026 оны 1 дүгээр сарын 15-ны өдөр")
	if !strings.Contains(long, "Гарын үсэг") {
		t.Errorf("got %q, want the purpose to survive a long title", long)
	}
	if !strings.HasSuffix(long, "…") {
		t.Errorf("got %q, want a cut title marked as cut", long)
	}

	for name, text := range map[string]string{"short": short, "long": long} {
		if len(text) > eidDisplayTextBytes {
			t.Errorf("%s: %d bytes, want at most %d — core would slice it mid-letter",
				name, len(text), eidDisplayTextBytes)
		}
		if !utf8.ValidString(text) {
			t.Errorf("%s: the cut left invalid UTF-8 behind: %q", name, text)
		}
	}

	// A document with no title still has to say what is being approved.
	if got := signatureDisplayText("   "); !strings.Contains(got, "гарын үсэг") {
		t.Errorf("got %q, want an empty title to still state the purpose", got)
	}
}

func TestSignaturePolicyDefaultsToBothChannels(t *testing.T) {
	policy := defaultSignaturePolicy("CONTRACT")

	if !policy.allows(SignerEID) || !policy.allows(SignerDAN) {
		t.Error("an unconfigured type must accept both national channels, as it did before policies existed")
	}
	if policy.RequireNamedSigner {
		t.Error("an unconfigured type must not require a named signer")
	}
	if policy.Configured {
		t.Error("a default policy is not a stored one")
	}
	if policy.allows("SMARTCARD") {
		t.Error("a channel the module does not speak must never be allowed")
	}

	only := SignaturePolicy{DocType: "CONTRACT", AllowEID: true}
	if only.allows(SignerDAN) {
		t.Error("DAN must be refused when the policy allows E-ID only")
	}
}

// A signature is held to whoever the document's next step names, and held back
// from anyone the chain still needs further along.
func TestCheckSigner(t *testing.T) {
	if err := checkSigner(nil, "CONTRACT", "AA90010111"); err != nil {
		t.Errorf("a document with no chain must accept any authorised signer: %v", err)
	}
	if err := checkSigner(&approvalPosition{}, "CONTRACT", "AA90010111"); err != nil {
		t.Errorf("a position with no next step must accept any authorised signer: %v", err)
	}

	open := &approvalPosition{Next: &ApprovalStep{Order: 1, Name: "Хэн ч"}}
	if err := checkSigner(open, "CONTRACT", "AA90010111"); err != nil {
		t.Errorf("an open step must accept any authorised signer: %v", err)
	}

	named := &approvalPosition{Next: &ApprovalStep{Order: 2, Name: "Захирал", SignerRegNumber: "CC90010111"}}
	if err := checkSigner(named, "CONTRACT", "CC90010111"); err != nil {
		t.Errorf("the named signer must be accepted: %v", err)
	}
	err := checkSigner(named, "CONTRACT", "AA90010111")
	if !errors.Is(err, ErrSignatureRejected) {
		t.Fatalf("got %v, want ErrSignatureRejected", err)
	}
	for _, want := range []string{"AA90010111", "CC90010111", "Захирал"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("got %q, want it to mention %q so an operator can tell why", err, want)
		}
	}

	// An open step is open — but not to somebody a later step names. Spending their
	// one signature here would leave their own step unfillable for ever.
	reserved := &approvalPosition{
		Next:     &ApprovalStep{Order: 1, Name: "Хянагч"},
		Reserved: []string{"CC90010111"},
	}
	if err := checkSigner(reserved, "CONTRACT", "AA90010111"); err != nil {
		t.Errorf("anyone else may still take the open step: %v", err)
	}
	err = checkSigner(reserved, "CONTRACT", "CC90010111")
	if !errors.Is(err, ErrSignatureRejected) {
		t.Fatalf("a citizen a later step names must be held back: got %v", err)
	}
	if !strings.Contains(err.Error(), "CC90010111") {
		t.Errorf("got %q, want it to name the signer being held back", err)
	}
}

// A chain that could never be completed under a named-signer policy must be
// refused when it is saved, not discovered when a document sticks.
func TestStepsCanRequireNamedSigners(t *testing.T) {
	ok := []WorkflowStep{
		{Order: 1, Name: "Дарга", SignerRegNumber: "AA90010111"},
		{Order: 2, Name: "Захирал", SignerRegNumber: "BB90010111"},
	}
	if err := stepsCanRequireNamedSigners("CONTRACT", ok); err != nil {
		t.Errorf("a chain naming a distinct signer per step is fine: %v", err)
	}

	for name, steps := range map[string][]WorkflowStep{
		"no steps at all": {},
		"an open step nobody could fill": {
			{Order: 1, Name: "Дарга", SignerRegNumber: "AA90010111"},
			{Order: 2, Name: "Хэн ч"},
		},
		"one citizen named twice, who signs once": {
			{Order: 1, Name: "Дарга", SignerRegNumber: "AA90010111"},
			{Order: 2, Name: "Дарга дахин", SignerRegNumber: "AA90010111"},
		},
	} {
		if err := stepsCanRequireNamedSigners("CONTRACT", steps); !errors.Is(err, ErrInvalidConfiguration) {
			t.Errorf("%s: got %v, want ErrInvalidConfiguration", name, err)
		}
	}
}

// A registration number this module can see is wrong is refused here, so that
// anything the provider then refuses can be treated as the provider's trouble —
// which a polling client must retry rather than give up on.
func TestAShortRegistrationNumberIsRefusedLocally(t *testing.T) {
	module := &DocumentsModule{}
	// A nil pool is safe: this refusal comes before any query, and before any
	// provider call.
	_, err := module.StartEIDSignature(context.Background(), "tenant",
		"3f1b9c62-2f1a-4a1c-9d3e-8b7a5c4e1d20", "AA1")
	if !errors.Is(err, ErrSignatureRejected) {
		t.Fatalf("got %v, want ErrSignatureRejected", err)
	}
	if errors.Is(err, ErrProviderUnavailable) {
		t.Error("a short registration number is not provider trouble")
	}
	if !strings.Contains(err.Error(), "AA1") {
		t.Errorf("got %q, want it to quote what was sent", err)
	}
}

// A Mongolian registration number is Cyrillic — "УБ99010111" is ten characters in
// twenty bytes — and every bound on it is a bound on CHARACTERS: the column counts
// characters, the SQL that repairs stored chains counts characters, and a citizen
// reading their own number counts characters. Measuring bytes in Go made the three
// places that enforce "a named step must be fillable" disagree: "УБ9901" is eight
// bytes but six characters, so the save accepted it as a named step and the snapshot
// then opened it, leaving the workflows screen naming a citizen the document's own
// chain did not.
func TestARegistrationNumberIsBoundedInCharactersNotBytes(t *testing.T) {
	for _, tc := range []struct {
		reg       string
		plausible bool
		why       string
	}{
		{"AA90010111", true, "the transliterated form the mock uses"},
		{"УБ99010111", true, "the real Cyrillic form, 10 characters in 20 bytes"},
		{"уб99010111", true, "the same, normalised on the way in"},
		{"УБ9901", false, "6 characters — 8 bytes, which once passed"},
		{"AA1", false, "3 characters"},
		{"", false, "an open step names nobody"},
		{strings.Repeat("У", RegNumberMax), true, "64 characters is what the column holds"},
		{strings.Repeat("У", RegNumberMax+1), false, "65 would not survive being stored"},
	} {
		if got := plausibleRegNumber(normaliseRegNumber(tc.reg)); got != tc.plausible {
			t.Errorf("plausibleRegNumber(%q) = %v, want %v — %s", tc.reg, got, tc.plausible, tc.why)
		}
	}

	// And the shortest Cyrillic number that passes must be exactly the limit in
	// characters, not in bytes.
	if !plausibleRegNumber(strings.Repeat("Ү", RegNumberLimit)) {
		t.Error("a number of exactly the limit in characters must be accepted")
	}
	if plausibleRegNumber(strings.Repeat("Ү", RegNumberLimit-1)) {
		t.Error("a number one character short must be refused")
	}
}

// The same rule, applied where a document's chain is decided: what cannot be saved
// must not be copied, and what can be saved must be copied intact.
func TestFillableChainOpensOnlyWhatNobodyCouldFill(t *testing.T) {
	got := fillableChain([]WorkflowStep{
		{Order: 1, Name: "Ня-бо", SignerRegNumber: "  уб99010111 "}, // normalised, kept
		{Order: 2, Name: "Дахилт", SignerRegNumber: "УБ99010111"},   // the same citizen
		{Order: 3, Name: "Алдаа", SignerRegNumber: "УБ9901"},        // 6 characters
		{Order: 4, Name: "Хэн ч", SignerRegNumber: ""},              // open already
		{Order: 5, Name: "Захирал", SignerRegNumber: "cc90010111"},  // fine, normalised
	})
	want := []string{"УБ99010111", "", "", "", "CC90010111"}
	if len(got) != len(want) {
		t.Fatalf("got %d steps, want %d — the tenant asked for that many approvals", len(got), len(want))
	}
	for i, step := range got {
		if step.SignerRegNumber != want[i] {
			t.Errorf("step %d = %q, want %q", i+1, step.SignerRegNumber, want[i])
		}
		if step.Order != i+1 {
			t.Errorf("step %d carries order %d", i+1, step.Order)
		}
	}
}

func TestResolveTitlePattern(t *testing.T) {
	at := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)

	if got, want := resolveTitlePattern("Гэрээ {year}", at), "Гэрээ 2026"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := resolveTitlePattern("{date} · {month}", at), "2026-08-06 · 08"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// An unknown token is left in place rather than silently dropped, so a typo
	// shows up in the document instead of disappearing.
	if got, want := resolveTitlePattern("Гэрээ {quarter}", at), "Гэрээ {quarter}"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The retention screen keeps the counts it already has when the server sends none,
// so "could not count" has to reach it as absence rather than as zero. A type with
// hundreds of documents reading "0 filed" under a green banner is the failure this
// guards against, and it is a wire-format promise the frontend depends on.
func TestUnknownRetentionCountsAreAbsentFromTheJSON(t *testing.T) {
	uncounted, err := json.Marshal(RetentionRule{DocType: "CONTRACT", RetainYears: 5, Configured: true})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(uncounted), "expired") || strings.Contains(string(uncounted), "total") {
		t.Errorf("uncounted rule = %s, want neither count — zero would be read as a fact", uncounted)
	}

	expired, total := 1, 2
	counted, err := json.Marshal(RetentionRule{
		DocType: "CONTRACT", RetainYears: 5, Configured: true,
		Expired: &expired, Total: &total,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"expired":1`, `"total":2`} {
		if !strings.Contains(string(counted), want) {
			t.Errorf("counted rule = %s, want it to carry %s", counted, want)
		}
	}

	// And a genuine zero is still stated, or a type with nothing filed under it
	// would look like one the server failed to count.
	zero := 0
	none, err := json.Marshal(RetentionRule{DocType: "INVOICE", Expired: &zero, Total: &zero})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(none), `"expired":0`) || !strings.Contains(string(none), `"total":0`) {
		t.Errorf("empty type = %s, want both zeroes stated", none)
	}
}

// A request body is read before anything can judge it, so the judging has to come
// first. Measured before this bound existed: a 143 MB approval chain took the API's
// resident memory from 86 MB to 444 MB in six tenths of a second and was then correctly
// refused for having more than ten steps — the refusal was right and far too late.
func TestAnOversizeBodyIsRefusedBeforeItIsRead(t *testing.T) {
	var reached bool
	handler := limitBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		body, err := io.ReadAll(r.Body)
		if err != nil {
			// What a chunked over-size body looks like from inside the handler.
			http.Error(w, "too big", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))

	// Declared over-size: refused without the handler ever running.
	big := httptest.NewRequest(http.MethodPut, "/documents/workflows/CONTRACT",
		strings.NewReader(strings.Repeat("x", bodyLimit+1)))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, big)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("declared over-size: got %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
	if reached {
		t.Error("the handler read a body that was already known to be too large")
	}

	// Undeclared length (chunked), over-size: the read itself is capped.
	reached = false
	chunked := httptest.NewRequest(http.MethodPut, "/documents/workflows/CONTRACT",
		iotest.OneByteReader(strings.NewReader(strings.Repeat("y", bodyLimit+100))))
	chunked.ContentLength = -1
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, chunked)
	if !reached {
		t.Error("a chunked request must reach the handler; the cap is on the read")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("chunked over-size: got %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}

	// And a real one goes through untouched.
	reached = false
	ok := httptest.NewRequest(http.MethodPut, "/documents/workflows/CONTRACT",
		strings.NewReader(`{"steps":[{"name":"Ня-бо","signer_reg_number":"УБ99010111"}]}`))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, ok)
	if !reached || rec.Code != http.StatusOK {
		t.Errorf("an ordinary request: reached=%v code=%d", reached, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "УБ99010111") {
		t.Errorf("the body did not arrive intact: %q", rec.Body.String())
	}
}
