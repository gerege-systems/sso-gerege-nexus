// Package apps assembles the business-module runtime. The platform owns HTTP,
// sessions and infrastructure; it does not need to import every module or know
// its route table.
package apps

import (
	"context"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/apps/billing"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/apps/contacts"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/apps/developer_portal"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/apps/documents"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/apps/esign"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/apps/gov_services"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/apps/inventory"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/apps/products"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/eidmongolia"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/gerege"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/integration"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/ssoprovider"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BackgroundModule interface {
	StartHousekeeping(context.Context)
}

type Runtime struct {
	Background []BackgroundModule
}

func Bootstrap(db *pgxpool.Pool, integrations *integration.Manager, eidMN *eidmongolia.Service, sso *ssoprovider.SSOProvider) Runtime {
	contacts.New(db)
	products.New(db)
	inventory.New(db, false)
	billing.New(db)
	documents.New(db)
	gov_services.New(db, integrations)
	developer_portal.NewDeveloperPortalModule(sso)
	esignModule := esign.New(db, gerege.NewEsignService(), eidMN, integrations)
	return Runtime{Background: []BackgroundModule{esignModule}}
}
