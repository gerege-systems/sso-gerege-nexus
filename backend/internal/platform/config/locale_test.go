package config_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/config"
)

// LocaleFromRequest answering a language it does not support is not an error —
// it returns the default, Mongolian. That is the right behaviour and it is also
// why the gap was invisible: a browser set to Chinese got Chinese body copy from
// the client dictionary and Mongolian menu labels from the server, on the same
// screen, with nothing logged. The languages the API claims to speak are
// therefore asserted rather than assumed.
func TestLocaleFromRequestHonoursEveryOfferedLanguage(t *testing.T) {
	tests := []struct {
		name, header, want string
	}{
		{"bare code", "zh", "zh"},
		{"region-qualified with quality values", "zh-CN,zh;q=0.9", "zh"},
		{"arabic", "ar", "ar"},
		{"french", "fr-FR,fr;q=0.9", "fr"},
		{"russian", "ru", "ru"},
		{"spanish", "es-ES,es;q=0.8", "es"},
		{"mongolian", "mn-MN,mn;q=0.9", "mn"},
		{"english", "en-US,en;q=0.9", "en"},
		{"first supported entry wins over a later one", "ko,ja;q=0.9,fr;q=0.8", "fr"},
		{"nothing offered falls back to the default", "ko", "mn"},
		{"absent header falls back to the default", "", "mn"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/v1/menus", nil)
			if tc.header != "" {
				r.Header.Set("Accept-Language", tc.header)
			}
			if got := config.LocaleFromRequest(r); got != tc.want {
				t.Errorf("Accept-Language %q → %q, want %q", tc.header, got, tc.want)
			}
		})
	}
}

// The `lang` query parameter overrides the header, so a link can carry a
// language without the recipient's browser settings deciding for them.
func TestLocaleFromRequestPrefersTheExplicitQueryParameter(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/menus?lang=es", nil)
	r.Header.Set("Accept-Language", "zh")
	if got := config.LocaleFromRequest(r); got != "es" {
		t.Errorf("lang=es with Accept-Language zh → %q, want es", got)
	}
}

// The default must remain Mongolian: it is the source language and the first
// entry is what every unsupported request resolves to.
func TestMongolianIsTheDefaultLocale(t *testing.T) {
	if config.SupportedLocales[0] != "mn" {
		t.Fatalf("default locale is %q, want mn", config.SupportedLocales[0])
	}
	for _, want := range []string{"mn", "ar", "zh", "en", "fr", "ru", "es"} {
		found := false
		for _, have := range config.SupportedLocales {
			if have == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s is offered by the client but not by the API", want)
		}
	}
}
