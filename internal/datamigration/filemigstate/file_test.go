package filemigstate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/datamigration/filemigstate"
	"github.com/Sourcehaven-BV/rela/internal/datamigration/migstatetest"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func newStore(t *testing.T) (store *filemigstate.Store, root string) {
	t.Helper()
	root = t.TempDir()
	store, err := filemigstate.New(storage.NewOsFS(), root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return store, root
}

func TestConformance(t *testing.T) {
	migstatetest.RunAll(t, func(t *testing.T) datamigration.StateStore {
		t.Helper()
		st, _ := newStore(t)
		return st
	})
}

func TestNew_RejectsNilFSAndEmptyRoot(t *testing.T) {
	if _, err := filemigstate.New(nil, "/tmp"); err == nil {
		t.Error("New must reject a nil filesystem rather than substituting a no-op")
	}
	if _, err := filemigstate.New(storage.NewOsFS(), "  "); err == nil {
		t.Error("New must reject an empty root")
	}
}

// The file is committed, so it must land where git will see it — NOT under the
// gitignored .rela/. That placement is the whole point of the change.
func TestSave_WritesToCommittedPath(t *testing.T) {
	st, root := newStore(t)
	saveFixture(t, st)

	want := filepath.Join(root, "migrations", "applied.json")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected state at %s: %v", want, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".rela")); err == nil {
		t.Error("state must not be written under the gitignored .rela/")
	}
}

// It is read in diffs, so it must be indented and newline-terminated.
func TestSave_IsReviewableJSON(t *testing.T) {
	st, root := newStore(t)
	saveFixture(t, st)

	data, err := os.ReadFile(filepath.Join(root, "migrations", "applied.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "\n  ") {
		t.Error("state should be indented so a diff is reviewable")
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Error("state should end with a newline")
	}
}

// A corrupt file must ERROR rather than read as absent. Absent means
// "un-bootstrapped", which re-baselines the store against the live schema and
// strands every pending migration — turning a bad hand-edit into silent drift.
func TestLoad_CorruptFileIsAnErrorNotAnAbsence(t *testing.T) {
	st, root := newStore(t)
	writeRaw(t, root, "{ this is not json")

	got, err := st.Load(t.Context())
	if err == nil {
		t.Fatalf("corrupt state must error, got state %+v", got)
	}
	if got != nil {
		t.Error("corrupt state must not be reported as a usable state")
	}
}

// A typo'd key must be refused, not silently dropped: `appliedd` parsed as
// nothing would make a fully migrated store look like it had run nothing.
func TestLoad_UnknownFieldIsRefused(t *testing.T) {
	st, root := newStore(t)
	writeRaw(t, root, `{"format_version":1,"appliedd":[],"projection":{},"updated_at":"2026-09-19T00:00:00Z"}`)

	if _, err := st.Load(t.Context()); err == nil {
		t.Fatal("an unknown field must be refused so a typo is visible")
	}
}

// The applied-list is hand-editable and its names reach a path join, so a
// traversal attempt must be rejected on the way IN from storage.
func TestLoad_RejectsUnsafeMigrationName(t *testing.T) {
	st, root := newStore(t)
	writeRaw(t, root, `{"format_version":1,`+
		`"applied":[{"name":"../../etc/passwd","applied_at":"2026-09-19T00:00:00Z"}],`+
		`"projection":{},"updated_at":"2026-09-19T00:00:00Z"}`)

	_, err := st.Load(t.Context())
	if err == nil {
		t.Fatal("a name that is not a safe migration filename must be refused")
	}
	if !strings.Contains(err.Error(), "applied-list entry") {
		t.Errorf("error should name the offending list, got: %v", err)
	}
}

// A state from a newer rela must be refused rather than re-baselined. This is
// the downgrade guard: without it, an older binary sees state it cannot parse,
// treats the store as fresh, and skips every pending migration.
func TestLoad_RefusesStateFromTheFuture(t *testing.T) {
	st, root := newStore(t)
	writeRaw(t, root, `{"format_version":9999,"applied":[],`+
		`"projection":{},"updated_at":"2026-09-19T00:00:00Z"}`)

	_, err := st.Load(t.Context())
	if err == nil {
		t.Fatal("state from a newer format version must be refused")
	}
	if !strings.Contains(err.Error(), "upgrade rela") {
		t.Errorf("error should tell the operator to upgrade, got: %v", err)
	}
}

func saveFixture(t *testing.T, st *filemigstate.Store) {
	t.Helper()
	s, err := datamigration.NewState(fixtureProjection(t), []datamigration.AppliedEntry{
		{Name: "20260919143022-backfill-owner.yaml", AppliedAt: fixedTime()},
	}, fixedTime())
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	if err := st.Save(t.Context(), s); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func writeRaw(t *testing.T, root, body string) {
	t.Helper()
	dir := filepath.Join(root, "migrations")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "applied.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func fixedTime() time.Time {
	return time.Date(2026, 9, 19, 14, 30, 22, 0, time.UTC)
}

func fixtureProjection(t *testing.T) metamodel.ShapeProjection {
	t.Helper()
	m := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"task": {Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string", Required: true},
			}},
		},
	}
	return m.ShapeProjection()
}
