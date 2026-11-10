// rela-docs renders a Markdown manual with embedded rela Lua islands (field
// tables, enum meanings, lifecycle/relation diagrams, role matrices, and live
// screenshot{} captures) to resolved Markdown.
//
// It is a separate binary from `rela` on purpose: the screenshot{} island
// drives a headless Chrome via internal/docscapture, which links a
// browser-automation library (~15 MB). Shipping docs build here keeps that
// dependency out of the default `rela` / `rela-server` binaries, which the
// common CRUD user never needs.
//
// Usage:
//
//	rela-docs build <manual.md> [--out out.md] [--strict] [--project .]
package main

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kong"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/docscli"
	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// Version is set at build time.
var Version = "dev"

// cli is the kong-parsed root: global flags plus the docs commands.
type cli struct {
	Version kong.VersionFlag `help:"Print version information and exit."`
	Project string           `help:"Project directory (default: auto-detect from cwd)." env:"RELA_PROJECT"`

	docscli.Root
}

// coverage-ignore-func: main function - entry point, tested via integration tests
func main() {
	os.Exit(run())
}

// coverage-ignore-func: CLI entry point - tested via integration tests
func run() int {
	var c cli
	ktx := kong.Parse(&c,
		kong.Name("rela-docs"),
		kong.Description("Render Markdown manuals with rela Lua islands to resolved Markdown."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
		kong.Vars{"version": Version},
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx = principal.With(ctx, principal.Principal{
		User: principal.SystemUser(),
		Tool: principal.ToolCLI,
	})

	if err := refuseTenantMode(c.Project); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	// docs build reads the metamodel + acl.yaml; Discover wires the project.
	// The postgres build reads its DSN from $RELA_DATABASE_URL inside Discover
	// (env-only); the filesystem build ignores it.
	svc, err := appbuild.Discover(c.Project, script.NewEngine())
	if err != nil {
		fmt.Fprintln(os.Stderr, relaerrors.WrapDiscoverError(err))
		return 1
	}
	defer svc.Close()

	// The request edge: resolve the stamped principal once into the ACL
	// request scope the build's reads use.
	bind, err := appbuild.IdentityBinder(svc)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx = appbuild.BindOrWarn(ctx, bind)

	ktx.BindTo(ctx, (*context.Context)(nil))
	// appbuild.Services satisfies docscli.Project via Meta()/Paths().
	ktx.BindTo(svc, (*docscli.Project)(nil))

	if err := ktx.Run(); err != nil {
		var exitErr *relaerrors.ExitError
		if stderrors.As(err, &exitErr) {
			return exitErr.Code
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// errTenantProject is rela-docs' refusal of a tenant deployment
// (TKT-M3SHYE0). rela-docs opens one store from $RELA_DATABASE_URL, which in
// tenant mode is the base DSN and names no tenant, so building against it
// would read, and on first open migrate, a schema no tenant owns.
var errTenantProject = stderrors.New("this deployment serves several tenants: " +
	"rela-docs builds against one store and cannot act in a tenant")

// refuseTenantMode returns [errTenantProject] in tenant mode, for a project
// that still holds the old tenant map file, and for a database that serves
// tenants. A directory in no project passes: the open that follows reports
// it as before.
func refuseTenantMode(startDir string) error {
	if tenant.ModeEnabled(os.Getenv) {
		return errTenantProject
	}
	root, err := appbuild.DiscoverRoot(startDir)
	if err != nil || root == "" {
		return err
	}
	if err := tenant.RefuseLegacyMap(root); err != nil {
		return fmt.Errorf("%w: %w", errTenantProject, err)
	}
	if dsn := os.Getenv("RELA_DATABASE_URL"); dsn != "" {
		if err := tenant.RefuseTenantDatabase(context.Background(), dsn); err != nil {
			return fmt.Errorf("%w: %w", errTenantProject, err)
		}
	}
	return nil
}
