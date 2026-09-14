package sqlitestore_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// TestOpenReadThenNew exercises the ordering the separation exists for: open
// the database, read config OUT of it, and only then build the store.
//
// Without this, the arrangement's reason for existing is unverified — a
// regression that re-coupled opening to store construction would still pass
// every other test in this package.
func TestOpenReadThenNew(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cfg.db")

	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: path})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Stage 1: the database is usable with no store in existence. This is
	// where loading schema.yaml out of project_files happens.
	const want = "entity_types:\n  - ticket\n"
	if _, writeErr := db.DB().ExecContext(ctx,
		`INSERT INTO project_files (path, content, updated_at) VALUES (?, ?, ?)`,
		"schema.yaml", []byte(want), time.Now().UTC().Format(time.RFC3339Nano),
	); writeErr != nil {
		t.Fatalf("write config before the store exists: %v", writeErr)
	}
	var got []byte
	if readErr := db.DB().QueryRowContext(ctx,
		`SELECT content FROM project_files WHERE path = ?`, "schema.yaml",
	).Scan(&got); readErr != nil {
		t.Fatalf("read config before the store exists: %v", readErr)
	}
	if string(got) != want {
		t.Errorf("config round-trip = %q, want %q", got, want)
	}

	// Stage 2: the store is built on that same connection and works normally.
	st, err := sqlitestore.New(db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = st.Close() }()

	if err := st.CreateEntity(ctx, &entity.Entity{ID: "T-1", Type: "ticket"}); err != nil {
		t.Fatalf("CreateEntity after New: %v", err)
	}
	if _, err := st.GetEntity(ctx, "T-1"); err != nil {
		t.Errorf("GetEntity after New: %v", err)
	}

	// And the config written before the store existed is still readable
	// through the connection the store now owns.
	if readErr := db.DB().QueryRowContext(ctx,
		`SELECT content FROM project_files WHERE path = ?`, "schema.yaml",
	).Scan(&got); readErr != nil {
		t.Errorf("read config after the store exists: %v", readErr)
	}
}

// TestNewRejectsNilDB pins the constructor contract: required collaborators
// are rejected up front rather than deferred to a nil-pointer panic on the
// first query.
func TestNewRejectsNilDB(t *testing.T) {
	st, err := sqlitestore.New(nil)
	if err == nil {
		t.Fatal("New(nil) should be rejected")
	}
	if st != nil {
		t.Errorf("New returned %v alongside an error, want nil", st)
	}
}
