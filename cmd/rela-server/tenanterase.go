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

const tenantUsage = `usage: rela-server tenant erase [-project DIR] [-force] ORG_ID

Erases one tenant: drops its PostgreSQL schema with everything in it and
removes its audit directory. The tenant must be marked "erase: true" in
tenants.yaml, and every server must have been restarted on that file first.
Without -force the command asks you to type the org id to confirm.

Before erasing, check that no open or recently closed incident, NCR or
dispute refers to the tenant while its evaluation period is still running
(PROCEDURE-u020). The erasure deletes the tenant's audit log at once.

The base DSN comes from base_dsn in tenants.yaml or from $RELA_DATABASE_URL.
`

// eraseIO is where the erase command reads its confirmation and writes its
// prompt and result. Passed in so the tests can drive it.
type eraseIO struct {
	in       io.Reader
	out, err io.Writer
	getenv   func(string) string
	// user names the operator in the erasure record.
	user func() string
}

// runTenantCommand runs `rela-server tenant ...` and returns the exit code.
//
// coverage-ignore-func: process entry point; eraseTenant carries the logic.
// coverage-ignore-start: main-or-wiring: reads the process stdio and signals
func runTenantCommand(args []string) int {
	return eraseTenant(context.Background(), args, eraseIO{
		in: os.Stdin, out: os.Stdout, err: os.Stderr, getenv: os.Getenv,
		user: func() string { return os.Getenv("USER") },
	})
}

// coverage-ignore-end

// eraseTenant parses `erase [-project DIR] [-force] ORG_ID`, confirms, and
// erases the tenant. It returns the exit code: 0 on success, 1 on a refusal
// or failure, 2 on a usage error.
//
// The DSN never reaches the output. It is read from tenants.yaml or the
// environment, and a pgx error redacts its password.
func eraseTenant(ctx context.Context, args []string, eio eraseIO) int {
	if len(args) == 0 || args[0] != "erase" {
		_, _ = io.WriteString(eio.err, tenantUsage)
		return 2
	}
	fs := flag.NewFlagSet("tenant erase", flag.ContinueOnError)
	fs.SetOutput(eio.err)
	fs.Usage = func() { _, _ = io.WriteString(eio.err, tenantUsage) }
	projectDir := fs.String("project", ".", "Path to the rela project directory")
	force := fs.Bool("force", false, "Erase without asking to type the org id")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 1 || fs.Arg(0) == "" {
		fs.Usage()
		return 2
	}
	orgID := fs.Arg(0)

	t, cacheDir, err := erasureTarget(*projectDir, orgID, eio.getenv)
	if err != nil {
		_, _ = fmt.Fprintf(eio.err, "refusing to erase: %v\n", err)
		return 1
	}
	target := tenant.DescribeTarget(t.DSN)
	if !*force && !confirmErasure(eio, t, target) {
		_, _ = fmt.Fprintln(eio.err, "refusing to erase: the org id typed does not match")
		return 1
	}

	// Signals are caught only now, so Ctrl-C at the prompt still ends the
	// process at once.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	res, err := tenant.Erase(ctx, t, tenant.AuditRoot(cacheDir))
	recordErasure(eio, cacheDir, t, res, err)
	if err != nil {
		switch {
		case errors.Is(err, tenant.ErrTenantInUse):
			err = fmt.Errorf("%w: restart every server on the marked tenants.yaml, then retry", err)
		case res.SchemaDropped:
			err = fmt.Errorf("the schema was dropped, but %w; fix that and run the command again", err)
		}
		_, _ = fmt.Fprintf(eio.err, "erase failed: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(eio.out, "erased org_id %q in %s: schema %q %s, audit directory %s\n",
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
func recordErasure(eio eraseIO, cacheDir string, t tenant.Tenant, res tenant.Erasure, eraseErr error) {
	operator := ""
	if eio.user != nil {
		operator = eio.user()
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
		_, _ = fmt.Fprintf(eio.err, "warning: the erasure was not recorded in %s: %v\n", erasureLog, err)
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

// erasureTarget loads the tenant map in projectDir and returns the tenant to
// erase and the project's cache directory, which holds the audit logs.
func erasureTarget(projectDir, orgID string, getenv func(string) string) (tenant.Tenant, string, error) {
	path, err := tenantsPath(projectDir)
	if err != nil {
		return tenant.Tenant{}, "", err
	}
	if path == "" {
		return tenant.Tenant{}, "", fmt.Errorf("no %s in %s", tenantsFile, projectDir)
	}
	cfg, err := tenant.LoadConfig(path)
	if err != nil {
		return tenant.Tenant{}, "", err
	}
	if cfg.BaseDSN == "" {
		cfg.BaseDSN = getenv("RELA_DATABASE_URL")
	}
	t, err := cfg.ErasureTarget(orgID)
	if err != nil {
		if errors.Is(err, tenant.ErrNotMarkedForErasure) {
			err = fmt.Errorf("%w: set erase: true on it in %s and restart every server first", err, tenantsFile)
		}
		return tenant.Tenant{}, "", err
	}
	abs, err := filepath.Abs(projectDir)
	if err != nil {
		return tenant.Tenant{}, "", fmt.Errorf("invalid project dir: %w", err)
	}
	cacheDir, err := appbuild.DiscoverCacheDir(abs)
	if err != nil {
		return tenant.Tenant{}, "", fmt.Errorf("find the project: %w", err)
	}
	return t, cacheDir, nil
}

// confirmErasure asks the operator to type the org id and reports whether
// the line read matches it exactly.
func confirmErasure(eio eraseIO, t tenant.Tenant, target string) bool {
	_, _ = fmt.Fprintf(eio.out,
		"This permanently erases org_id %q: schema %q in %s, and its audit directory.\n"+
			"Check first that no open or recently closed incident, NCR or dispute refers to\n"+
			"this tenant while its evaluation period is still running (PROCEDURE-u020).\n"+
			"Type the org id to confirm: ",
		t.OrgID, t.Schema, target)
	line, err := bufio.NewReader(eio.in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	return strings.TrimRight(line, "\r\n") == t.OrgID
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
		return errors.New("the tenant command must come first: rela-server tenant erase [-project DIR] ORG_ID")
	}
	return fmt.Errorf("unexpected argument %q", args[0])
}
