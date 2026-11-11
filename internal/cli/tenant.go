package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// Tenant selection for the CLI's operator-run entry points (TKT-M3SHYE0).
//
// A tenant deployment serves several tenants from one config. Tenant mode
// is switched on by $RELA_TENANT_MODE=1, and the tenants are those the
// control table in $RELA_DATABASE_URL serves (TKT-HF5KE12). Both come from
// [tenant.ForDeployment] through [selectTenant] alone; the rest of the CLI
// sees only the chosen [tenant.Tenant]. The HTTP host picks the tenant from the verified
// `org_id`; the CLI, the scheduler and stdio MCP have no assertion, so the
// operator names the tenant with --tenant (or $RELA_TENANT). The rules:
//
//   - Without tenant mode nothing changes, and --tenant is refused. A
//     project that still holds the old tenant map file is refused, and so
//     is a database that serves tenants, so a forgotten switch never reads
//     `public` instead of a tenant.
//   - In tenant mode, a command that touches data must name a tenant the
//     control table serves. A missing, unknown, suspended or erasing tenant
//     is an error; there is no default store to fall back to.
//   - The tenant is opened exactly as the server opens it
//     ([tenant.OpenOne]): schema checked and locked, its own audit sink, its
//     own store. Its queue, scheduler run state and settings live in its
//     schema, so `rela --tenant X scheduler` runs X's schedules only.
//
// Identity is bound at the edge as before, with [appbuild.IdentityBinder]
// over the tenant's services, so a principal is resolved in that tenant's
// graph and no other. The principal carries no org claim: isolation comes
// from the store handle, not from the principal.

// tenantOrg is the --tenant value of this invocation, set by [runKong]
// before any Run method executes, like [projectPath].
var tenantOrg string

// openProject opens the services of the project at startDir for one CLI
// entry point. Every CLI path that opens project data goes through it, so
// none can skip tenant mode (TestCLIOpensProjectsOnlyThroughOpenProject).
//
// Callers close the returned closer, never the services. On a tenant it is
// the [tenant.Opened], which closes the services and then releases the
// tenant's schema lock; closing the services alone would keep the lock.
func openProject(startDir, org string, opts ...appbuild.Option) (*appbuild.Services, io.Closer, error) {
	choice, err := selectTenant(startDir, org, os.Getenv)
	if err != nil {
		return nil, nil, err
	}
	if !choice.tenantMode {
		if dsn := os.Getenv("RELA_DATABASE_URL"); dsn != "" {
			if guardErr := tenant.RefuseTenantDatabase(context.Background(), dsn); guardErr != nil {
				return nil, nil, guardErr
			}
		}
		svc, openErr := appbuild.Discover(startDir, script.NewEngine(), opts...)
		if openErr != nil {
			return nil, nil, openErr
		}
		return svc, svc, nil
	}
	base, err := appbuild.DiscoverSharedBase(startDir, script.NewEngine(), opts...)
	if err != nil {
		return nil, nil, err
	}
	opened, err := tenant.OpenOne(context.Background(), base, choice.tenant)
	if err != nil {
		// The error may name the database host and user (pgx redacts the
		// password), never the DSN itself.
		return nil, nil, fmt.Errorf("open tenant %q: %w", choice.tenant.OrgID, err)
	}
	stop, err := watchTenant(choice.tenant.OrgID)
	if err != nil {
		_ = opened.Close()
		return nil, nil, err
	}
	return opened.Services, watchedTenant{opened: opened, stop: stop}, nil
}

// tenantWatchInterval is how often [watchTenant] checks the tenant. A
// variable so a test need not wait the full interval.
var tenantWatchInterval = tenant.RevalidateInterval

// cancelCommand cancels the context of this invocation, as SIGTERM does. Set
// by [runKong]; nil outside it.
var cancelCommand context.CancelFunc

// tenantExitGrace is how long a command whose tenant stopped being served
// may take to end on its own before the process exits.
const tenantExitGrace = 30 * time.Second

// tenantRefused ends the command when the tenant it holds open is no longer
// served. It cancels the command's context, so the scheduler and MCP loops
// stop and deferred cleanup runs: the store, the audit sink and the run
// state close in order. A command that does not end within
// [tenantExitGrace] is stopped with os.Exit; exiting ends its database
// sessions, which releases the tenant's schema lock so an erasure can
// proceed. A variable so a test can observe it instead.
var tenantRefused = func(org string, err error) {
	fmt.Fprintf(os.Stderr, "rela: tenant %q is no longer served (suspended or being erased): %v; stopping\n", org, err)
	if cancelCommand == nil {
		os.Exit(1)
	}
	cancelCommand()
	time.AfterFunc(tenantExitGrace, func() { os.Exit(1) })
}

// watchTenant makes a long-running process such as `rela --tenant X
// scheduler` or stdio MCP stop when X is suspended or erased, as a server
// closes it. The returned stop ends the watcher.
func watchTenant(org string) (func(), error) {
	return tenant.WatchDeployment(context.Background(), os.Getenv, org, tenantWatchInterval,
		func(err error) { tenantRefused(org, err) })
}

// watchedTenant closes an opened tenant after stopping its watcher.
type watchedTenant struct {
	opened io.Closer
	stop   func()
}

func (w watchedTenant) Close() error {
	w.stop()
	return w.opened.Close()
}

// tenantChoice is the store an entry point acts on: the project and, in a
// tenant project, the one tenant it named.
type tenantChoice struct {
	// root is the project root; "" when the directory is in no project.
	root string
	// tenantMode is true in tenant mode; tenant is then the one named.
	tenantMode bool
	tenant     tenant.Tenant
}

// selectTenant returns the store org names for the project startDir
// belongs to. Without tenant mode naming a tenant is
// [tenant.ErrNotTenantMode]; in tenant mode a missing or unknown org is an
// error, and no tenant is returned with it. A project that holds the old
// tenant map file is refused either way.
//
// It does not check whether a single-store database serves tenants: that
// needs a connection, so [openProject] and resolveDSN check it when they
// open the store.
func selectTenant(startDir, org string, getenv func(string) string) (tenantChoice, error) {
	root, err := appbuild.DiscoverRoot(startDir)
	if err != nil {
		return tenantChoice{}, err
	}
	t, ok, err := tenant.Choose(context.Background(), root, org, getenv)
	if err != nil {
		return tenantChoice{}, err
	}
	if !ok {
		return tenantChoice{root: root}, nil
	}
	return tenantChoice{root: root, tenantMode: true, tenant: t}, nil
}

// noTenantToOpen reports whether err from [openProject] means tenant mode
// was reached without a tenant to open: none was named, or this
// build cannot serve tenants. A tenant-free command can skip a data step on
// it; an unknown tenant is a different error and is not matched.
func noTenantToOpen(err error) bool {
	return errors.Is(err, tenant.ErrTenantRequired) || errors.Is(err, tenant.ErrUnsupportedBuild)
}
