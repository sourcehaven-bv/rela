package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// tenantCommand is the first argument that selects the tenant
// administration commands instead of the server.
const tenantCommand = "tenant"

const tenantUsage = `usage: rela-server tenant COMMAND [-project DIR] ORG_ID

Manages the tenants of a tenant-mode deployment (postgres build only). Each
organization's data lives in its own schema, named from its org id; the
control table rela_control.tenants records each tenant's status. The
database is the one $RELA_DATABASE_URL names.

Commands:
  create [-adopt] ORG_ID   create the schema, run the migrations, and set
                           the status to active. -adopt takes over a schema
                           that already has the derived name.
  suspend ORG_ID           stop serving the tenant; its data stays.
  resume ORG_ID            serve a suspended tenant again.
  erase [-force] ORG_ID    set the status to erasing, which cannot be
                           undone, then drop the schema and remove the
                           audit directory. Without -force the command asks
                           you to type the org id to confirm.
  show ORG_ID              print the tenant's schema and status.

Before erasing, check that no open or recently closed incident, NCR or
dispute refers to the tenant while its evaluation period is still running
(PROCEDURE-u020). The erasure deletes the tenant's audit log at once.
`

// lifecycle is what the tenant commands need from [tenant.Provisioner].
type lifecycle interface {
	Create(ctx context.Context, orgID string, adopt bool) (tenant.Tenant, error)
	Suspend(ctx context.Context, orgID string) error
	Resume(ctx context.Context, orgID string) error
	Erase(ctx context.Context, orgID string) (tenant.Tenant, tenant.Erasure, error)
}

// recordReader is what `tenant show` needs from [tenant.ClusterResolver].
type recordReader interface {
	Lookup(ctx context.Context, orgID string) (tenant.Record, error)
	Close() error
}

// tenantIO is where the tenant commands read their confirmation and write
// their prompt and result, and how they reach the database. Passed in so
// the tests can drive it.
type tenantIO struct {
	in       io.Reader
	out, err io.Writer
	getenv   func(string) string
	// user names the operator in the erasure record.
	user func() string
	// provisioner and reader build the database clients from the base DSN.
	provisioner func(baseDSN, auditRoot string) (lifecycle, error)
	reader      func(ctx context.Context, baseDSN string) (recordReader, error)
}

// productionTenantIO returns the process's stdio and the real clients.
//
// coverage-ignore-func: process wiring; runTenant carries the logic.
func productionTenantIO() tenantIO {
	return tenantIO{
		in: os.Stdin, out: os.Stdout, err: os.Stderr, getenv: os.Getenv,
		user: func() string { return os.Getenv("USER") },
		provisioner: func(base, auditRoot string) (lifecycle, error) {
			return tenant.NewProvisioner(base, auditRoot)
		},
		reader: func(ctx context.Context, base string) (recordReader, error) {
			return tenant.NewClusterResolver(ctx, base)
		},
	}
}

// runTenantCommand runs `rela-server tenant ...` and returns the exit code.
//
// coverage-ignore-func: process entry point; runTenant carries the logic.
// coverage-ignore-start: main-or-wiring: reads the process stdio and signals
func runTenantCommand(args []string) int {
	return runTenant(context.Background(), args, productionTenantIO())
}

// coverage-ignore-end

// tenantArgs are the parsed arguments of one tenant command.
type tenantArgs struct {
	cmd, orgID, cacheDir string
	adopt, force         bool
}

// runTenant parses `COMMAND [flags] ORG_ID` and runs it. It returns the exit
// code: 0 on success, 1 on a refusal or failure, 2 on a usage error.
//
// The DSN never reaches the output: it is read from the environment, only
// [tenant.DescribeTarget] of it is printed, and a pgx error redacts its
// password.
func runTenant(ctx context.Context, args []string, tio tenantIO) int {
	a, code, ok := parseTenantArgs(args, tio.err)
	if !ok {
		return code
	}
	base := tio.getenv("RELA_DATABASE_URL")
	if base == "" {
		_, _ = fmt.Fprintln(tio.err, "tenant: set $RELA_DATABASE_URL to the tenant database")
		return 1
	}
	if a.cmd == "show" {
		return showTenant(ctx, a, base, tio)
	}
	p, err := tio.provisioner(base, tenant.AuditRoot(a.cacheDir))
	if err != nil {
		_, _ = fmt.Fprintf(tio.err, "tenant %s: %v\n", a.cmd, err)
		return 1
	}
	switch a.cmd {
	case "create":
		t, err := p.Create(ctx, a.orgID, a.adopt)
		recordLifecycle(tio, a, t.Schema, err)
		return report(tio, a, err, fmt.Sprintf("created org_id %q in schema %q; it is active", a.orgID, t.Schema))
	case "suspend":
		err := p.Suspend(ctx, a.orgID)
		recordLifecycle(tio, a, "", err)
		return report(tio, a, err,
			fmt.Sprintf("suspended org_id %q; requests are refused now, and open sessions close within %s",
				a.orgID, tenant.RevalidateInterval))
	case "resume":
		err := p.Resume(ctx, a.orgID)
		recordLifecycle(tio, a, "", err)
		return report(tio, a, err, fmt.Sprintf("resumed org_id %q", a.orgID))
	default:
		return eraseTenant(ctx, a, base, p, tio)
	}
}

// report prints the outcome of a lifecycle command and returns its exit code.
func report(tio tenantIO, a tenantArgs, err error, success string) int {
	if err != nil {
		if errors.Is(err, tenant.ErrSchemaExists) {
			err = fmt.Errorf("%w; if the schema is this org's, run create again with -adopt", err)
		}
		_, _ = fmt.Fprintf(tio.err, "tenant %s failed: %v\n", a.cmd, err)
		return 1
	}
	_, _ = fmt.Fprintln(tio.out, success)
	return 0
}

// parseTenantArgs parses the command line. ok is false when the command
// should exit with code at once.
func parseTenantArgs(args []string, stderr io.Writer) (a tenantArgs, code int, ok bool) {
	usage := func() { _, _ = io.WriteString(stderr, tenantUsage) }
	if len(args) == 0 {
		usage()
		return a, 2, false
	}
	switch args[0] {
	case "create", "suspend", "resume", "erase", "show":
		a.cmd = args[0]
	default:
		usage()
		return a, 2, false
	}
	fs := flag.NewFlagSet("tenant "+a.cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = usage
	projectDir := fs.String("project", ".", "Path to the rela project directory")
	if a.cmd == "create" {
		fs.BoolVar(&a.adopt, "adopt", false, "Take over a schema that already has the derived name")
	}
	if a.cmd == "erase" {
		fs.BoolVar(&a.force, "force", false, "Erase without asking to type the org id")
	}
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return a, 0, false
		}
		return a, 2, false
	}
	if fs.NArg() != 1 || fs.Arg(0) == "" {
		usage()
		return a, 2, false
	}
	a.orgID = fs.Arg(0)
	abs, err := filepath.Abs(*projectDir)
	if err == nil {
		a.cacheDir, err = appbuild.DiscoverCacheDir(abs)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tenant %s: find the project: %v\n", a.cmd, err)
		return a, 1, false
	}
	return a, 0, true
}

// showTenant prints orgID's derived schema and its status.
func showTenant(ctx context.Context, a tenantArgs, base string, tio tenantIO) int {
	schema, err := tenant.SchemaFor(a.orgID)
	if err != nil {
		_, _ = fmt.Fprintf(tio.err, "tenant show: %v\n", err)
		return 1
	}
	r, err := tio.reader(ctx, base)
	if err != nil {
		_, _ = fmt.Fprintf(tio.err, "tenant show: %v\n", err)
		return 1
	}
	defer func() { _ = r.Close() }()
	rec, err := r.Lookup(ctx, a.orgID)
	switch {
	case errors.Is(err, tenant.ErrUnknownTenant):
		_, _ = fmt.Fprintf(tio.out, "org_id %q: schema %q, no tenant\n", a.orgID, schema)
		return 0
	case err != nil:
		_, _ = fmt.Fprintf(tio.err, "tenant show: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(tio.out, "org_id %q: schema %q, %s%s\n", rec.OrgID, rec.Schema, rec.Status,
		done(rec.SchemaExists, "", " (the schema does not exist)"))
	return 0
}

// eraseTenant confirms and erases the tenant, and records the outcome.
func eraseTenant(ctx context.Context, a tenantArgs, base string, p lifecycle, tio tenantIO) int {
	schema, err := tenant.SchemaFor(a.orgID)
	if err != nil {
		_, _ = fmt.Fprintf(tio.err, "refusing to erase: %v\n", err)
		return 1
	}
	target := tenant.DescribeTarget(base)
	if !a.force && !confirmErasure(tio, a.orgID, schema, target) {
		_, _ = fmt.Fprintln(tio.err, "refusing to erase: the org id typed does not match")
		return 1
	}

	// Signals are caught only now, so Ctrl-C at the prompt still ends the
	// process at once.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	t, res, err := p.Erase(ctx, a.orgID)
	if t.Schema == "" {
		t = tenant.Tenant{OrgID: a.orgID, Schema: schema}
	}
	recordErasure(tio, a.cacheDir, t, res, err)
	if err != nil {
		switch {
		case errors.Is(err, tenant.ErrUnknownTenant):
			err = fmt.Errorf("%w; nothing was erased", err)
		case errors.Is(err, tenant.ErrTenantInUse):
			err = fmt.Errorf("%w: servers and rela --tenant processes close it within %s of the "+
				"status change; retry then, or stop the process that still holds it", err, tenant.RevalidateInterval)
		case res.SchemaDropped:
			err = fmt.Errorf("the schema was dropped, but %w; fix that and run the command again", err)
		}
		_, _ = fmt.Fprintf(tio.err, "erase failed: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(tio.out, "erased org_id %q in %s: schema %q %s, audit directory %s\n",
		t.OrgID, target, t.Schema, done(res.SchemaDropped, "dropped", "already absent"),
		done(res.AuditRemoved, "removed", "already absent"))
	return 0
}

// erasureLog is the host-level record of erasures, beside the per-tenant
// audit directories. It is not a tenant's, so erasing a tenant does not
// remove it, and it holds no tenant data: who erased which org, when, and
// what came of it.
const erasureLog = "tenant-erasures.jsonl"

// recordErasure appends one line to the erasure log and logs the outcome. A
// failure to write the record is reported but does not change the exit code:
// the erasure has already happened or failed.
func recordErasure(tio tenantIO, cacheDir string, t tenant.Tenant, res tenant.Erasure, eraseErr error) {
	operator := ""
	if tio.user != nil {
		operator = tio.user()
	}
	rec := map[string]any{
		"time":           time.Now().UTC().Format(time.RFC3339),
		"operator":       operator,
		"tool":           "rela-server tenant erase",
		"org_id":         t.OrgID,
		"schema":         t.Schema,
		"schema_dropped": res.SchemaDropped,
		"audit_removed":  res.AuditRemoved,
		"ok":             eraseErr == nil,
	}
	if eraseErr != nil {
		rec["error"] = eraseErr.Error()
		slog.Error("tenant erase failed", "org_id", t.OrgID, "schema", t.Schema,
			"schema_dropped", res.SchemaDropped, "error", eraseErr)
	} else {
		slog.Info("tenant erased", "org_id", t.OrgID, "schema", t.Schema,
			"schema_dropped", res.SchemaDropped, "audit_removed", res.AuditRemoved)
	}
	line, err := json.Marshal(rec)
	if err == nil {
		err = appendLine(filepath.Join(cacheDir, "audit", erasureLog), line)
	}
	if err != nil {
		_, _ = fmt.Fprintf(tio.err, "warning: the erasure was not recorded in %s: %v\n", erasureLog, err)
	}
}

// lifecycleLog is the host-level record of create, suspend and resume. Like
// the erasure log it sits beside the per-tenant audit directories and holds
// no tenant data.
const lifecycleLog = "tenant-lifecycle.jsonl"

// recordLifecycle appends one line to the lifecycle log for a create,
// suspend or resume attempt. schema is empty when the command does not
// report one. A failure to write the record is a warning only, as for
// recordErasure.
func recordLifecycle(tio tenantIO, a tenantArgs, schema string, cmdErr error) {
	operator := ""
	if tio.user != nil {
		operator = tio.user()
	}
	rec := map[string]any{
		"time":     time.Now().UTC().Format(time.RFC3339),
		"operator": operator,
		"tool":     "rela-server tenant " + a.cmd,
		"action":   a.cmd,
		"org_id":   a.orgID,
		"ok":       cmdErr == nil,
	}
	if schema != "" {
		rec["schema"] = schema
	}
	if a.cmd == "create" {
		rec["adopt"] = a.adopt
	}
	if cmdErr != nil {
		rec["error"] = cmdErr.Error()
		slog.Warn("tenant "+a.cmd+" failed", "org_id", a.orgID, "error", cmdErr)
	} else {
		slog.Info("tenant "+a.cmd, "org_id", a.orgID, "schema", schema)
	}
	line, err := json.Marshal(rec)
	if err == nil {
		err = appendLine(filepath.Join(a.cacheDir, "audit", lifecycleLog), line)
	}
	if err != nil {
		_, _ = fmt.Fprintf(tio.err, "warning: the %s was not recorded in %s: %v\n", a.cmd, lifecycleLog, err)
	}
}

// Permissions of the erasure log and its directory: the audit sinks' own.
const (
	erasureLogDirPerm  = 0o750
	erasureLogFilePerm = 0o600
)

func appendLine(path string, line []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), erasureLogDirPerm); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, erasureLogFilePerm)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// confirmErasure asks the operator to type the org id and reports whether
// the line read matches it exactly.
func confirmErasure(tio tenantIO, orgID, schema, target string) bool {
	_, _ = fmt.Fprintf(tio.out,
		"This permanently erases org_id %q: schema %q in %s, and its audit directory.\n"+
			"The tenant stops being served at once, and that cannot be undone.\n"+
			"Check first that no open or recently closed incident, NCR or dispute refers to\n"+
			"this tenant while its evaluation period is still running (PROCEDURE-u020).\n"+
			"Type the org id to confirm: ",
		orgID, schema, target)
	line, err := bufio.NewReader(tio.in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	return strings.TrimRight(line, "\r\n") == orgID
}

func done(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

// refuseExtraArgs refuses arguments left after the server's flags. The
// flag package stops at the first one and ignores the rest, so
// `rela-server -project DIR tenant erase ORG` would otherwise start the
// server instead of erasing.
func refuseExtraArgs(args []string) error {
	if len(args) == 0 {
		return nil
	}
	if args[0] == tenantCommand {
		return errors.New("the tenant command must come first: rela-server tenant COMMAND [-project DIR] ORG_ID")
	}
	return fmt.Errorf("unexpected argument %q", args[0])
}
