package fsimport_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/flowrun/kvflowrun"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// flowSource adds a flows.yaml, one unfinished run in the file tier's run
// store and the run's header and subject edge.
func flowSource(t *testing.T) string {
	t.Helper()
	source := baseProject(t)
	writeTree(t, source, map[string]string{
		"flows.yaml": "flows:\n  review:\n    subject: item\n    initial: open\n" +
			"    states:\n      open: {}\n",
		"entities/_flow-runs/RUN-1.md":         "---\nid: RUN-1\ntype: _flow-run\nflow: review\n---\n",
		"relations/RUN-1--_carries--ITEM-1.md": "---\nfrom: RUN-1\nrelation: _carries\nto: ITEM-1\n---\n",
	})
	rooted, err := storage.NewRootedFS(storage.NewOsFS(), filepath.Join(source, ".rela"))
	require.NoError(t, err)
	runs, err := kvflowrun.New(state.NewFSKV(rooted), filelock.ProcessPrivate{})
	require.NoError(t, err)
	_, err = runs.Create(context.Background(), flowrun.NewRun{
		ID: "RUN-1", Flow: "review", Version: "0123456789abcdef", Definition: []byte("states: {}\n"),
		Subject: entity.Ref{ID: "ITEM-1", Face: entity.ImplicitFace}, Singleton: true,
		At:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Instance: flow.Instance{State: "open", Generation: 1},
		Pending:  &flowrun.Pending{State: "open", Generation: 1},
	})
	require.NoError(t, err)
	return source
}

// TestRun_CopiesFlowRuns: an unfinished run reaches the target with its
// definition, pending mark and header (TKT-9IT3LN8).
func TestRun_CopiesFlowRuns(t *testing.T) {
	ctx := context.Background()
	source := flowSource(t)
	b := newMemBackend(t)
	runs, err := kvflowrun.New(b.target.State, filelock.ProcessPrivate{})
	require.NoError(t, err)
	b.target.FlowRuns = runs

	rep, err := run(t, source, filepath.Join(filepath.Dir(source), "target"), b)
	require.NoError(t, err, "errors: %v", rep.Errors)
	assert.Equal(t, 1, rep.FlowRuns)

	got, err := runs.Get(ctx, "RUN-1")
	require.NoError(t, err)
	assert.Equal(t, "open", got.Instance.State)
	assert.True(t, got.Singleton)
	require.NotNil(t, got.Pending)
	def, err := runs.Definition(ctx, "review", "0123456789abcdef")
	require.NoError(t, err)
	assert.Equal(t, "states: {}\n", string(def))

	hdr, err := b.target.Store.GetEntity(ctx, entity.Ref{ID: "RUN-1"})
	require.NoError(t, err, "the header is read by the schema plus _flow-run")
	assert.Equal(t, entity.FlowRunType, hdr.Type)
}

// TestRun_RefusesFlowRunsWithoutRunStore: a target that cannot take runs
// refuses a source with unfinished ones rather than copying headers no
// state moves.
func TestRun_RefusesFlowRunsWithoutRunStore(t *testing.T) {
	source := flowSource(t)
	rep, err := run(t, source, filepath.Join(filepath.Dir(source), "target"), newMemBackend(t))
	require.Error(t, err)
	assert.Contains(t, strings.Join(rep.Errors, "\n"), "1 unfinished flow run(s)")
}

// TestRun_ReportsFlowRunsWithoutFlows: unfinished runs whose source no
// longer declares flows fail the import with the reason, rather than being
// skipped as copied.
func TestRun_ReportsFlowRunsWithoutFlows(t *testing.T) {
	source := flowSource(t)
	writeTree(t, source, map[string]string{"flows.yaml": "flows: {}\n"})
	b := newMemBackend(t)
	runs, err := kvflowrun.New(b.target.State, filelock.ProcessPrivate{})
	require.NoError(t, err)
	b.target.FlowRuns = runs

	rep, err := run(t, source, filepath.Join(filepath.Dir(source), "target"), b)
	require.Error(t, err)
	assert.Contains(t, strings.Join(rep.Errors, "\n"), "1 unfinished flow run(s) but its flows.yaml declares no flows")
	assert.Zero(t, rep.FlowRuns)
}
