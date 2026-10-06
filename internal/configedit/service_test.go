package configedit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

const testSchema = `version: "1.0"
types:
  status:
    values: [open, doing, done]
entities:
  task:
    label: Task
    id_prefix: TSK
    properties:
      title:
        type: string
        required: true
      state:
        type: status
      notes:
        type: string
`

const testDataEntry = `app:
  name: Tasks
`

type memFiles map[string][]byte

func (m memFiles) ReadFile(p string) ([]byte, error) {
	b, ok := m[p]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return b, nil
}
func (m memFiles) WriteFile(p string, data []byte, _ os.FileMode) error { m[p] = data; return nil }
func (m memFiles) Remove(p string) error                                { delete(m, p); return nil }
func (m memFiles) MkdirAll(string, os.FileMode) error                   { return nil }

type parseValidator struct{}

func (parseValidator) Validate(schema, _ []byte) (*metamodel.Metamodel, error) {
	m, err := metamodel.Parse(schema)
	if err != nil {
		return nil, &ValidationError{Problems: []string{err.Error()}}
	}
	return m, nil
}

// counts maps "type.property=value" (value empty: any) to a count.
type counts map[string]int

func (c counts) Count(_ context.Context, t, p, v string) (int, error) {
	return c[t+"."+p+"="+v], nil
}

type readyState struct {
	err      error
	adoptErr error // Adopt fails with this, else with err
	adopted  *int
}

func (r readyState) Ready(context.Context, *metamodel.Metamodel) error { return r.err }
func (r readyState) Adopt(context.Context, *metamodel.Metamodel) error {
	if r.adoptErr != nil {
		return r.adoptErr
	}
	if r.adopted != nil {
		*r.adopted++
	}
	return r.err
}

type fakeActivator struct {
	files      memFiles
	prepareErr error
	migrateErr error
	pauseErr   error
	paused     int
	activated  []string // schema each activation served
	migrated   int
}

func (a *fakeActivator) Pause(context.Context) (func(), error) {
	if a.pauseErr != nil {
		return nil, a.pauseErr
	}
	a.paused++
	return func() { a.paused-- }, nil
}
func (a *fakeActivator) Prepare(context.Context) (Candidate, error) {
	if a.prepareErr != nil {
		return nil, a.prepareErr
	}
	return &fakeCandidate{a: a}, nil
}

type fakeCandidate struct{ a *fakeActivator }

func (c *fakeCandidate) Migrate(context.Context) (int, error) {
	c.a.migrated++
	return 3, c.a.migrateErr
}
func (c *fakeCandidate) Activate() {
	c.a.activated = append(c.a.activated, string(c.a.files["schema.yaml"]))
}
func (c *fakeCandidate) Discard() {}

type harness struct {
	svc   *Service
	files memFiles
	act   *fakeActivator
	audit *audit.Memory
}

func newHarness(t *testing.T, c counts, ready error) *harness {
	t.Helper()
	files := memFiles{"schema.yaml": []byte(testSchema), "data-entry.yaml": []byte(testDataEntry)}
	act := &fakeActivator{files: files}
	mem := audit.NewMemory()
	svc, err := NewService(Deps{
		Files: files, Validator: parseValidator{}, Counter: c,
		Migrations: readyState{err: ready}, Activator: act, Audit: mem,
	})
	if err != nil {
		t.Fatal(err)
	}
	svc.now = func() time.Time { return time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC) }
	return &harness{svc: svc, files: files, act: act, audit: mem}
}

// draft edits the schema tree with fn and wraps it in a Draft.
func (h *harness) draft(t *testing.T, fn func(schema *Map)) *Draft {
	t.Helper()
	snap, err := h.svc.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	schema := snap.Schema.(*Map)
	fn(schema)
	raw, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	return &Draft{BaseVersion: snap.Version, Schema: raw}
}

func taskProps(schema *Map) *Map {
	e, _ := schema.Get("entities")
	task, _ := e.(*Map).Get("task")
	props, _ := task.(*Map).Get("properties")
	return props.(*Map)
}

func setStatusValues(vals ...any) func(*Map) {
	return func(schema *Map) {
		types, _ := schema.Get("types")
		st, _ := types.(*Map).Get("status")
		st.(*Map).Set("values", vals)
	}
}

func renameNotes(schema *Map) {
	props := taskProps(schema)
	for i, k := range props.Keys {
		if k == "notes" {
			props.Keys[i] = "remarks"
		}
	}
}

func codes(ps []Problem) string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Code)
	}
	return strings.Join(out, ",")
}

func TestSave_LabelChangeNeedsNoMigration(t *testing.T) {
	h := newHarness(t, counts{}, nil)
	d := h.draft(t, func(s *Map) {
		e, _ := s.Get("entities")
		task, _ := e.(*Map).Get("task")
		task.(*Map).Set("label", "Job")
	})
	res, err := h.svc.Save(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Saved || res.Migration != nil || len(res.Problems) > 0 {
		t.Fatalf("result = %+v", res)
	}
	if !strings.Contains(string(h.files["schema.yaml"]), "label: Job") || len(h.act.activated) != 1 {
		t.Fatalf("not written and activated:\n%s", h.files["schema.yaml"])
	}
	if recs := h.audit.Records(); len(recs) != 1 || recs[0].Op != audit.OpConfigEdit {
		t.Fatalf("audit = %+v", recs)
	}
}

func TestSave_RemovedValueInUse(t *testing.T) {
	h := newHarness(t, counts{"task.state=doing": 4}, nil)
	d := h.draft(t, setStatusValues("open", "done"))

	res, err := h.svc.Preview(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	if codes(res.Problems) != "value_in_use" || res.Problems[0].Count != 4 {
		t.Fatalf("problems = %+v", res.Problems)
	}

	d.Values = []ValueMapping{{EntityType: "task", Property: "state", From: "doing", To: "done"}}
	d.MigrationTitle = "Fold doing into done"
	res, err = h.svc.Save(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Saved || res.Migration == nil || len(res.Migration.Steps) != 1 || h.act.migrated != 1 {
		t.Fatalf("result = %+v", res)
	}
	name := "migrations/" + res.Migration.File
	if _, err := datamigration.ParseFile(res.Migration.File, h.files[name]); err != nil {
		t.Fatalf("written migration does not parse: %v", err)
	}
}

func TestSave_UnusedValueRemovalNeedsNoChoice(t *testing.T) {
	h := newHarness(t, counts{}, nil)
	res, err := h.svc.Save(context.Background(), h.draft(t, setStatusValues("open", "done")))
	if err != nil || !res.Saved {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestSave_Rename(t *testing.T) {
	h := newHarness(t, counts{"task.notes=": 2}, nil)
	d := h.draft(t, renameNotes)
	d.Renames = []Rename{{EntityType: "task", From: "notes", To: "remarks"}}
	res, err := h.svc.Save(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Saved || res.Migration == nil || res.Migration.Steps[0].Kind != "rename_property" ||
		res.Migration.Steps[0].Count != 2 {

		t.Fatalf("result = %+v", res)
	}
}

func TestSave_Refusals(t *testing.T) {
	tests := []struct {
		name   string
		acl    string
		ready  error
		edit   func(*Map)
		rename []Rename
		want   string
	}{
		{"locked key", "", nil, func(s *Map) {
			e, _ := s.Get("entities")
			task, _ := e.(*Map).Get("task")
			task.(*Map).Set("scan_cmd", "rm -rf /")
		}, nil, "locked"},
		{"invalid schema", "", nil, func(s *Map) {
			taskProps(s).Set("notes", &Map{Keys: []string{"type"}, Values: []any{"no-such-type"}})
		}, nil, "invalid"},
		{"acl names removed property", "roles:\n  r:\n    redact: {task: [notes]}\n", nil,
			renameNotes, []Rename{{EntityType: "task", From: "notes", To: "remarks"}}, "acl_reference"},
		{"bad rename", "", nil, renameNotes, []Rename{{EntityType: "task", From: "nope", To: "remarks"}}, "rename"},
		{"migration state", "", errors.New("pending migrations"), renameNotes,
			[]Rename{{EntityType: "task", From: "notes", To: "remarks"}}, "migration_state"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, counts{"task.notes=": 1}, tc.ready)
			if tc.acl != "" {
				h.files["acl.yaml"] = []byte(tc.acl)
			}
			d := h.draft(t, tc.edit)
			d.Renames = tc.rename
			res, err := h.svc.Save(context.Background(), d)
			if err != nil {
				t.Fatal(err)
			}
			if res.Saved || !strings.Contains(codes(res.Problems), tc.want) {
				t.Fatalf("want %s, got %+v", tc.want, res)
			}
			if string(h.files["schema.yaml"]) != testSchema || len(h.act.activated) != 0 {
				t.Fatal("a refused save changed something")
			}
		})
	}
}

func TestSave_Conflict(t *testing.T) {
	h := newHarness(t, counts{}, nil)
	d := h.draft(t, setStatusValues("open", "done"))
	h.files["schema.yaml"] = append([]byte("# edited on disk\n"), h.files["schema.yaml"]...)
	if _, err := h.svc.Save(context.Background(), d); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v", err)
	}
}

func TestSave_PrepareFailureRestoresFiles(t *testing.T) {
	h := newHarness(t, counts{"task.notes=": 1}, nil)
	h.act.prepareErr = &ValidationError{Problems: []string{"boom"}}
	d := h.draft(t, renameNotes)
	d.Renames = []Rename{{EntityType: "task", From: "notes", To: "remarks"}}
	res, err := h.svc.Save(context.Background(), d)
	if err != nil || res.Saved || codes(res.Problems) != "invalid" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if string(h.files["schema.yaml"]) != testSchema || len(h.files) != 2 || h.act.paused != 0 {
		t.Fatalf("files not restored: %v", h.files)
	}
}

func TestSave_MigrationNotStartedRestoresFiles(t *testing.T) {
	for _, tc := range []struct {
		name    string
		migrate error
		want    error
	}{
		{"lock held", fmt.Errorf("run: %w", datamigration.ErrLockHeld), ErrBusy},
		{"not started", fmt.Errorf("%w: resolve: no path", ErrMigrationNotStarted), ErrMigrationNotStarted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, counts{"task.notes=": 1}, nil)
			h.act.migrateErr = tc.migrate
			d := h.draft(t, renameNotes)
			d.Renames = []Rename{{EntityType: "task", From: "notes", To: "remarks"}}
			if _, err := h.svc.Save(context.Background(), d); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v", err)
			}
			if string(h.files["schema.yaml"]) != testSchema || len(h.files) != 2 || len(h.act.activated) != 0 {
				t.Fatal("files not restored")
			}
		})
	}
}

// TestSave_AdoptsBeforeWriting pins that a save with a migration records the
// current shape before writing anything, and writes nothing when it cannot.
func TestSave_AdoptsBeforeWriting(t *testing.T) {
	h := newHarness(t, counts{"task.notes=": 1}, nil)
	adopted := 0
	h.svc.deps.Migrations = readyState{adopted: &adopted}
	d := h.draft(t, renameNotes)
	d.Renames = []Rename{{EntityType: "task", From: "notes", To: "remarks"}}
	if res, err := h.svc.Save(context.Background(), d); err != nil || !res.Saved || adopted != 1 {
		t.Fatalf("res=%+v err=%v adopted=%d", res, err, adopted)
	}

	h = newHarness(t, counts{"task.notes=": 1}, nil)
	h.svc.deps.Migrations = readyState{adoptErr: errors.New("a migration is pending")}
	d = h.draft(t, renameNotes)
	d.Renames = []Rename{{EntityType: "task", From: "notes", To: "remarks"}}
	res, err := h.svc.Save(context.Background(), d)
	if err != nil || res.Saved || codes(res.Problems) != "migration_state" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if string(h.files["schema.yaml"]) != testSchema || len(h.files) != 2 || h.act.paused != 0 {
		t.Fatal("files written")
	}
}

func TestSave_MigrationFailureRollsForward(t *testing.T) {
	h := newHarness(t, counts{"task.notes=": 1}, nil)
	h.act.migrateErr = errors.New("disk full")
	d := h.draft(t, renameNotes)
	d.Renames = []Rename{{EntityType: "task", From: "notes", To: "remarks"}}
	res, err := h.svc.Save(context.Background(), d)
	if err != nil || !res.Saved || !res.Incomplete || len(h.act.activated) != 1 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	h.act.migrateErr = nil
	h.svc.deps.Migrations = readyState{err: errors.New("a migration file has not run")}
	if err := h.svc.Retry(context.Background()); err != nil || h.act.migrated != 2 || len(h.act.activated) != 2 {
		t.Fatalf("retry: %v", err)
	}

	// With nothing left to finish, Retry neither migrates nor rebuilds.
	h.svc.deps.Migrations = readyState{}
	if err := h.svc.Retry(context.Background()); err != nil || h.act.migrated != 2 || len(h.act.activated) != 2 {
		t.Fatalf("idle retry: %v", err)
	}
}

// TestSave_BusyWritesRefuse pins that a save is refused, and writes nothing,
// when the running server cannot hold its writes.
func TestSave_BusyWritesRefuse(t *testing.T) {
	h := newHarness(t, nil, nil)
	h.act.pauseErr = errors.New("2 writes still running")
	_, err := h.svc.Save(context.Background(), h.draft(t, func(schema *Map) {
		taskProps(schema).Set("due", &Map{Keys: []string{"type"}, Values: []any{"date"}})
	}))
	if !errors.Is(err, ErrBusy) || string(h.files["schema.yaml"]) != testSchema {
		t.Fatalf("err=%v", err)
	}
}

func TestNewService_RejectsNil(t *testing.T) {
	if _, err := NewService(Deps{}); err == nil {
		t.Fatal("want error")
	}
}
