package menu

import (
	"context"
	"fmt"
	"sort"

	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/platform/appregistry"
)

type InstalledAppStore interface {
	GetEnabledAppIDsForTenant(context.Context, string) ([]string, error)
}

type futureMenu struct{ ID, EN, MN, Icon string }
type blueprint struct {
	Slug              string
	Modules, Settings []futureMenu
}

// blueprints declares the extra menu entries an installed app contributes.
//
// Every entry here must have a real page at /module/<slug>/<id>; a screen that
// only announces itself is worse than a missing menu item, because it costs the
// reader a click to learn nothing. TestBlueprintEntriesHaveRealPages walks this
// map against the frontend and fails when the two drift apart.
//
// This map used to carry five speculative entries per app — price lists,
// retention rules, HSM connections, valuation methods and so on — none of which
// had a table, an endpoint or a page behind them. What survived is what the
// data supports today. Restore an entry when its feature exists, not before.
var blueprints = map[string]blueprint{
	// Segments, duplicates and the CSV import all read or write the contacts
	// table that is already there.
	"io.example.contacts": {"contacts", []futureMenu{{"segments", "Segments", "Сегментүүд", "users"}, {"duplicates", "Duplicates", "Давхардал", "copy"}}, []futureMenu{{"import", "Import contacts", "Импорт", "upload"}}},

	// products has one working screen and nothing else to stand on: the table
	// holds sku, name, price and active, so categories, price lists, units,
	// attributes and tax profiles would all be menu entries over no data.
	"io.example.products": {"products", nil, nil},

	"io.example.inventory": {"inventory", []futureMenu{{"replenishment", "Replenishment", "Нөхөн дүүргэлт", "refresh-cw"}}, []futureMenu{{"warehouses", "Warehouses", "Агуулах", "warehouse"}}},

	"io.example.billing": {"billing", []futureMenu{{"reports", "Revenue reports", "Орлогын тайлан", "chart-column"}}, nil},

	"io.example.documents": {"documents", []futureMenu{{"approvals", "Approval queue", "Батлах дараалал", "list-checks"}}, nil},

	"io.example.esign": {"esign", []futureMenu{{"logs", "Signature logs", "Гарын үсгийн лог", "scroll-text"}}, nil},

	// The developer portal's own screens, all backed by the OAuth2 provider.
	"io.example.developer_portal": {"developer", []futureMenu{{"api-keys", "API keys", "API түлхүүр", "key-round"}, {"audit", "Access audit", "Хандалтын аудит", "scroll-text"}}, []futureMenu{{"scopes", "OAuth scopes", "OAuth scope", "shield-check"}, {"redirects", "Redirect policies", "Redirect бодлого", "route"}, {"signing-keys", "Signing keys", "Гарын үсгийн түлхүүр", "key-square"}}},

	"io.example.gov_services": {"gov-services", []futureMenu{{"requests", "Service requests", "Үйлчилгээний хүсэлт", "inbox"}, {"appointments", "Appointments", "Цаг захиалга", "calendar-clock"}}, []futureMenu{{"catalog", "Service catalog", "Үйлчилгээний каталог", "landmark"}, {"workflow", "Decision workflow", "Шийдвэрлэх урсгал", "workflow"}, {"sla", "Service levels", "Үйлчилгээний түвшин", "timer"}}},
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
			localized(internal.MenuDefinition{ID: modulesID, AppID: mod.ID(), AppName: mod.Name(), Label: "Modules", Icon: "boxes", Order: 10, Labels: map[string]string{"mn": "Модуль"}}, locale))
		// Settings is only a heading over its entries, so an app with none —
		// products, billing, documents, esign — must not render an empty one.
		if len(bp.Settings) > 0 {
			menus = append(menus,
				localized(internal.MenuDefinition{ID: settingsID, AppID: mod.ID(), AppName: mod.Name(), Label: "Settings", Icon: "settings", Order: 20, Labels: map[string]string{"mn": "Тохиргоо"}}, locale))
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
	label := item.EN
	if locale == "mn" {
		label = item.MN
	}
	return internal.MenuDefinition{ID: fmt.Sprintf("%s_%s", slug, item.ID), AppID: appID, AppName: appName, ParentID: parent, Label: label, Path: fmt.Sprintf("/module/%s/%s", slug, item.ID), Icon: item.Icon, Order: order}
}
