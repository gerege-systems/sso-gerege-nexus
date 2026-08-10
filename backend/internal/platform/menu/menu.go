package menu

import (
	"context"
	"fmt"
	"sort"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/appregistry"
)

type InstalledAppStore interface {
	GetEnabledAppIDsForTenant(context.Context, string) ([]string, error)
}

func GetTenantMenus(ctx context.Context, store InstalledAppStore, tenantID, locale string) ([]internal.MenuDefinition, error) {
	enabledIDs, err := store.GetEnabledAppIDsForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	enabled := map[string]bool{}
	for _, id := range enabledIDs {
		enabled[id] = true
	}
	menus := make([]internal.MenuDefinition, 0)
	for _, mod := range appregistry.List() {
		if !enabled[mod.ID()] {
			continue
		}
		bp, ok := blueprints[mod.ID()]
		if !ok {
			continue
		}
		modulesID, settingsID := bp.Slug+"_modules", bp.Slug+"_settings"
		// The Modules group always has the app's own working screen in it.
		menus = append(menus,
			localized(internal.MenuDefinition{ID: modulesID, AppID: mod.ID(), AppName: mod.Name(), Label: "Modules", Icon: "boxes", Order: 10, Labels: groupModules}, locale))
		// Settings is only a heading over its entries, so an app with none —
		// products, billing — must not render an empty one.
		if len(bp.Settings) > 0 {
			menus = append(menus,
				localized(internal.MenuDefinition{ID: settingsID, AppID: mod.ID(), AppName: mod.Name(), Label: "Settings", Icon: "settings", Order: 20, Labels: groupSettings}, locale))
		}
		for _, item := range mod.Menus() {
			item.AppID, item.AppName, item.ParentID, item.Order = mod.ID(), mod.Name(), modulesID, 10
			menus = append(menus, localized(item, locale))
		}
		for i, item := range bp.Modules {
			menus = append(menus, futureDefinition(mod.ID(), mod.Name(), modulesID, bp.Slug, item, 20+i*10, locale))
		}
		for i, item := range bp.Settings {
			menus = append(menus, futureDefinition(mod.ID(), mod.Name(), settingsID, bp.Slug, item, 10+i*10, locale))
		}
	}
	sort.Slice(menus, func(i, j int) bool {
		if menus[i].AppID != menus[j].AppID {
			return menus[i].AppID < menus[j].AppID
		}
		if menus[i].ParentID != menus[j].ParentID {
			return menus[i].ParentID < menus[j].ParentID
		}
		return menus[i].Order < menus[j].Order
	})
	return menus, nil
}

func localized(item internal.MenuDefinition, locale string) internal.MenuDefinition {
	item.Label = item.LocalizedLabel(locale)
	return item
}
func futureDefinition(appID, appName, parent, slug string, item futureMenu, order int, locale string) internal.MenuDefinition {
	// Resolving through LocalizedLabel rather than an if/else on "mn" is what
	// lets a blueprint entry answer in all seven languages: an unknown locale
	// falls back to EN instead of silently returning Mongolian.
	return localized(internal.MenuDefinition{
		ID:       fmt.Sprintf("%s_%s", slug, item.ID),
		AppID:    appID,
		AppName:  appName,
		ParentID: parent,
		Label:    item.EN,
		Path:     fmt.Sprintf("/module/%s/%s", slug, item.ID),
		Icon:     item.Icon,
		Order:    order,
		Labels:   item.Labels,
	}, locale)
}
