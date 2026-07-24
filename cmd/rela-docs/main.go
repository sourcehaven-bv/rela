// rela-docs renders a Markdown manual with embedded rela Lua islands (field
// tables, enum meanings, lifecycle/relation diagrams, role matrices, and live
// screenshot{} captures) to resolved Markdown.
//
// It is a separate binary from `rela` on purpose: the screenshot{} island
// drives a headless Chrome via internal/docscapture, which links chromedp
// (~15 MB). Shipping docs build here keeps that dependency out of the default
// `rela` / `rela-server` binaries, which the common CRUD user never needs.
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
)

// Version is set at build time.
var Version = "dev"

// cli is the kong-parsed root: global flags plus the docs commands.
type cli struct {
	Version kong.VersionFlag `help:"Print version information and exit."`
	Project string           `help:"Project directory (default: auto-detect from cwd)." env:"RELA_PROJECT"`

	docscli.Root
}

// coverage-ignore: main function - entry point, tested via integration tests
func main() {
	os.Exit(run())
}

// coverage-ignore: CLI entry point - tested via integration tests
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

	// docs build reads the metamodel + acl.yaml; Discover wires the project.
	// The postgres build reads its DSN from $RELA_DATABASE_URL inside Discover
	// (env-only); the filesystem build ignores it.
	svc, err := appbuild.Discover(c.Project, script.NewEngine())
	if err != nil {
		fmt.Fprintln(os.Stderr, relaerrors.WrapDiscoverError(err))
		return 1
	}
	defer svc.Close()

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
