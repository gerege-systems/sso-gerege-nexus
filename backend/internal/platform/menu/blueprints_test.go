package menu

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/config"
)

// TestBlueprintEntriesHaveRealPages is the inverse of the rule this file used
// to enforce.
//
// The old assertion was that every app declares at least two module and three
// settings entries — a shape rule that filled the sidebar whether or not the
// screens existed, and thirty of them did not. What matters is the opposite:
// nothing may appear in the navigation unless there is something behind it.
//
// The check reaches into the frontend on purpose. The menu is declared in Go
// and rendered from Next.js pages, so the drift this catches is exactly the
// kind that no single-language test can see.
func TestBlueprintEntriesHaveRealPages(t *testing.T) {
	root := repoRoot(t)

	for appID, bp := range blueprints {
		if bp.Slug == "" {
			t.Errorf("%s has an empty route slug", appID)
			continue
		}
		for _, item := range append(append([]futureMenu{}, bp.Modules...), bp.Settings...) {
			page := filepath.Join(root, "frontend", "app", "module", bp.Slug, item.ID, "page.tsx")
			if _, err := os.Stat(page); err != nil {
				t.Errorf("%s declares menu %q at /module/%s/%s but %s does not exist;"+
					" remove the entry or build the screen",
					appID, item.EN, bp.Slug, item.ID, filepath.Join("frontend", "app", "module", bp.Slug, item.ID, "page.tsx"))
			}
		}
	}
}

// A menu label missing a locale does not fail, it falls back to English — which
// is why the gap went unnoticed until a screen showed three languages at once.
// Coverage is therefore asserted rather than left to be noticed.
func TestEveryMenuLabelCoversEverySupportedLocale(t *testing.T) {
	check := func(name, en string, labels map[string]string) {
		t.Helper()
		if en == "" {
			t.Errorf("%s: empty English label", name)
		}
		for _, locale := range config.SupportedLocales {
			if locale == "en" {
				continue // en is the fallback and lives in the Label field
			}
			if labels[locale] == "" {
				t.Errorf("%s: no %s translation", name, locale)
			}
		}
	}

	for appID, bp := range blueprints {
		for _, item := range append(append([]futureMenu{}, bp.Modules...), bp.Settings...) {
			check(appID+"/"+item.ID, item.EN, item.Labels)
			if item.Icon == "" {
				t.Errorf("%s menu %q has no icon", appID, item.ID)
			}
		}
	}
	check("group/modules", "Modules", groupModules)
	check("group/settings", "Settings", groupSettings)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cwd: %v", err)
	}
	// Walk up until the directory holding both halves of the repository.
	for range 8 {
		if _, err := os.Stat(filepath.Join(dir, "frontend", "app")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Skip("frontend tree not found next to the backend module; skipping the page-existence check")
	return ""
}
