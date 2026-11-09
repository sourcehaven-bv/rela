package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// Tenant selection for the CLI's operator-run entry points (TKT-M3SHYE0).
//
// A tenant project serves several tenants from one config. Whether a
// project is one, and which tenants it serves, comes from [tenant.ForProject]
// through [projectTenants] alone; the rest of the CLI sees only the
// [tenant.Resolver]. The HTTP host picks the tenant from the verified
// `org_id`; the CLI, the scheduler and stdio MCP have no assertion, so the
// operator names the tenant with --tenant (or $RELA_TENANT). The rules:
//
//   - In a single-store project nothing changes, and --tenant is refused.
//   - In a tenant project, a command that touches data must name a tenant
//     the resolver serves. A missing, unknown or erased tenant is an error;
//     there is no default store to fall back to.
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
	return opened.Services, opened, nil
}

// projectTenants discovers the project startDir belongs to and, when it
// serves several tenants, the resolver that names them. It is the CLI's one
// tenant-mode detection: everything else sees only the [tenant.Resolver].
// root is "" and r nil when startDir is in no project, and r is nil for a
// single-store project. Any other discovery error is returned, so a failure
// to look is never read as single-store.
func projectTenants(startDir string, getenv func(string) string) (root string, r tenant.Resolver, err error) {
	root, err = appbuild.DiscoverRoot(startDir)
	if err != nil || root == "" {
		return "", nil, err
	}
	r, err = tenant.ForProject(root, getenv)
	if err != nil {
		return "", nil, err
	}
	return root, r, nil
}

// tenantChoice is the store an entry point acts on: the project and, in a
// tenant project, the one tenant it named.
type tenantChoice struct {
	// root is the project root; "" when the directory is in no project.
	root string
	// tenantMode is true in a tenant project; tenant is then the one named.
	tenantMode bool
	tenant     tenant.Tenant
}

// selectTenant returns the store org names in the project startDir belongs
// to. In a single-store project naming a tenant is [tenant.ErrNoTenantMap];
// in a tenant project a missing or unknown org is an error, and no tenant is
// returned with it.
func selectTenant(startDir, org string, getenv func(string) string) (tenantChoice, error) {
	root, r, err := projectTenants(startDir, getenv)
	if err != nil {
		return tenantChoice{}, err
	}
	if r == nil {
		if org != "" {
			return tenantChoice{}, tenant.ErrNoTenantMap
		}
		return tenantChoice{root: root}, nil
	}
	t, err := tenant.Select(r, org)
	if err != nil {
		return tenantChoice{}, err
	}
	return tenantChoice{root: root, tenantMode: true, tenant: t}, nil
}

// noTenantToOpen reports whether err from [openProject] means a tenant
// project was reached without a tenant to open: none was named, or this
// build cannot serve tenants. A tenant-free command can skip a data step on
// it; an unknown tenant is a different error and is not matched.
func noTenantToOpen(err error) bool {
	return errors.Is(err, tenant.ErrTenantRequired) || errors.Is(err, tenant.ErrUnsupportedBuild)
}
