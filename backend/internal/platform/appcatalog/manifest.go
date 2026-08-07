package appcatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Masterminds/semver/v3"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal"
)

type Manifest struct {
	ID           string                          `json:"id"`
	Name         string                          `json:"name"`
	Version      string                          `json:"version"`
	Platform     string                          `json:"platform"`
	Dependencies []internal.Dependency           `json:"dependencies"`
	Permissions  []internal.PermissionDefinition `json:"permissions"`
	Menus        []internal.MenuDefinition       `json:"menus"`
}

type CatalogApp struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	IconURL     string   `json:"icon_url"`
	Category    string   `json:"category"`
	Visibility  string   `json:"visibility"`
	Version     string   `json:"version"`
	Manifest    Manifest `json:"manifest"`

	// Translations holds per-locale overrides keyed by ISO 639-1 code. The
	// store API resolves them before responding, so clients never have to
	// translate catalog content themselves.
	Translations map[string]CatalogAppText `json:"translations,omitempty"`
}

// CatalogAppText is the translatable part of a catalog entry.
type CatalogAppText struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
}

// Localized returns a copy with any translation for the locale applied. Fields
// missing from the translation keep their default value.
func (a CatalogApp) Localized(locale string) CatalogApp {
	text, ok := a.Translations[locale]
	if !ok {
		return a
	}
	if text.Name != "" {
		a.Name = text.Name
	}
	if text.Description != "" {
		a.Description = text.Description
	}
	if text.Category != "" {
		a.Category = text.Category
	}
	return a
}

// ValidateManifest validates semver and manifest rules.
func ValidateManifest(m Manifest, platformVersion string) error {
	if m.ID == "" || m.Name == "" || m.Version == "" {
		return fmt.Errorf("invalid manifest: id, name, and version are required")
	}

	_, err := semver.NewVersion(m.Version)
	if err != nil {
		return fmt.Errorf("invalid app version semver %q: %w", m.Version, err)
	}

	if m.Platform != "" && platformVersion != "" {
		constraint, err := semver.NewConstraint(m.Platform)
		if err != nil {
			return fmt.Errorf("invalid platform constraint %q: %w", m.Platform, err)
		}
		platVer, err := semver.NewVersion(platformVersion)
		if err != nil {
			return fmt.Errorf("invalid platform version %q: %w", platformVersion, err)
		}
		if !constraint.Check(platVer) {
			return fmt.Errorf("app %s version %s requires platform %s, current is %s", m.ID, m.Version, m.Platform, platformVersion)
		}
	}

	for _, dep := range m.Dependencies {
		if dep.ID == "" {
			return fmt.Errorf("dependency ID cannot be empty in app %s", m.ID)
		}
		if dep.VersionConstraint != "" {
			if _, err := semver.NewConstraint(dep.VersionConstraint); err != nil {
				return fmt.Errorf("invalid dependency constraint %q for dep %s in app %s: %w", dep.VersionConstraint, dep.ID, m.ID, err)
			}
		}
	}
	return nil
}

// LoadManifestFile loads and validates a manifest file.
func LoadManifestFile(path string, platformVersion string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest file %s: %w", path, err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("unmarshal manifest JSON: %w", err)
	}
	if err := ValidateManifest(m, platformVersion); err != nil {
		return Manifest{}, fmt.Errorf("validate manifest %s: %w", path, err)
	}
	return m, nil
}

// Interfaces for future marketplace / remote registry boundary:
type CatalogRepository interface {
	GetAppBySlug(ctx context.Context, slug string) (CatalogApp, error)
	ListApps(ctx context.Context) ([]CatalogApp, error)
}

type PackageStorage interface {
	FetchPackage(ctx context.Context, packageURL string) ([]byte, error)
}

type PackageVerifier interface {
	VerifyChecksum(data []byte, expectedSHA256 string) error
	VerifySignature(data []byte, signature string) error
}

type Installer interface {
	InstallApp(ctx context.Context, tenantID, appSlug string) error
	DisableApp(ctx context.Context, tenantID, appSlug string) error
	EnableApp(ctx context.Context, tenantID, appSlug string) error
}
