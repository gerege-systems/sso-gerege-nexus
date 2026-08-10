// Package config centralises how the platform reads environment-driven
// runtime switches.
package config

import (
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
)

// MockEnabled reports whether a national-integration connector (E-ID, DAN,
// XYP) should serve canned identities instead of calling the live gateway.
//
// The previous rule was `os.Getenv(name) != "false"`, i.e. mock mode was ON by
// default. In production that turns the E-ID and DAN login endpoints into an
// unauthenticated door: any registration number returns a "verified" citizen.
// Mock mode is therefore never implicit in production — it must be requested.
func MockEnabled(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "true", "1", "yes":
		if IsProduction() {
			slog.Warn("mock integration mode explicitly enabled in production", "flag", name)
		}
		return true
	case "false", "0", "no":
		return false
	}
	// Unset: convenient for local development, refused in production.
	if IsProduction() {
		slog.Info("mock integration mode disabled (production default)", "flag", name)
		return false
	}
	return true
}

// IsProduction reports whether ENVIRONMENT=production.
func IsProduction() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("ENVIRONMENT")), "production")
}

// SupportedLocales lists the languages the API can answer in. The first entry
// is the default.
//
// This must stay in step with the client's LOCALES list (frontend/lib/i18n).
// While it held only mn and en, a browser asking for any of the other five got
// SupportedLocales[0] — Mongolian — for everything the server owns. The result
// was a single screen in three languages at once: menu labels in Mongolian
// because the server fell back, body copy in the language the user actually
// picked because the client had it, and English wherever neither did.
var SupportedLocales = []string{"mn", "ar", "zh", "en", "fr", "ru", "es"}

// LocaleFromRequest resolves the caller's language from the `lang` query
// parameter, falling back to Accept-Language and then to the default locale.
// Server-owned content (menu labels, catalog copy) is translated with it, so a
// client never has to ship translations for data it does not own.
func LocaleFromRequest(r *http.Request) string {
	if lang := normalizeLocale(r.URL.Query().Get("lang")); lang != "" {
		return lang
	}

	// Accept-Language: mn-MN,mn;q=0.9,en;q=0.8
	for _, part := range strings.Split(r.Header.Get("Accept-Language"), ",") {
		tag, _, _ := strings.Cut(part, ";")
		if lang := normalizeLocale(tag); lang != "" {
			return lang
		}
	}

	return SupportedLocales[0]
}

func normalizeLocale(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	base, _, _ := strings.Cut(tag, "-")
	if slices.Contains(SupportedLocales, base) {
		return base
	}
	return ""
}
