package security

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/httpx"
)

// CSRFMiddleware rejects cross-site state-changing requests authenticated by
// the session cookie.
//
// Only the cookie is at risk. A cookie is ambient authority — the browser
// attaches it to a request the page did not intend to make — and that is the
// whole of what this defends. A client holding a bearer token has to put it on
// the request itself, so it is never sent by accident and never reaches here.
// That is the supported path for the desktop clients: they authenticate with
// Authorization: Bearer and are unaffected by everything below.
//
// The rule is that a cookie-authenticated write must carry positive evidence
// that a page of ours made it. The previous version asked the opposite — it
// refused what looked cross-site and allowed everything else — so a request
// with neither Origin nor Sec-Fetch-Site went through, and so did Sec-Fetch-Site:
// same-site, which on this deployment means a different product under
// gerege.mn rather than a different page of this one.
func CSRFMiddleware(next http.Handler) http.Handler {
	allowed := make(map[string]struct{})
	for _, raw := range SafeCORSOrigins() {
		if u, err := url.Parse(raw); err == nil && u.Scheme != "" && u.Host != "" {
			allowed[u.Scheme+"://"+u.Host] = struct{}{}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		if _, err := r.Cookie(auth.SessionCookieName); err != nil {
			next.ServeHTTP(w, r)
			return
		}

		// Sec-Fetch-Site is sent by every browser released in the last several
		// years and cannot be forged by the page making the request. Two of its
		// values settle the question on their own.
		switch strings.ToLower(r.Header.Get("Sec-Fetch-Site")) {
		case "same-origin":
			// Our own page.
			next.ServeHTTP(w, r)
			return
		case "none":
			// The person typed the address or used a bookmark. A page on
			// another site cannot produce this.
			next.ServeHTTP(w, r)
			return
		}

		// Everything else — same-site, cross-site, or no header at all — is
		// decided by the Origin allowlist, which is the same list CORS already
		// answers with. Deferring to it rather than judging the header keeps
		// one policy instead of two that can disagree.
		//
		// It also keeps same-site honest in both directions. In production the
		// frontend and the API share an origin, so a same-site write comes from
		// one of the neighbouring products under gerege.mn and its Origin is not
		// on the list. In development they are the same host on different ports,
		// which is also same-site, and there the Origin is localhost:3000 and is.
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil {
				forbid(w, "invalid origin")
				return
			}
			if _, ok := allowed[u.Scheme+"://"+u.Host]; !ok {
				forbid(w, "origin not allowed")
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		// Neither signal. This used to be allowed, which meant the defence could
		// be skipped by not participating in it. A browser always sends one of
		// them on a cookie-carrying write; something that sends neither is not a
		// browser, and a client that is not a browser has no reason to rely on a
		// cookie — it can hold the same token and present it as
		// Authorization: Bearer, which does not come through here at all.
		forbid(w, "a cookie-authenticated request must carry Origin or Sec-Fetch-Site; "+
			"clients that are not browsers should authenticate with Authorization: Bearer")
	})
}

func forbid(w http.ResponseWriter, reason string) {
	httpx.Error(w, http.StatusForbidden, "forbidden: "+reason)
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}
