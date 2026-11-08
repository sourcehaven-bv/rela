//go:build postgres

package backendtest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
)

// buildName labels this build in skip and failure messages.
const buildName = "postgres"

var (
	adminPoolOnce sync.Once
	adminPool     *pgxpool.Pool
	errAdminPool  error
	schemaCounter atomic.Int64
)

// Options returns a DSN option pointing at a private, migrated schema created
// for this test and dropped when it finishes. Skips (or fails — see the package
// doc) when no database is configured.
//
// Isolation is per-schema, matching the pgstore suite: the store opens its own
// transactions and emits watcher events after commit, so a wrapping rollback
// would break both. A fresh schema also keeps rela_seq per-test and lets
// subtests run in parallel.
//
// Migrations are NOT run here. appbuild's postgres recipe opens the store via
// pgstore.Open, which migrates on first open — so letting it do that is both
// less duplication and a slightly wider test: it exercises the auto-migrate
// path the real binary takes on a fresh database.
func Options(tb TB) []appbuild.Option {
	tb.Helper()
	dsn := scopedDSN(tb)
	if dsn == "" {
		return nil // unreachable: scopedDSN skips or fails first.
	}
	return []appbuild.Option{appbuild.WithDatabaseURL(dsn)}
}

// Env returns the environment a test must set for a code path that reads the
// DSN from the process environment rather than accepting an option —
// appbuild.Discover, and therefore cli.newMCPServices. The caller applies it
// with t.Setenv so it is undone automatically.
func Env(tb TB) map[string]string {
	tb.Helper()
	dsn := scopedDSN(tb)
	if dsn == "" {
		return nil // unreachable: scopedDSN skips or fails first.
	}
	return map[string]string{"RELA_DATABASE_URL": dsn}
}

// DSN returns a connection string for a private, migrated schema, for a test
// that builds an appbuild.Config by hand.
//
// appbuild.New deliberately reads the DSN from Config.DatabaseURL and ignores
// WithDatabaseURL — a caller assembling a Config has already decided where the
// data lives — so [Options] is inert on that path and the field must be set
// directly. Returns "" on the fs/memory builds, where Config.DatabaseURL is
// unused.
func DSN(tb TB) string {
	tb.Helper()
	return scopedDSN(tb)
}

// SchemaDSN is [DSN] that also returns the name of the schema the DSN is
// pinned to, for a test that must state which schema a connection lands in.
func SchemaDSN(tb TB) (dsn, schema string) {
	tb.Helper()
	dsn = scopedDSN(tb)
	if dsn == "" {
		return "", ""
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		tb.Fatalf("parse scoped DSN: %v", err)
		return "", ""
	}
	schema, _, _ = strings.Cut(cfg.RuntimeParams["search_path"], ",")
	return dsn, schema
}

// Database creates a private database for a test that must control what the
// whole database holds, such as whether `public` has tables, and drops it
// when the test ends. It returns a DSN for the database on the default
// search_path. Use [SchemaIn] for schemas in it.
func Database(tb TB) string {
	tb.Helper()
	base := Getenv(testDBEnv)
	if base == "" {
		missingDSN(tb, Getenv)
		return ""
	}
	admin := adminConn(tb, base)
	if admin == nil {
		return ""
	}
	ctx := context.Background()
	name := fmt.Sprintf("relawiring_db_%d_%d", os.Getpid(), schemaCounter.Add(1))
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+quoteIdent(name)); err != nil {
		tb.Fatalf("create database %s: %v", name, err)
		return ""
	}
	tb.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE "+quoteIdent(name)+" WITH (FORCE)")
	})
	dsn, err := dsnFor(base, name, "")
	if err != nil {
		tb.Fatalf("build database DSN: %v", err)
		return ""
	}
	return dsn
}

// SchemaIn creates a schema in the database db names and returns a DSN
// pinned to it, with the schema's name. The schema goes when the database
// does.
func SchemaIn(tb TB, db string) (dsn, schema string) {
	tb.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, db)
	if err != nil {
		tb.Fatalf("connect: %v", err)
		return "", ""
	}
	defer func() { _ = conn.Close(ctx) }()
	schema = fmt.Sprintf("relawiring_%d_%d", os.Getpid(), schemaCounter.Add(1))
	if _, err = conn.Exec(ctx, "CREATE SCHEMA "+quoteIdent(schema)); err != nil {
		tb.Fatalf("create schema %s: %v", schema, err)
		return "", ""
	}
	cfg, err := pgx.ParseConfig(db)
	if err != nil {
		tb.Fatalf("parse DSN: %v", err)
		return "", ""
	}
	if dsn, err = dsnFor(db, cfg.Database, schema); err != nil {
		tb.Fatalf("build schema DSN: %v", err)
		return "", ""
	}
	return dsn, schema
}

// scopedDSN creates the per-test schema and returns a DSN pinned to it.
func scopedDSN(tb TB) string {
	tb.Helper()
	base := Getenv(testDBEnv)
	if base == "" {
		missingDSN(tb, Getenv)
		return ""
	}

	admin := adminConn(tb, base)
	if admin == nil {
		return ""
	}

	ctx := context.Background()
	schema := fmt.Sprintf("relawiring_%d_%d", os.Getpid(), schemaCounter.Add(1))
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quoteIdent(schema)); err != nil {
		tb.Fatalf("create schema %s: %v", schema, err)
		return ""
	}
	tb.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+quoteIdent(schema)+" CASCADE")
	})

	dsn, err := dsnWithSearchPath(base, schema)
	if err != nil {
		tb.Fatalf("build scoped DSN: %v", err)
		return ""
	}
	return dsn
}

// adminConn returns a process-wide pool on the default search_path, used only
// to CREATE and DROP the per-test schemas.
func adminConn(tb TB, dsn string) *pgxpool.Pool {
	tb.Helper()
	adminPoolOnce.Do(func() {
		adminPool, errAdminPool = pgxpool.New(context.Background(), dsn)
	})
	if errAdminPool != nil {
		tb.Fatalf("connect to %s: %v", testDBEnv, errAdminPool)
		return nil
	}
	return adminPool
}

// dsnWithSearchPath pins search_path to the private schema, keeping public on
// the path for pg_trgm (the search backend's operators live there).
//
// The setting is threaded through the DSN rather than a pgxpool.Config because
// appbuild's recipe takes a DSN string and builds its own pool — the seam that
// exists so appbuild owns the pool's lifecycle.
//
// Built by re-serializing the parsed config rather than appending "?search_path="
// to the caller's string: pgx accepts both URL and key/value ("host=x dbname=y")
// DSNs, and query-string surgery silently produces garbage for the latter.
func dsnWithSearchPath(base, schema string) (string, error) {
	cfg, err := pgx.ParseConfig(base)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", testDBEnv, err)
	}
	return dsnFor(base, cfg.Database, schema)
}

// dsnFor rebuilds base for database, with search_path pinned to schema and
// public when schema is not empty.
func dsnFor(base, database, schema string) (string, error) {
	cfg, err := pgx.ParseConfig(base)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", testDBEnv, err)
	}
	kv := []string{
		"host=" + cfg.Host,
		fmt.Sprintf("port=%d", cfg.Port),
		"user=" + cfg.User,
		"dbname=" + database,
	}
	if schema != "" {
		kv = append(kv, "search_path="+schema+",public")
	}
	if cfg.Password != "" {
		kv = append(kv, "password="+cfg.Password)
	}
	// TLSConfig is nil exactly when the DSN disabled it; preserve that, since
	// the CI container speaks plaintext and would reject a negotiated upgrade.
	if cfg.TLSConfig == nil {
		kv = append(kv, "sslmode=disable")
	}
	return strings.Join(kv, " "), nil
}

// quoteIdent renders a generated schema name as a quoted identifier. The names
// are built from a PID and a counter so they cannot contain a quote; the
// doubling is belt-and-braces against a future caller-supplied name.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
