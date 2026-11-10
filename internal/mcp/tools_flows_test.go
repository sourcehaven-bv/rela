package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowview"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// fakeFlowSteps records the calls the flow tools make and answers err.
type fakeFlowSteps struct {
	mu    sync.Mutex
	err   error
	calls []string
}

func (f *fakeFlowSteps) rec(format string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fmt.Sprintf(format, args...))
}

func (f *fakeFlowSteps) Tasks(_ context.Context, name string, page, perPage int) (flowview.Tasks, error) {
	f.rec("tasks %s %d %d", name, page, perPage)
	return flowview.Tasks{Tasks: []flowview.Task{{Run: "RUN-1"}}, Total: 1}, f.err
}

func (f *fakeFlowSteps) Run(_ context.Context, id flow.RunID) (flowview.Run, error) {
	f.rec("run %s", id)
	return flowview.Run{ID: string(id), Generation: 2}, f.err
}

func (f *fakeFlowSteps) Complete(_ context.Context, id flow.RunID, gen int64, props map[string]any) (
	flowview.Completed, error) {
	f.rec("complete %s %d %v", id, gen, props)
	return flowview.Completed{Moved: true, State: "done"}, f.err
}

func newFlowServer(t *testing.T, f FlowSteps) *Server {
	t.Helper()
	meta, st := makeTestFixture(t)
	srv, err := NewServer(newTestDeps(t, meta, st), "test",
		WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}), WithFlows(f))
	require.NoError(t, err)
	return srv
}

// TestFlowTools_Calls pins each flow tool's arguments and result.
func TestFlowTools_Calls(t *testing.T) {
	tests := []struct {
		name, tool, args string
		call             string
		wantErr          string
		want             string
	}{
		{name: "tasks", tool: "list_flow_tasks", args: `{}`, call: "tasks  1 50", want: `"run": "RUN-1"`},
		{name: "tasks, filtered and paged", tool: "list_flow_tasks", args: `{"flow":"onboarding","page":2,"per_page":999}`,
			call: "tasks onboarding 2 200"},
		{name: "show", tool: "show_flow_run", args: `{"run":"RUN-1"}`, call: "run RUN-1", want: `"generation": 2`},
		{name: "show without run", tool: "show_flow_run", args: `{}`, wantErr: "run"},
		{name: "complete", tool: "complete_flow_step",
			args: `{"run":"RUN-1","generation":2,"properties":{"documents":true}}`,
			call: "complete RUN-1 2 map[documents:true]", want: `"written": []`},
		{name: "complete without properties", tool: "complete_flow_step", args: `{"run":"RUN-1","generation":0}`,
			call: "complete RUN-1 0 map[]"},
		{name: "fractional generation", tool: "complete_flow_step", args: `{"run":"RUN-1","generation":1.5}`,
			wantErr: "generation"},
		{name: "no generation", tool: "complete_flow_step", args: `{"run":"RUN-1"}`, wantErr: "generation"},
		{name: "properties not an object", tool: "complete_flow_step",
			args: `{"run":"RUN-1","generation":1,"properties":[1]}`, wantErr: "properties"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeFlowSteps{}
			text, isErr := callTool(t, newFlowServer(t, fake), tc.tool, tc.args)
			if tc.wantErr != "" {
				require.True(t, isErr, text)
				require.Contains(t, text, tc.wantErr)
				require.Empty(t, fake.calls)
				return
			}
			require.False(t, isErr, text)
			require.Equal(t, []string{tc.call}, fake.calls)
			require.True(t, json.Valid([]byte(text)), text)
			if tc.want != "" {
				require.Contains(t, text, tc.want)
			}
		})
	}
}

// TestFlowTools_Errors pins that a refusal is a tool error, and that a
// stale step says how to recover.
func TestFlowTools_Errors(t *testing.T) {
	fake := &fakeFlowSteps{err: fmt.Errorf("%w: the run moved on", flowview.ErrConflict)}
	text, isErr := callTool(t, newFlowServer(t, fake), "complete_flow_step", `{"run":"RUN-1","generation":1}`)
	require.True(t, isErr)
	require.Contains(t, text, "show_flow_run")

	fake = &fakeFlowSteps{err: fmt.Errorf("%w: run", flowview.ErrNotFound)}
	text, isErr = callTool(t, newFlowServer(t, fake), "show_flow_run", `{"run":"RUN-1"}`)
	require.True(t, isErr)
	require.True(t, strings.HasPrefix(text, "not found"), text)
}

// TestFlowTools_AbsentWithoutFlows pins that a server without WithFlows,
// or with a nil service, has no flow tools.
func TestFlowTools_AbsentWithoutFlows(t *testing.T) {
	_, rpcErr := dispatch(t, newFlowServer(t, nil), "tools/call", `{"name":"list_flow_tasks","arguments":{}}`)
	require.NotNil(t, rpcErr)
	require.Contains(t, rpcErr.Message, "unknown tool")
}
