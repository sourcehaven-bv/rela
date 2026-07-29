// Package cli wires the kong-based CLI. Subcommand structs live in
// dedicated <name>.go files. Each subcommand declares a `Run` method
// whose parameters name the services it needs; kong's dispatcher
// matches them against the bindings registered in [Execute].
//
// Package-level globals (verbose, quiet, outputFormat, out) are set by
// [runKong] before any Run method executes and read by subcommands
// that need them (fmt --verbose, validate --quiet, etc.). They are
// invocation-scoped — kong constructs a fresh CLI per process, and
// there is exactly one process per CLI invocation.
package cli

import (
	"context"
	stderrors "errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kong"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/cmdexec"
	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/scheduler"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// Version is set at build time.
var Version = "dev"

// The scheduler command's workspace is bound at the wiring site below;
// kong.BindTo only verifies assignability at runtime, so pin it here.
var _ scheduler.WorkspaceProvider = (*appbuild.Services)(nil)

// Invocation-scoped globals populated by [runKong] before any Run
// method executes. Subcommands read them for behavior driven by the
// root persistent flags.
var (
	out          *output.Writer
	verbose      bool
	quiet        bool
	outputFormat string
	projectPath  string
)

// CLI is the kong-parsed root.
//
// TODO(TKT-N0IKN9): CLI has 38 exported fields (kong binds one per subcommand,
// so growth is structural here) — over the 20-field load line. Revisit grouping
// subcommands into sub-structs; ratchet this number down if/when that lands.
//
//plimsoll:max-fields=46
type CLI struct {
	// Global flags.
	Project string `help:"Project directory (default: auto-detect from cwd)." env:"RELA_PROJECT"`
	Output  string `help:"Output format (table, json)." short:"o" default:"table"`
	Verbose bool   `help:"Verbose output." short:"v"`
	Quiet   bool   `help:"Quiet output."   short:"q"`

	// Subcommands.
	Version    VersionCmd    `cmd:"" help:"Print version information."`
	Init       InitCmd       `cmd:"" help:"Initialize a new rela project."`
	Apps       AppsCmd       `cmd:"" help:"Manage custom data-entry apps."`
	Migrate    MigrateCmd    `cmd:"" help:"Migrate project files to current schema."`
	Completion CompletionCmd `cmd:"" help:"Generate shell completion scripts."`
	Mcp        McpCmd        `cmd:"" name:"mcp" help:"Start the MCP server."`
	Db         DBCmd         `cmd:"" name:"db" help:"Manage the PostgreSQL schema (postgres build)."`
	Flow       FlowCmd       `cmd:"" help:"Run an interactive Lua flow."`
	Validate   ValidateCmd   `cmd:"" help:"Validate project configuration files."`

	Show      ShowCmd      `cmd:"" help:"Show entity details."`
	List      ListCmd      `cmd:"" help:"List entities."`
	Create    CreateCmd    `cmd:"" help:"Create a new entity."`
	Update    UpdateCmd    `cmd:"" help:"Update an entity."`
	Delete    DeleteCmd    `cmd:"" help:"Delete an entity."`
	Link      LinkCmd      `cmd:"" help:"Create a relation between entities."`
	Unlink    UnlinkCmd    `cmd:"" help:"Remove a relation between entities."`
	Trace     TraceCmd     `cmd:"" help:"Trace dependencies between entities."`
	Graph     GraphCmd     `cmd:"" help:"Export graph to Graphviz DOT format."`
	Export    ExportCmd    `cmd:"" help:"Export entities in JSON, CSV, or YAML format."`
	Render    RenderCmd    `cmd:"" help:"Render an entity to a file via a registered transform (e.g. pdf)."`
	Import    ImportCmd    `cmd:"" help:"Import entities and relations from JSON, YAML, or CSV."`
	Fmt       FmtCmd       `cmd:"" help:"Format entity and relation files."`
	Normalize NormalizeCmd `cmd:"" help:"Normalize markdown headers in entity files."`
	Schema    SchemaCmd    `cmd:"" help:"View the metamodel schema."`
	Template  TemplateCmd  `cmd:"" help:"Manage entity and relation templates."`
	Analyze   AnalyzeCmd   `cmd:"" help:"Analyze the entity graph."`
	ACL       ACLCmd       `cmd:"" name:"acl" help:"Audit the ACL policy (acl.yaml)."`
	Rename    RenameCmd    `cmd:"" help:"Rename entities or relations."`
	History   HistoryCmd   `cmd:"" help:"Show an entity's version history (postgres build)."`
	Restore   RestoreCmd   `cmd:"" help:"Restore an entity to a past version (postgres build)."`

	RelationHistory RelationHistoryCmd `cmd:"" name:"relation-history" help:"Show a relation's version history (postgres build)."`
	RelationRestore RelationRestoreCmd `cmd:"" name:"relation-restore" help:"Restore a relation to a past version (postgres build)."`

	HistoryPurge         HistoryPurgeCmd         `cmd:"" name:"history-purge" help:"Hard-delete an entity's version history for compliance (postgres build; irreversible)."`
	RelationHistoryPurge RelationHistoryPurgeCmd `cmd:"" name:"relation-history-purge" help:"Hard-delete a relation's version history for compliance (postgres build; irreversible)."`
	Attach               AttachCmd               `cmd:"" help:"Attach file(s) to an entity."`
	Attachments          AttachmentsCmd          `cmd:"" help:"List attachments for an entity."`
	Detach               DetachCmd               `cmd:"" help:"Remove the attachment from an entity property."`
	Gc                   GcCmd                   `cmd:"" name:"gc" help:"Garbage collect orphaned files."`
	Script               ScriptCmd               `cmd:"" help:"Execute a Lua script against the graph."`
	Scheduler            SchedulerCmd            `cmd:"" help:"Run scheduled Lua tasks."`
	Renumber             RenumberCmd             `cmd:"" help:"Renumber managed order properties on orderable relations."`
	Sync                 SyncCmd                 `cmd:"" help:"Sync local changes with a remote rela-server."`
}

// VersionCmd needs no services.
type VersionCmd struct{}

// Run prints the version and returns.
func (c *VersionCmd) Run() error {
	fmt.Printf("rela version %s\n", Version)
	return nil
}

// Execute is the process entry point.
//
// coverage-ignore-func: CLI entry point - tested via integration tests
func Execute() {
	os.Exit(runKong())
}

// coverage-ignore-func: CLI entry point - tested via integration tests
func runKong() int {
	var cli CLI
	ktx := kong.Parse(&cli,
		kong.Name("rela"),
		kong.Description("rela is a schema-driven entity-graph platform for tracking entities and their relationships."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
		kong.Vars{"version": Version},
	)

	// Populate package globals before any Run executes.
	verbose = cli.Verbose
	quiet = cli.Quiet
	outputFormat = cli.Output
	projectPath = cli.Project

	// Host-level command-confinement opt-out, from the same env var the server
	// honors. `rela render` shells out to a converter; on a host that cannot
	// sandbox it fails closed unless this is set. (A local CLI user can already
	// run anything, so this is convenience more than a security boundary — but
	// keeping one knob avoids surprising divergence from the server.)
	cmdexec.SetUnconfinedByDefault(os.Getenv("RELA_UNCONFINED_COMMANDS") == "1")

	configureKongLogging(verbose, quiet)
	out = output.New(output.Format(outputFormat))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx = principal.With(ctx, principal.Principal{
		User: principal.SystemUser(),
		Tool: principal.ToolCLI,
	})

	var svc *appbuild.Services
	var bundles *cliBundles
	if requiresProject(ktx.Command()) {
		var err error
		// The postgres build reads its DSN from $RELA_DATABASE_URL inside
		// Discover (env-only — no flag — so the credential never lands on a
		// command line). The filesystem build ignores it.
		svc, err = appbuild.Discover(projectPath, script.NewEngine())
		if err != nil {
			fmt.Fprintln(os.Stderr, relaerrors.WrapDiscoverError(err))
			return 1
		}
		defer svc.Close()
		bundles, err = newCLIBundles(svc)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		// Resolve entity display titles through the metamodel so the entity
		// table, detail, and trace-tree output honor display_property (bare
		// name or template), matching the data-entry app. Without a project
		// (no metamodel) the writer falls back to the literal `title` property.
		out.Titles = bundles.read.Meta
	}

	ktx.BindTo(ctx, (*context.Context)(nil))
	ktx.Bind(out)
	if bundles != nil {
		ktx.Bind(bundles.read, bundles.write, bundles.attachment, bundles.renametype, bundles.analysis)
		// The scheduler command's workspace is supplied here at the wiring
		// site (consumer-side interface): appbuild.Services satisfies
		// scheduler.WorkspaceProvider directly.
		ktx.BindTo(svc, (*scheduler.WorkspaceProvider)(nil))
	}

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

func configureKongLogging(verbose, quiet bool) {
	level := slog.LevelInfo
	switch {
	case verbose:
		level = slog.LevelDebug
	case quiet:
		level = slog.LevelWarn
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}

// requiresProject reports whether the matched kong command needs
// project services constructed via [appbuild.Discover]. Subcommands
// not in this list don't need project services (version, init,
// completion, migrate) or do their own discovery (mcp, flow, validate).
func requiresProject(cmd string) bool {
	switch firstKongToken(cmd) {
	case "show", "list", "trace", "graph", "export", "render", "fmt", "schema",
		"template", "create", "update", "delete", "link", "unlink",
		"detach", "import", "normalize", "script", "scheduler",
		"rename", "analyze", "acl", "attach", "attachments", "gc", "renumber",
		"sync", "history", "restore",
		"relation-history", "relation-restore", "history-purge", "relation-history-purge":
		return true
	}
	return false
}

// firstKongToken returns the leading whitespace-delimited token of
// kong's Command() string (e.g. "show <id>" -> "show", "analyze
// orphans" -> "analyze").
func firstKongToken(s string) string {
	for i, r := range s {
		if r == ' ' {
			return s[:i]
		}
	}
	return s
}
