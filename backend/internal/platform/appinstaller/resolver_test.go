package appinstaller_test

import (
	"testing"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/appcatalog"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/appinstaller"
)

func TestDependencyGraph_ResolutionAndCycleDetection(t *testing.T) {
	contacts := appcatalog.Manifest{
		ID:           "io.example.contacts",
		Name:         "Contacts",
		Version:      "1.0.0",
		Dependencies: nil,
	}

	products := appcatalog.Manifest{
		ID:           "io.example.products",
		Name:         "Products",
		Version:      "1.0.0",
		Dependencies: nil,
	}

	inventory := appcatalog.Manifest{
		ID:      "io.example.inventory",
		Name:    "Inventory",
		Version: "1.0.0",
		Dependencies: []internal.Dependency{
			{ID: "io.example.contacts", VersionConstraint: "^1.0.0"},
			{ID: "io.example.products", VersionConstraint: "^1.0.0"},
		},
	}

	t.Run("Happy path: Inventory resolves Contacts and Products first", func(t *testing.T) {
		g := appinstaller.NewDependencyGraph([]appcatalog.Manifest{contacts, products, inventory})
		order, err := g.ResolveInstallOrder("io.example.inventory")
		if err != nil {
			t.Fatalf("expected resolution to succeed, got: %v", err)
		}
		if len(order) != 3 {
			t.Fatalf("expected 3 apps, got %d", len(order))
		}
		if order[len(order)-1] != "io.example.inventory" {
			t.Errorf("inventory must be last, got %v", order)
		}
	})

	t.Run("Missing dependency fails resolution", func(t *testing.T) {
		g := appinstaller.NewDependencyGraph([]appcatalog.Manifest{inventory}) // missing contacts & products
		_, err := g.ResolveInstallOrder("io.example.inventory")
		if err == nil {
			t.Fatal("expected error due to missing dependencies, got nil")
		}
	})

	t.Run("Cycle detection fails resolution", func(t *testing.T) {
		appA := appcatalog.Manifest{
			ID:           "app.a",
			Dependencies: []internal.Dependency{{ID: "app.b"}},
		}
		appB := appcatalog.Manifest{
			ID:           "app.b",
			Dependencies: []internal.Dependency{{ID: "app.a"}},
		}
		g := appinstaller.NewDependencyGraph([]appcatalog.Manifest{appA, appB})
		_, err := g.ResolveInstallOrder("app.a")
		if err == nil {
			t.Fatal("expected cycle detection error, got nil")
		}
	})
}
