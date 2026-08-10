/*
 * Gerege SSO
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 */

// Package httpx holds the two ways this platform answers an HTTP request.
//
// It exists because every app module had grown its own copy: five identical
// writeJSON/writeError pairs across platform, documents, esign, gov_services
// and billing, plus ninety-odd handlers that skipped the helper and passed a
// hand-written JSON string to http.Error. Those last ones all answered with
// Content-Type: text/plain, because that is what http.Error sets — a browser
// was told "this is text" and handed JSON on every single error path.
//
// It sits below both platform and apps on purpose. platform imports apps to
// mount their routes, so a helper living in platform is unreachable from an app
// module; this package imports neither.
package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// JSON writes value as the whole response body.
//
// An encoding failure is logged rather than returned: the status line and
// headers are already on the wire by the time Encode runs, so there is no
// second answer left to give and a caller could do nothing with the error.
func JSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("failed to encode JSON response", "status", status, "error", err)
	}
}

// Error answers with {"error": message}.
//
// message is encoded rather than interpolated. Handlers used to build the body
// by concatenating a permission code or a failure reason into a JSON literal,
// which produced invalid JSON the moment the value contained a quote, a
// backslash or a newline — and the client saw a parse error instead of the
// reason it was refused.
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, map[string]string{"error": message})
}
