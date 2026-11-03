package datamigration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// The DSN gate mirrors pgmigstate's: skip without a DSN, unless
// RELA_TEST_DATABASE_REQUIRED says this run must reach PostgreSQL, in which
// case a missing DSN fails rather than passing as a skip.
const (
	readsTestDBEnv    = "RELA_TEST_DATABASE_URL"
	readsRequireDBEnv = "RELA_TEST_DATABASE_REQUIRED"
)

var (
	readsAdminOnce sync.Once
	readsAdminPool *pgxpool.Pool
	errReadsAdmin  error
	readsSchemaSeq atomic.Int64
)

// openSQLiteStore opens a sqlitestore over a fresh database file.
func openSQLiteStore(t *testing.T) store.Store {
	t.Helper()
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{
		Path: filepath.Join(t.TempDir(), "rela.db"),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	st, err := sqlitestore.New(context.Background(), db)
	if err != nil {
		t.Fatalf("sqlitestore.New: %v", err)
	}
	return st
}

// pgBackend is the postgres case: a pgstore in a private, migrated schema.
func pgBackend() []backendCase {
	return []backendCase{{"pgstore", openPGStore}}
}

func openPGStore(t *testing.T) store.Store {
	t.Helper()
	dsn := os.Getenv(readsTestDBEnv)
	if dsn == "" {
		if v := os.Getenv(readsRequireDBEnv); v != "" && v != "0" && !strings.EqualFold(v, "false") {
			t.Fatalf("%s is set but %s is empty: the postgres leg must run", readsRequireDBEnv, readsTestDBEnv)
		}
		t.Skipf("%s not set", readsTestDBEnv)
	}
	ctx := context.Background()
	readsAdminOnce.Do(func() { readsAdminPool, errReadsAdmin = pgxpool.New(ctx, dsn) })
	if errReadsAdmin != nil {
		t.Fatalf("admin pool: %v", errReadsAdmin)
	}
	schema := fmt.Sprintf("relamigreads_%d_%d", os.Getpid(), readsSchemaSeq.Add(1))
	quoted := `"` + schema + `"`
	if _, err := readsAdminPool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { _, _ = readsAdminPool.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE") })

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	cfg.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err = pgstore.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st, err := pgstore.New(pool)
	if err != nil {
		t.Fatalf("pgstore.New: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}
