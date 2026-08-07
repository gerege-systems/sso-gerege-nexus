package menu

import (
	"os"
	"path/filepath"
	"testing"
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

// TestBlueprintLabelsAreTranslated keeps a menu entry from shipping with an
// English label in the Mongolian navigation.
func TestBlueprintLabelsAreTranslated(t *testing.T) {
	for appID, bp := range blueprints {
		for _, item := range append(append([]futureMenu{}, bp.Modules...), bp.Settings...) {
			if item.EN == "" || item.MN == "" {
				t.Errorf("%s menu %q is missing a label: en=%q mn=%q", appID, item.ID, item.EN, item.MN)
			}
			if item.Icon == "" {
				t.Errorf("%s menu %q has no icon", appID, item.ID)
			}
		}
	}
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
