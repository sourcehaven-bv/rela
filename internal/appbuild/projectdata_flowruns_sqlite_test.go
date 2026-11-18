//go:build sqlite

package appbuild_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/flowrun/kvflowrun"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

const dataFlowsYAML = "flows:\n  review:\n    subject: doc\n    initial: open\n    states:\n      open: {}\n"

// fileRuns opens the run store a markdown project at root keeps under .rela/.
func fileRuns(t *testing.T, root string) *kvflowrun.Store {
	t.Helper()
	rooted, err := storage.NewRootedFS(storage.NewOsFS(), filepath.Join(root, ".rela"))
	if err != nil {
		t.Fatal(err)
	}
	runs, err := kvflowrun.New(state.NewFSKV(rooted), filelock.ProcessPrivate{})
	if err != nil {
		t.Fatal(err)
	}
	return runs
}

// writeFlowDataProject is writeDataProject with a flow and one unfinished run
// that has forked into a branch.
func writeFlowDataProject(t *testing.T) string {
	t.Helper()
	root := writeDataProject(t)
	writeFile(t, root, "flows.yaml", dataFlowsYAML)
	writeFile(t, root, "entities/_flow-runs/RUN-1.md", "---\nid: RUN-1\ntype: _flow-run\nflow: review\n---\n")
	writeFile(t, root, "relations/RUN-1--_carries--DOC-1.md", "---\nfrom: RUN-1\nrelation: _carries\nto: DOC-1\n---\n")
	ctx := context.Background()
	runs := fileRuns(t, root)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	created, err := runs.Create(ctx, flowrun.NewRun{
		ID: "RUN-1", Flow: "review", Version: "0123456789abcdef", Definition: []byte("states: {}\n"),
		Subject: entity.Ref{ID: "DOC-1", Face: entity.ImplicitFace}, Singleton: true, At: at,
		Instance: flow.Instance{State: "open", Generation: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	st := created.State
	st.Branches = []flowrun.Branch{{Name: "open", Instance: flow.Instance{State: "open", Generation: 2}}}
	if _, err := runs.Update(ctx, "RUN-1", created.Rev, st, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	return root
}

// checkRun asserts that runs holds RUN-1 with its branch and definition.
func checkRun(t *testing.T, where string, runs flowrun.Store) {
	t.Helper()
	ctx := context.Background()
	got, err := runs.Get(ctx, "RUN-1")
	if err != nil {
		t.Fatalf("%s: RUN-1: %v", where, err)
	}
	if got.Instance.State != "open" || !got.Singleton || len(got.Branches) != 1 || got.Branches[0].Name != "open" {
		t.Errorf("%s: RUN-1 = %+v; want its instance, singleton and branch", where, got)
	}
	if def, err := runs.Definition(ctx, "review", "0123456789abcdef"); err != nil || string(def) != "states: {}\n" {
		t.Errorf("%s: definition = %q, %v", where, def, err)
	}
}

// TestMarkdownData_FlowRunsRoundTrip pins that unfinished flow runs travel
// with the data: the import into an existing database keeps them
// (TKT-TWLN8UB), and the export writes them where the markdown project
// reads them (BUG-EWG0R90), branches included.
func TestMarkdownData_FlowRunsRoundTrip(t *testing.T) {
	ctx := context.Background()
	root := writeFlowDataProject(t)
	rep, err := importData(t, root, false, audit.NewMemory())
	if err != nil {
		t.Fatalf("import: %v (%v)", err, rep.Errors)
	}
	if rep.FlowRuns != 1 {
		t.Fatalf("import copied %d flow runs, want 1", rep.FlowRuns)
	}
	data, err := appbuild.OpenSQLiteData(ctx, filepath.Join(root, ".rela", "rela.db"))
	if err != nil {
		t.Fatal(err)
	}
	checkRun(t, "database", data.FlowRuns)
	if err = data.Close(); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "out")
	sum, err := appbuild.ExportMarkdownData(ctx, osFS(), projectAt(t, root), out)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if sum.FlowRuns != 1 || sum.Entities != 3 {
		t.Fatalf("export summary = %+v, want 3 entities and 1 flow run", sum)
	}
	checkRun(t, "export", fileRuns(t, out))

	// The export is a project again once its config sits beside it.
	writeFile(t, out, "schema.yaml", dataSchemaYAML)
	writeFile(t, out, "flows.yaml", dataFlowsYAML)
	if rep, err := importData(t, out, false, audit.NewMemory()); err != nil || rep.FlowRuns != 1 {
		t.Fatalf("re-import: %v, %d flow runs (%v)", err, rep.FlowRuns, rep.Errors)
	}
}
