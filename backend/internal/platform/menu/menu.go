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

var blueprints = map[string]blueprint{
	"io.example.contacts":  {"contacts", []futureMenu{{"segments", "Segments", "Сегментүүд", "users"}, {"activity", "Contact activity", "Харилцааны түүх", "activity"}}, []futureMenu{{"fields", "Custom fields", "Нэмэлт талбар", "settings"}, {"duplicates", "Duplicate rules", "Давхардлын дүрэм", "copy"}, {"import", "Import mapping", "Импортын зураглал", "upload"}}},
	"io.example.products":  {"products", []futureMenu{{"categories", "Categories", "Ангилал", "tags"}, {"price-lists", "Price lists", "Үнийн жагсаалт", "badge-dollar-sign"}}, []futureMenu{{"units", "Units of measure", "Хэмжих нэгж", "ruler"}, {"attributes", "Product attributes", "Барааны шинж", "sliders"}, {"taxes", "Tax profiles", "Татварын тохиргоо", "percent"}}},
	"io.example.inventory": {"inventory", []futureMenu{{"transfers", "Stock transfers", "Бараа шилжүүлэг", "arrow-right-left"}, {"replenishment", "Replenishment", "Нөхөн дүүргэлт", "refresh-cw"}}, []futureMenu{{"warehouses", "Warehouse settings", "Агуулахын тохиргоо", "warehouse"}, {"routes", "Logistics routes", "Ложистикийн маршрут", "route"}, {"valuation", "Valuation methods", "Өртөг тооцоолол", "calculator"}}},
	"io.example.billing":   {"billing", []futureMenu{{"payments", "Payments", "Төлбөрүүд", "wallet"}, {"reports", "Revenue reports", "Орлогын тайлан", "chart-column"}}, []futureMenu{{"numbering", "Invoice numbering", "Нэхэмжлэх дугаарлалт", "list-ordered"}, {"ebarimt", "e-Barimt connection", "e-Barimt холболт", "receipt"}, {"payment-methods", "Payment methods", "Төлбөрийн хэлбэр", "credit-card"}}},
	"io.example.documents": {"documents", []futureMenu{{"approvals", "Approval queue", "Батлах дараалал", "list-checks"}, {"templates", "Document templates", "Баримтын загвар", "files"}}, []futureMenu{{"workflows", "Document workflows", "Баримтын урсгал", "workflow"}, {"signatures", "Signature policies", "Гарын үсгийн бодлого", "pen-tool"}, {"retention", "Retention rules", "Хадгалалтын дүрэм", "archive"}}},
	"io.example.esign":     {"esign", []futureMenu{{"logs", "Signature logs", "Гарын үсгийн лог", "scroll-text"}, {"batch", "Batch signing", "Багц баталгаажуулалт", "layers"}}, []futureMenu{{"placement", "Stamp placement", "Тамганы байрлал", "move"}, {"hsm", "HSM connection", "HSM холболт", "server-cog"}, {"policies", "Signing policies", "Гарын үсгийн бодлого", "shield-check"}}},
	// No Webhooks entry: Settings → Integrations already registers webhook
	// listeners with a target URL and a signing secret, and a second screen
	// for the same records would only disagree with the first one eventually.
	// Access audit sits under Modules rather than Settings — it is something
	// you read, not something you configure.
	"io.example.developer_portal": {"developer", []futureMenu{{"api-keys", "API keys", "API түлхүүр", "key-round"}, {"audit", "Access audit", "Хандалтын аудит", "scroll-text"}}, []futureMenu{{"scopes", "OAuth scopes", "OAuth scope", "shield-check"}, {"redirects", "Redirect policies", "Redirect бодлого", "route"}, {"signing-keys", "Signing keys", "Гарын үсгийн түлхүүр", "key-square"}}},
	"io.example.gov_services":     {"gov-services", []futureMenu{{"requests", "Service requests", "Үйлчилгээний хүсэлт", "inbox"}, {"appointments", "Appointments", "Цаг захиалга", "calendar-clock"}}, []futureMenu{{"catalog", "Service catalog", "Үйлчилгээний каталог", "landmark"}, {"workflow", "Decision workflow", "Шийдвэрлэх урсгал", "workflow"}, {"sla", "Service levels", "Үйлчилгээний түвшин", "timer"}}},
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
		menus = append(menus,
			localized(internal.MenuDefinition{ID: modulesID, AppID: mod.ID(), AppName: mod.Name(), Label: "Modules", Icon: "boxes", Order: 10, Labels: map[string]string{"mn": "Модуль"}}, locale),
			localized(internal.MenuDefinition{ID: settingsID, AppID: mod.ID(), AppName: mod.Name(), Label: "Settings", Icon: "settings", Order: 20, Labels: map[string]string{"mn": "Тохиргоо"}}, locale),
		)
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
