package statesql_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/state/statesql"
	"github.com/Sourcehaven-BV/rela/internal/state/statetest"
)

// TestConformance is what actually proves this is a state.KV.
//
// The package deliberately does not name the state.KV interface (importing
// internal/state would invert the dependency), so there is no compile-time
// assertion to lean on. The conformance suite is the stronger check anyway: it
// pins BEHAVIOUR — that a missing key is os.ErrNotExist-compatible, that
// delete is idempotent, that an empty value round-trips distinctly from an
// absent one — none of which a type assertion would catch.
//
// Wrapped in ValidatedKV exactly as the wiring site wraps it, so the
// key-rejection cases exercise the same composition production uses.
func TestConformance(t *testing.T) {
	statetest.RunAll(t, newKV)
}

func newKV(tb testing.TB) state.KV {
	tb.Helper()
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{
		Path: filepath.Join(tb.TempDir(), "state.db"),
	})
	if err != nil {
		tb.Fatalf("open database: %v", err)
	}
	tb.Cleanup(func() { _ = db.Close() })

	raw, err := statesql.New(db.DB())
	if err != nil {
		tb.Fatalf("New: %v", err)
	}
	kv, err := state.NewValidatedKV(raw)
	if err != nil {
		tb.Fatalf("NewValidatedKV: %v", err)
	}
	return kv
}

func TestNewRejectsNilDB(t *testing.T) {
	kv, err := statesql.New(nil)
	if err == nil {
		t.Fatal("New(nil) should be rejected")
	}
	if kv != nil {
		t.Errorf("New returned %v alongside an error, want nil", kv)
	}
}

func TestPutRejectsOversizeValue(t *testing.T) {
	kv := newKV(t)
	// A value over the cap must be REJECTED, not stored short: a silently
	// truncated cached render would be served as if it were valid.
	huge := make([]byte, (32<<20)+1)
	if err := kv.Put(context.Background(), "render/big", huge); err == nil {
		t.Error("Put of an oversize value should be rejected")
	}
}
