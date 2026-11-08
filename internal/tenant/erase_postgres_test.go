//go:build postgres

package tenant_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// TestErase_RemovesOnlyThatTenant is the erasure property of TKT-TNERAS on a
// real database: erasing tenant A drops A's schema and audit directory, and
// leaves tenant B's schema, rows and audit records as they were.
//
// A private database, so that `public` holds no table another test left
// there (the erasure refuses then, see TestErase_RefusesWhenPublicHasTables).
func TestErase_RemovesOnlyThatTenant(t *testing.T) {
	root := writeIsolationProject(t)
	db := backendtest.Database(t)
	dsnA, schemaA := backendtest.SchemaIn(t, db)
	dsnB, schemaB := backendtest.SchemaIn(t, db)
	a := tenant.Tenant{OrgID: "org-a", Schema: schemaA, DSN: dsnA}
	b := tenant.Tenant{OrgID: "org-b", Schema: schemaB, DSN: dsnB}
	resolver, err := tenant.NewMapResolver([]tenant.Tenant{a, b})
	if err != nil {
		t.Fatalf("NewMapResolver: %v", err)
	}
	open := isolationOpener(t, root)
	reg, err := tenant.NewRegistry(resolver, open, 4)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	ctx := t.Context()
	for _, tn := range []tenant.Tenant{a, b} {
		lease, aerr := reg.Acquire(ctx, tn.OrgID)
		if aerr != nil {
			t.Fatalf("Acquire(%s): %v", tn.OrgID, aerr)
		}
		id := "TKT-" + tn.OrgID
		if err = lease.Value().Services.Store().CreateEntity(ctx, &entity.Entity{
			ID: id, Type: "ticket", Properties: map[string]any{"title": tn.OrgID},
		}); err != nil {
			t.Fatalf("create in %s: %v", tn.OrgID, err)
		}
		lease.Value().Services.Audit().Record(audit.Record{
			Op: audit.OpCreateEntity, Subject: &audit.Subject{Kind: "entity", Type: "ticket", ID: id},
		})
		lease.Release()
	}
	auditRoot := tenant.AuditRoot(filepath.Join(root, ".rela"))

	// While a server has the tenant open, erasure is refused.
	if _, err = tenant.Erase(ctx, a, auditRoot); !errors.Is(err, tenant.ErrTenantInUse) {
		t.Fatalf("Erase of an open tenant error = %v, want ErrTenantInUse", err)
	}
	if !schemaExists(t, dsnB, schemaA) {
		t.Fatal("a refused erase dropped the schema")
	}
	if err = reg.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	res, err := tenant.Erase(ctx, a, auditRoot)
	if err != nil {
		t.Fatalf("Erase: %v", err)
	}
	if !res.SchemaDropped || !res.AuditRemoved {
		t.Errorf("Erase = %+v, want the schema dropped and the audit directory removed", res)
	}
	if schemaExists(t, dsnB, schemaA) {
		t.Error("tenant A's schema survived its erasure")
	}
	if _, err = os.Stat(filepath.Join(auditRoot, schemaA)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("tenant A's audit directory survived its erasure: %v", err)
	}

	// Tenant B is untouched: schema, rows and audit records.
	if !schemaExists(t, dsnB, schemaB) {
		t.Fatal("erasing tenant A dropped tenant B's schema")
	}
	files, err := filepath.Glob(filepath.Join(auditRoot, schemaB, "*.jsonl"))
	if err != nil || len(files) != 1 {
		t.Errorf("tenant B's audit files = %v (%v), want one", files, err)
	}
	optB, err := open(ctx, b)
	if err != nil {
		t.Fatalf("reopen tenant B: %v", err)
	}
	assertVisible(t, optB.Services, "TKT-org-b", "org-b")
	if err = optB.Close(); err != nil {
		t.Errorf("close tenant B: %v", err)
	}

	// The erased tenant cannot be opened again: its connection would land
	// in public, and the opener refuses that.
	if optA, oerr := open(ctx, a); oerr == nil {
		_ = optA.Close()
		t.Fatal("an erased tenant opened again")
	}

	// A second run finds nothing left and succeeds.
	res, err = tenant.Erase(ctx, a, auditRoot)
	if err != nil {
		t.Fatalf("second Erase: %v", err)
	}
	if res.SchemaDropped || res.AuditRemoved {
		t.Errorf("second Erase = %+v, want nothing removed", res)
	}
}

// TestErase_RefusesOpenDuringErasure pins the other side of the lock: while
// an erasure holds the tenant's lock, a server cannot open the tenant.
func TestErase_RefusesOpenDuringErasure(t *testing.T) {
	root := writeIsolationProject(t)
	dsn, schema := backendtest.SchemaDSN(t)
	conn, err := pgx.Connect(t.Context(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	// The same lock dropSchema takes.
	var locked bool
	if err := conn.QueryRow(t.Context(), "SELECT pg_try_advisory_lock($1, hashtext($2))",
		int32(0x72656c61), schema).Scan(&locked); err != nil || !locked {
		t.Fatalf("take the erasure lock: %v (locked %v)", err, locked)
	}
	open := isolationOpener(t, root)
	if opt, err := open(t.Context(), tenant.Tenant{OrgID: "org-a", Schema: schema, DSN: dsn}); err == nil {
		_ = opt.Close()
		t.Fatal("a tenant opened while its erasure held the lock")
	}
}

// TestErase_RefusesWhenPublicHasTables pins the guard for a server that was
// not restarted and lost its lock: with a table in `public`, its pools would
// fall back to that table once the tenant's schema is gone.
func TestErase_RefusesWhenPublicHasTables(t *testing.T) {
	db := backendtest.Database(t)
	dsn, schema := backendtest.SchemaIn(t, db)
	conn, err := pgx.Connect(t.Context(), db)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close(context.Background()) }()
	if _, err = conn.Exec(t.Context(), "CREATE TABLE public.entities (id text)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	_, err = tenant.Erase(t.Context(), tenant.Tenant{OrgID: "org-a", Schema: schema, DSN: dsn}, t.TempDir())
	if !errors.Is(err, tenant.ErrPublicHasTables) {
		t.Fatalf("Erase error = %v, want ErrPublicHasTables", err)
	}
	if !schemaExists(t, db, schema) {
		t.Error("the refused erasure dropped the schema")
	}
}

// TestErase_AuditFailures pins the two ways removing the audit directory can
// fail after the drop: a symlinked directory is refused rather than reported
// removed, and an error leaves Erasure saying the schema is gone so the
// command can tell the operator.
func TestErase_AuditFailures(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		db := backendtest.Database(t)
		dsn, schema := backendtest.SchemaIn(t, db)
		auditRoot := t.TempDir()
		target := t.TempDir()
		if err := os.WriteFile(filepath.Join(target, "a.jsonl"), []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(auditRoot, schema)); err != nil {
			t.Fatal(err)
		}
		res, err := tenant.Erase(t.Context(), tenant.Tenant{OrgID: "org-a", Schema: schema, DSN: dsn}, auditRoot)
		if err == nil {
			t.Fatal("Erase reported success over a symlinked audit directory")
		}
		if !res.SchemaDropped || res.AuditRemoved {
			t.Errorf("Erase = %+v, want the schema dropped and the audit directory not removed", res)
		}
		if _, err := os.Stat(filepath.Join(target, "a.jsonl")); err != nil {
			t.Errorf("the symlink target was touched: %v", err)
		}
	})
	t.Run("unremovable", func(t *testing.T) {
		db := backendtest.Database(t)
		dsn, schema := backendtest.SchemaIn(t, db)
		auditRoot := t.TempDir()
		dir := filepath.Join(auditRoot, schema)
		if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "sub", "a.jsonl"), []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		// A directory whose entries cannot be removed.
		if err := os.Chmod(filepath.Join(dir, "sub"), 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "sub"), 0o750) })
		res, err := tenant.Erase(t.Context(), tenant.Tenant{OrgID: "org-a", Schema: schema, DSN: dsn}, auditRoot)
		if err == nil {
			t.Skip("this user can remove a read-only directory's entries (running as root?)")
		}
		if !res.SchemaDropped || res.AuditRemoved {
			t.Errorf("Erase = %+v, want the schema dropped and the audit directory not removed", res)
		}
		if schemaExists(t, db, schema) {
			t.Error("the schema survived")
		}
	})
}

// TestErase_LostLockFailsClosed covers the case the lock cannot: the
// connection holding it dies while the tenant stays open, and the operator
// erases without restarting the server. The open store must then fail, and
// never read from another schema.
func TestErase_LostLockFailsClosed(t *testing.T) {
	root := writeIsolationProject(t)
	db := backendtest.Database(t)
	dsn, schema := backendtest.SchemaIn(t, db)
	open := isolationOpener(t, root)
	opt, err := open(t.Context(), tenant.Tenant{OrgID: "org-a", Schema: schema, DSN: dsn})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = opt.Close() })
	if err = opt.Services.Store().CreateEntity(t.Context(), &entity.Entity{
		ID: "TKT-A1", Type: "ticket", Properties: map[string]any{"title": "a"},
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	conn, err := pgx.Connect(t.Context(), db)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close(context.Background()) }()
	var killed int
	if err := conn.QueryRow(t.Context(), `SELECT count(pg_terminate_backend(pid)) FROM pg_locks
		WHERE locktype = 'advisory' AND classid = $1::bigint::oid AND objsubid = 2`,
		int64(0x72656c61)).Scan(&killed); err != nil || killed != 1 {
		t.Fatalf("terminate the lock holder: %v (terminated %d)", err, killed)
	}

	if _, err := tenant.Erase(t.Context(), tenant.Tenant{OrgID: "org-a", Schema: schema, DSN: dsn},
		t.TempDir()); err != nil {
		t.Fatalf("Erase after the lock was lost: %v", err)
	}
	if got, err := opt.Services.Store().GetEntity(t.Context(), entity.Ref{ID: "TKT-A1"}); err == nil {
		t.Fatalf("the open store still read %+v after its schema was dropped", got)
	}
}

func schemaExists(t *testing.T, dsn, schema string) bool {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close(context.Background()) }()
	var ok bool
	if err := conn.QueryRow(t.Context(),
		"SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1)", schema).Scan(&ok); err != nil {
		t.Fatalf("look up schema: %v", err)
	}
	return ok
}
