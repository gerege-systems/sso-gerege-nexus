package appcatalog_test

import (
	"testing"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/appcatalog"
)

func TestValidateManifest(t *testing.T) {
	validManifest := appcatalog.Manifest{
		ID:       "io.example.test",
		Name:     "Test App",
		Version:  "1.0.0",
		Platform: ">=0.1.0 <2.0.0",
	}

	t.Run("Valid manifest passes", func(t *testing.T) {
		err := appcatalog.ValidateManifest(validManifest, "1.0.0")
		if err != nil {
			t.Fatalf("expected valid manifest to pass, got: %v", err)
		}
	})

	t.Run("Invalid semver fails", func(t *testing.T) {
		invalid := validManifest
		invalid.Version = "invalid-semver"
		err := appcatalog.ValidateManifest(invalid, "1.0.0")
		if err == nil {
			t.Fatal("expected invalid semver to fail validation")
		}
	})

	t.Run("Incompatible platform constraint fails", func(t *testing.T) {
		incompatible := validManifest
		incompatible.Platform = ">=2.0.0"
		err := appcatalog.ValidateManifest(incompatible, "1.0.0")
		if err == nil {
			t.Fatal("expected platform constraint incompatibility to fail validation")
		}
	})
}
