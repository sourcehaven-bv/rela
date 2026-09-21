package sqlitemigstate_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/datamigration/migstatetest"
	"github.com/Sourcehaven-BV/rela/internal/datamigration/sqlitemigstate"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

func TestConformance(t *testing.T) {
	migstatetest.RunAll(t, func(t *testing.T) datamigration.StateStore {
		t.Helper()
		db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{
			Path: filepath.Join(t.TempDir(), "rela.db"),
		})
		if err != nil {
			t.Fatalf("open database: %v", err)
		}
		t.Cleanup(func() { _ = db.Close() })

		st, err := sqlitemigstate.New(db.DB())
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		return st
	})
}

func TestNew_RejectsNilDB(t *testing.T) {
	st, err := sqlitemigstate.New(nil)
	if err == nil {
		t.Fatal("New(nil) must be rejected: a no-op store would replay every migration")
	}
	if st != nil {
		t.Errorf("New returned %v alongside an error, want nil", st)
	}
}
