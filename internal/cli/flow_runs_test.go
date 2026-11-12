package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/output"
)

type fakeFlowControl struct {
	started, cancelled string
	runs               []appbuild.FlowRunHeader
	err                error
}

func (f *fakeFlowControl) Start(_ context.Context, name, addr string) (flow.RunID, error) {
	f.started = name + " " + addr
	return "FLR-1", f.err
}

func (f *fakeFlowControl) Cancel(_ context.Context, id flow.RunID) error {
	f.cancelled = string(id)
	return f.err
}

func (f *fakeFlowControl) Runs(context.Context, string) ([]appbuild.FlowRunHeader, error) {
	return f.runs, f.err
}

// TestFlowRunCmds pins the output of `rela flow start|runs|cancel`, and
// that a project without flows says so.
func TestFlowRunCmds(t *testing.T) {
	ctx := context.Background()
	buf := captureOut(t)
	fake := &fakeFlowControl{runs: []appbuild.FlowRunHeader{
		{ID: "FLR-1", Flow: "onboarding", State: "collect", Status: "input_required", Started: "2026-11-07T10:00:00Z"},
	}}
	svc := &flowServices{Ctl: fake}

	require.NoError(t, (&FlowStartCmd{Flow: "onboarding", Subject: "PER-1"}).Run(ctx, svc))
	require.Equal(t, "onboarding PER-1", fake.started)
	require.Contains(t, buf.String(), "Started FLR-1")

	buf.Reset()
	require.NoError(t, (&FlowRunsCmd{}).Run(ctx, svc))
	require.Contains(t, buf.String(), "input_required")

	buf.Reset()
	require.NoError(t, (&FlowCancelCmd{ID: "FLR-1"}).Run(ctx, svc))
	require.Equal(t, "FLR-1", fake.cancelled)
	require.Contains(t, buf.String(), "Cancelled FLR-1")

	fake.runs = nil
	buf.Reset()
	require.NoError(t, (&FlowRunsCmd{}).Run(ctx, svc))
	require.Contains(t, buf.String(), "No runs.")

	boom := errors.New("boom")
	fake.err = boom
	err := (&FlowStartCmd{}).Run(ctx, svc)
	require.ErrorIs(t, err, boom)
	require.ErrorContains(t, err, "started FLR-1", "a run created before the failure is named")
	require.ErrorContains(t, err, "will be retried while rela-server", "the message says what retries the run")
	require.ErrorContains(t, err, "rela flow cancel FLR-1", "and how to give up on it")
	require.ErrorIs(t, (&FlowCancelCmd{}).Run(ctx, svc), boom)
	require.ErrorIs(t, (&FlowRunsCmd{}).Run(ctx, svc), boom)

	none := &flowServices{}
	require.ErrorIs(t, (&FlowStartCmd{}).Run(ctx, none), errNoFlows)
	require.ErrorIs(t, (&FlowCancelCmd{}).Run(ctx, none), errNoFlows)
	require.ErrorIs(t, (&FlowRunsCmd{}).Run(ctx, none), errNoFlows)
}

// TestFlowRunCmds_JSON pins the JSON shapes: the started id, and an empty
// list as [] rather than null.
func TestFlowRunCmds_JSON(t *testing.T) {
	ctx := context.Background()
	prev := out
	buf := &bytes.Buffer{}
	out = output.NewWithWriter(buf, output.FormatJSON)
	t.Cleanup(func() { out = prev })
	svc := &flowServices{Ctl: &fakeFlowControl{}}

	require.NoError(t, (&FlowStartCmd{Flow: "f", Subject: "S-1"}).Run(ctx, svc))
	var started map[string]string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &started))
	require.Equal(t, "FLR-1", started["id"])

	buf.Reset()
	require.NoError(t, (&FlowRunsCmd{}).Run(ctx, svc))
	require.JSONEq(t, "[]", buf.String())
}
