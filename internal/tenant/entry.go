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

// ErrTenantRequired reports that a project is in tenant mode and the entry
// point named no tenant. It is kept apart from [ErrUnknownTenant] so a
// forgotten `--tenant` is not reported as a typo in the org id.
var ErrTenantRequired = errors.New("tenant: this project serves several tenants; " +
	"name one with --tenant <org_id> or $RELA_TENANT")

// ErrNoTenantMap reports that a tenant was named for a project without a
// tenant map. Refused rather than ignored: an operator who names a tenant
// expects to act in it, and silently acting on the single default store
// instead would be the wrong data.
var ErrNoTenantMap = errors.New("tenant: a tenant was named, but the project has no " + ConfigFileName)

// ConfigPath returns the path of the tenant map in the project root, or ""
// when the root has none. The presence of the file is what turns tenant mode
// on, for the server and for every operator entry point.
func ConfigPath(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("tenant: invalid project dir: %w", err)
	}
	p := filepath.Join(abs, ConfigFileName)
	switch _, err := os.Stat(p); {
	case err == nil:
		return p, nil
	case errors.Is(err, os.ErrNotExist):
		return "", nil
	default:
		return "", fmt.Errorf("tenant: check %s: %w", p, err)
	}
}

// LoadDeployment is [LoadConfig] plus the base DSN rule every entry point
// shares: a map that gives no `base_dsn` takes it from $RELA_DATABASE_URL.
// The file may not carry a password (LoadConfig refuses one), so the
// environment is where the credential lives. getenv is injected so tests do
// not mutate the process environment.
func LoadDeployment(path string, getenv func(string) string) (*Config, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}
	if cfg.BaseDSN == "" {
		cfg.BaseDSN = getenv("RELA_DATABASE_URL")
	}
	return cfg, nil
}

// Select returns the tenant orgID names, for an operator-run entry point
// that acts in exactly one tenant. It resolves through r, the lookup the
// server routes by, so a tenant the server would refuse is refused here too.
// An empty orgID is [ErrTenantRequired]; an org r does not serve is
// [ErrUnknownTenant]. Neither returns a usable tenant.
//
// Nil: rejected — r is required.
func Select(r Resolver, orgID string) (Tenant, error) {
	if r == nil {
		return Tenant{}, errors.New("tenant.Select: resolver is required")
	}
	if orgID == "" {
		return Tenant{}, ErrTenantRequired
	}
	return r.Resolve(orgID)
}

// ErrUnsupportedBuild reports a tenant project on a build that cannot serve
// tenant mode. Such a build keeps data in the project directory, so every
// tenant would read the same data.
var ErrUnsupportedBuild = errors.New("tenant: " + ConfigFileName + " is present, but tenant mode needs " +
	"the postgres build (rela-postgres): this build keeps data in the project directory, so every " +
	"tenant would read the same data")

// ForProject is how an operator entry point learns whether the project at
// root serves several tenants, and through which [Resolver]. It returns a
// nil Resolver for a single-store project. It is the one place those entry
// points look at how tenants are configured; they see only the Resolver it
// returns, so a different source of tenants changes this function alone.
//
// Today the source is the project's tenant map, read with [LoadDeployment].
// On a build without tenant mode a tenant project is [ErrUnsupportedBuild],
// returned before the map is read: callers must treat that as tenant mode
// they cannot serve, never as a single-store project.
func ForProject(root string, getenv func(string) string) (Resolver, error) {
	path, err := ConfigPath(root)
	if err != nil || path == "" {
		return nil, err
	}
	if !Supported {
		return nil, ErrUnsupportedBuild
	}
	cfg, err := LoadDeployment(path, getenv)
	if err != nil {
		return nil, err
	}
	r, err := cfg.Resolver()
	if err != nil {
		// Not `return cfg.Resolver()`: a nil *MapResolver in a Resolver is
		// not a nil interface, and would read as tenant mode.
		return nil, err
	}
	return r, nil
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
	open, err := SharedBaseOpener(base, AuditRoot(base.Paths().CacheDir))
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
