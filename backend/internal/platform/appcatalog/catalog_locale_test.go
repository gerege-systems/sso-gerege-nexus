package appcatalog_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/appcatalog"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/config"
)

// The shipped catalog must be able to answer in every language the API claims
// to support. A missing translation is not an error at runtime — Localized
// simply keeps the English default — so the App Store rendered English cards
// under a Mongolian sidebar and nobody was told why.
func TestShippedCatalogCoversEverySupportedLocale(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "catalog", "apps.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("catalog not readable from this working directory: %v", err)
	}

	var apps []appcatalog.CatalogApp
	if err := json.Unmarshal(raw, &apps); err != nil {
		t.Fatalf("catalog is not valid JSON: %v", err)
	}
	if len(apps) == 0 {
		t.Fatal("catalog is empty")
	}

	for _, app := range apps {
		for _, locale := range config.SupportedLocales {
			if locale == "en" {
				continue // en is the source text on the entry itself
			}
			text, ok := app.Translations[locale]
			if !ok {
				t.Errorf("%s: no %s translation", app.ID, locale)
				continue
			}
			if text.Name == "" || text.Description == "" || text.Category == "" {
				t.Errorf("%s/%s: incomplete translation (name=%q description=%q category=%q)",
					app.ID, locale, text.Name, text.Description, text.Category)
			}
		}
	}
}
