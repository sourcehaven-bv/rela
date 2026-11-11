package tenant

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
)

// EnvTenantMode switches tenant mode on when set to "1", for the server
// (which also takes -tenant-mode) and for every operator entry point. Tenant
// mode is an explicit switch: nothing turns it on by being present.
const EnvTenantMode = "RELA_TENANT_MODE"

// ModeEnabled reports whether getenv switches tenant mode on.
func ModeEnabled(getenv func(string) string) bool { return getenv(EnvTenantMode) == "1" }

// ErrTenantRequired reports that tenant mode is on and the entry point named
// no tenant. It is kept apart from [ErrUnknownTenant] so a forgotten
// `--tenant` is not reported as a typo in the org id.
var ErrTenantRequired = errors.New("tenant: tenant mode is on; " +
	"name a tenant with --tenant <org_id> or $RELA_TENANT")

// ErrNotTenantMode reports that a tenant was named while tenant mode is off.
// Refused rather than ignored: an operator who names a tenant expects to act
// in it, and silently acting on the single default store instead would be
// the wrong data.
var ErrNotTenantMode = errors.New("tenant: a tenant was named, but tenant mode is off; set " +
	EnvTenantMode + "=1")

// Select returns the tenant orgID names, for an operator-run entry point
// that acts in exactly one tenant. It resolves through r, the lookup the
// server routes by, so a tenant the server would refuse is refused here too.
// An empty orgID is [ErrTenantRequired]; an org r does not serve is
// [ErrUnknownTenant]. Neither returns a usable tenant.
//
// Nil: rejected — r is required.
func Select(ctx context.Context, r Resolver, orgID string) (Tenant, error) {
	if r == nil {
		return Tenant{}, errors.New("tenant.Select: resolver is required")
	}
	if orgID == "" {
		return Tenant{}, ErrTenantRequired
	}
	return r.Resolve(ctx, orgID)
}

// ErrUnsupportedBuild reports tenant mode on a build that cannot serve it.
// Such a build keeps data in the project directory, so every tenant would
// read the same data.
var ErrUnsupportedBuild = errors.New("tenant: tenant mode needs the postgres build (rela-postgres, " +
	"rela-server-postgres): this build keeps data in the project directory, so every tenant would " +
	"read the same data")

// ForDeployment is how an operator entry point learns whether this
// deployment serves several tenants, and through which resolver. It returns
// nil without an error when tenant mode is off. It is the one place those
// entry points look at how tenants are configured. The caller closes the
// resolver.
//
// On a build without tenant mode, tenant mode is [ErrUnsupportedBuild]:
// callers must treat that as tenant mode they cannot serve, never as a
// single-store deployment.
func ForDeployment(ctx context.Context, getenv func(string) string) (*ClusterResolver, error) {
	if !ModeEnabled(getenv) {
		return nil, nil //nolint:nilnil // tenant mode off: no resolver, and not an error
	}
	if !Supported {
		return nil, ErrUnsupportedBuild
	}
	return NewClusterResolver(ctx, getenv("RELA_DATABASE_URL"))
}

// legacyMapFile is the tenant map file of rela v26.11.2, which no longer
// configures anything (TKT-HF5KE12).
const legacyMapFile = "tenants.yaml"

// ErrLegacyTenantMap reports a project that still holds the old tenant map
// file. Its contents are never read. Starting anyway would ignore a file
// the operator meant to switch tenant mode on with, and a server without
// tenant mode would serve every org from one store.
var ErrLegacyTenantMap = errors.New("tenant: the project holds " + legacyMapFile + ", which rela no " +
	"longer reads: tenants are derived from org_id. Set " + EnvTenantMode + "=1, register each tenant " +
	"with `rela-server tenant create`, then remove the file (see the PostgreSQL backend guide)")

// RefuseLegacyMap returns [ErrLegacyTenantMap] when the project at root
// holds the old tenant map file. It checks only that the file exists.
func RefuseLegacyMap(root string) error {
	if root == "" {
		return nil
	}
	switch _, err := os.Lstat(filepath.Join(root, legacyMapFile)); {
	case err == nil:
		return ErrLegacyTenantMap
	case errors.Is(err, os.ErrNotExist):
		return nil
	default:
		return fmt.Errorf("tenant: check %s: %w", legacyMapFile, err)
	}
}

// OpenOne opens exactly one tenant over base, as the server's registry
// opens each tenant: its schema checked and locked shared, its own audit
// sink under [AuditRoot] of the project's cache directory, and its own
// store. The caller closes the result with [Opened.Close], never by closing
// its Services alone, so the schema lock is released after the store.
//
// Nil: rejected — a nil base has no configuration to open the tenant with.
func OpenOne(ctx context.Context, base *appbuild.SharedBase, t Tenant) (*Opened, error) {
	if base == nil {
		return nil, errors.New("tenant.OpenOne: base is required")
	}
	open, err := SharedBaseOpener(base, AuditRoot(base.CacheDir()))
	if err != nil {
		return nil, err
	}
	return open(ctx, t)
}

// Pin checks that t's connection lands in t's own schema and takes the
// schema's lock shared, as [SharedBaseOpener] does before it opens a store.
// It is for an operator command that reaches the tenant's database without
// a store, such as `rela db migrate`: without the check a schema that does
// not exist yet is skipped by PostgreSQL's `search_path`, and the command
// would act on `public`. Close the result when the command is done.
func Pin(ctx context.Context, t Tenant) (io.Closer, error) {
	return pinSchema(ctx, t.DSN, t.Schema)
}

// Choose is the tenant selection of an operator entry point for the project
// at root: it refuses the old tenant map file, and in tenant mode returns
// the tenant orgID names, as [Select] does. Without tenant mode it returns
// ok false, and naming a tenant is [ErrNotTenantMode].
func Choose(ctx context.Context, root, orgID string, getenv func(string) string) (t Tenant, ok bool, err error) {
	if legacyErr := RefuseLegacyMap(root); legacyErr != nil {
		return Tenant{}, false, legacyErr
	}
	r, err := ForDeployment(ctx, getenv)
	if err != nil {
		return Tenant{}, false, err
	}
	if r == nil {
		if orgID != "" {
			return Tenant{}, false, ErrNotTenantMode
		}
		return Tenant{}, false, nil
	}
	defer func() { _ = r.Close() }()
	t, err = Select(ctx, r, orgID)
	if err != nil {
		return Tenant{}, false, err
	}
	return t, true, nil
}
