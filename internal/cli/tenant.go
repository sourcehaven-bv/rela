package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/storage"
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
// paths and r are nil when startDir is in no project, and r is nil for a
// single-store project. Any other discovery error is returned, so a failure
// to look is never read as single-store.
func projectTenants(startDir string, getenv func(string) string) (*project.Context, tenant.Resolver, error) {
	paths, err := project.Discover(startDir, storage.NewSafeFS(storage.NewOsFS()))
	if errors.Is(err, relaerrors.ErrNoProject) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	r, err := tenant.ForProject(paths.Root, getenv)
	if err != nil {
		return nil, nil, err
	}
	return paths, r, nil
}

// tenantChoice is the store an entry point acts on: the project and, in a
// tenant project, the one tenant it named.
type tenantChoice struct {
	// paths is the project; nil when the directory is in no project.
	paths *project.Context
	// tenantMode is true in a tenant project; tenant is then the one named.
	tenantMode bool
	tenant     tenant.Tenant
}

// selectTenant returns the store org names in the project startDir belongs
// to. In a single-store project naming a tenant is [tenant.ErrNoTenantMap];
// in a tenant project a missing or unknown org is an error, and no tenant is
// returned with it.
func selectTenant(startDir, org string, getenv func(string) string) (tenantChoice, error) {
	paths, r, err := projectTenants(startDir, getenv)
	if err != nil {
		return tenantChoice{}, err
	}
	if r == nil {
		if org != "" {
			return tenantChoice{}, tenant.ErrNoTenantMap
		}
		return tenantChoice{paths: paths}, nil
	}
	t, err := tenant.Select(r, org)
	if err != nil {
		return tenantChoice{}, err
	}
	return tenantChoice{paths: paths, tenantMode: true, tenant: t}, nil
}

// tenantFreeCommands need no tenant in tenant mode, because they read or
// write no tenant's data: the config they touch is shared by every tenant.
// Every other command is refused in tenant mode without a tenant, so a new
// command is gated until someone decides it belongs here.
//
// validate checks the shared config without a tenant; its steps that read
// data (scheduled mail recipients, --check) open the tenant themselves
// through [openProject] and so are gated there.
var tenantFreeCommands = map[string]bool{
	"version":    true,
	"init":       true,
	"completion": true,
	"apps":       true,
	"validate":   true,
}

// commandIsTenantFree reports whether cmd reads or writes no tenant's data.
// Bare `rela migrate` rewrites config files only; its data subcommands need
// the store, as [requiresProject] says.
func commandIsTenantFree(cmd string) bool {
	if firstKongToken(cmd) == "migrate" {
		return !requiresProject(cmd)
	}
	return tenantFreeCommands[firstKongToken(cmd)]
}

// checkTenantMode is the CLI's tenant gate, run once before a command is
// dispatched. In tenant mode it refuses a data command that names no
// tenant, or one the resolver does not serve, before anything is opened. In
// a single-store project it refuses only a named tenant.
//
// It judges the project startDir belongs to. A command that opens another
// project (`rela flow <script>`) is judged again by [openProject].
func checkTenantMode(cmd, startDir, org string, getenv func(string) string) error {
	if commandIsTenantFree(cmd) {
		return nil
	}
	_, err := selectTenant(startDir, org, getenv)
	return err
}
