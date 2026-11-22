package dataentry

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowview"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// fakeFlows records the calls the flow routes make and answers err.
type fakeFlows struct {
	err   error
	calls []string
	// startErr fails Startable alone.
	startErr error
}

func (f *fakeFlows) rec(format string, args ...any) {
	f.calls = append(f.calls, fmt.Sprintf(format, args...))
}

func (f *fakeFlows) Flows() []flowview.Flow {
	f.rec("flows")
	return []flowview.Flow{{Name: "onboarding"}}
}

func (f *fakeFlows) RunList(_ context.Context, name, status string, page, perPage int) (flowview.Runs, error) {
	f.rec("list %s %s %d %d", name, status, page, perPage)
	return flowview.Runs{Runs: []flowview.Run{{ID: "RUN-1"}}, Total: 1}, f.err
}

func (f *fakeFlows) SubjectRuns(_ context.Context, addr string) ([]flowview.Run, error) {
	f.rec("subject %s", addr)
	return nil, f.err
}

func (f *fakeFlows) Startable(_ context.Context, addr string) ([]string, error) {
	f.rec("startable %s", addr)
	if err := cmp.Or(f.startErr, f.err); err != nil {
		return nil, err
	}
	return []string{"onboarding"}, nil
}

func (f *fakeFlows) Run(_ context.Context, id flow.RunID) (flowview.Run, error) {
	f.rec("run %s", id)
	return flowview.Run{ID: string(id)}, f.err
}

func (f *fakeFlows) Start(_ context.Context, name, addr string) (flow.RunID, error) {
	f.rec("start %s %s", name, addr)
	return "RUN-9", f.err
}

func (f *fakeFlows) Complete(_ context.Context, id flow.RunID, gen int64, props map[string]any) (flowview.Completed,
	error) {
	f.rec("complete %s %d %v", id, gen, props)
	return flowview.Completed{Moved: true}, f.err
}

func (f *fakeFlows) Cancel(_ context.Context, id flow.RunID) error {
	f.rec("cancel %s", id)
	return f.err
}

func (f *fakeFlows) Retry(_ context.Context, id flow.RunID) error {
	f.rec("retry %s", id)
	return f.err
}

func (f *fakeFlows) Tasks(_ context.Context, name string, page, perPage int) (flowview.Tasks, error) {
	f.rec("tasks %s %d %d", name, page, perPage)
	return flowview.Tasks{}, f.err
}

func (f *fakeFlows) FailedTasks(_ context.Context, name string) ([]flowview.Task, bool, error) {
	f.rec("failed %s", name)
	return nil, false, f.err
}

func flowRequest(t *testing.T, app *App, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	w := httptest.NewRecorder()
	app.flows.handleV1Flows(w, r)
	return w
}

// TestFlowsAPI_Routes pins each route's dispatch, arguments and status.
func TestFlowsAPI_Routes(t *testing.T) {
	tests := []struct {
		name, method, path, body string
		want                     int
		call                     string
	}{
		{"flows", http.MethodGet, "/api/v1/_flows", "", http.StatusOK, "flows"},
		{"tasks", http.MethodGet, "/api/v1/_flows/tasks?flow=onboarding&page=2&per_page=10", "", http.StatusOK,
			"tasks onboarding 2 10; failed onboarding"},
		{"run list", http.MethodGet, "/api/v1/_flows/runs?flow=f&status=running", "", http.StatusOK,
			"list f running 1 25"},
		{"subject runs", http.MethodGet, "/api/v1/_flows/runs?subject=PER-1", "", http.StatusOK,
			"subject PER-1; startable PER-1"},
		{"start", http.MethodPost, "/api/v1/_flows/runs", `{"flow":"onboarding","subject":"PER-1"}`,
			http.StatusCreated, "start onboarding PER-1"},
		{"start without subject", http.MethodPost, "/api/v1/_flows/runs", `{"flow":"onboarding"}`,
			http.StatusBadRequest, ""},
		{"start, unknown field", http.MethodPost, "/api/v1/_flows/runs", `{"flow":"f","subject":"s","x":1}`,
			http.StatusBadRequest, ""},
		{"run", http.MethodGet, "/api/v1/_flows/runs/RUN-1", "", http.StatusOK, "run RUN-1"},
		{"complete", http.MethodPost, "/api/v1/_flows/runs/RUN-1/_complete",
			`{"generation":3,"properties":{"documents":true}}`, http.StatusOK, "complete RUN-1 3 map[documents:true]"},
		{"cancel", http.MethodPost, "/api/v1/_flows/runs/RUN-1/_cancel", "", http.StatusNoContent, "cancel RUN-1"},
		{"retry", http.MethodPost, "/api/v1/_flows/runs/RUN-1/_retry", "", http.StatusNoContent, "retry RUN-1"},
		{"unknown op", http.MethodPost, "/api/v1/_flows/runs/RUN-1/_nope", "", http.StatusNotFound, ""},
		{"unknown path", http.MethodGet, "/api/v1/_flows/nope/a/b/c", "", http.StatusNotFound, ""},
		{"wrong method", http.MethodDelete, "/api/v1/_flows/runs", "", http.StatusMethodNotAllowed, ""},
		{"get on an op", http.MethodGet, "/api/v1/_flows/runs/RUN-1/_cancel", "", http.StatusMethodNotAllowed, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newHandlerTestApp(t)
			fake := &fakeFlows{}
			require.NoError(t, SetFlows(app, fake))
			w := flowRequest(t, app, tc.method, tc.path, tc.body)
			require.Equal(t, tc.want, w.Code, w.Body.String())
			if tc.call == "" {
				require.Empty(t, fake.calls)
				return
			}
			require.Equal(t, tc.call, strings.Join(fake.calls, "; "))
		})
	}
}

// TestFlowsAPI_SubjectRunsStartable pins that the runs on one entity carry
// the flows the caller may start on it.
func TestFlowsAPI_SubjectRunsStartable(t *testing.T) {
	app := newHandlerTestApp(t)
	require.NoError(t, SetFlows(app, &fakeFlows{}))
	w := flowRequest(t, app, http.MethodGet, "/api/v1/_flows/runs?subject=PER-1", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var got flowview.SubjectRuns
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, []string{"onboarding"}, got.Startable)
	require.Equal(t, []flowview.Run{}, got.Runs.Runs)

	// A failed Startable fails the answer; it never sends the runs alone.
	require.NoError(t, SetFlows(app, &fakeFlows{startErr: flowview.ErrNotFound}))
	w = flowRequest(t, app, http.MethodGet, "/api/v1/_flows/runs?subject=PER-1", "")
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// TestFlowsAPI_ErrorStatus pins the status each kind of flow error maps to.
// A hidden run answers the uniform 404 and names nothing.
func TestFlowsAPI_ErrorStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"not found", fmt.Errorf("%w: run RUN-1 is secret", flowview.ErrNotFound), http.StatusNotFound},
		{"conflict", fmt.Errorf("%w: moved on", flowview.ErrConflict), http.StatusConflict},
		{"forbidden", fmt.Errorf("%w: needs role hr", flowview.ErrForbidden), http.StatusForbidden},
		{"invalid", fmt.Errorf("%w: writes", flowview.ErrInvalid), http.StatusUnprocessableEntity},
		{"validation", &entitymanager.ValidationError{Errors: []*metamodel.ValidationError{{Property: "x"}}},
			http.StatusUnprocessableEntity},
		{"acl denied", &acl.ForbiddenError{Decision: acl.Decision{RuleKind: "role"}}, http.StatusForbidden},
		{"no identity", acl.ErrNoIdentity, http.StatusUnauthorized},
		{"other", errors.New("backend: table flow_runs is gone"), http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newHandlerTestApp(t)
			require.NoError(t, SetFlows(app, &fakeFlows{err: tc.err}))
			w := flowRequest(t, app, http.MethodPost, "/api/v1/_flows/runs/RUN-1/_complete",
				`{"generation":1,"properties":{}}`)
			require.Equal(t, tc.want, w.Code, w.Body.String())
			require.NotContains(t, w.Body.String(), "secret")
			require.NotContains(t, w.Body.String(), "flow_runs", "a backend error stays in the log")
		})
	}
}

// TestFlowsAPI_NotWired pins the 404 of every route without flows, and
// that SetFlows refuses nil.
func TestFlowsAPI_NotWired(t *testing.T) {
	app := newHandlerTestApp(t)
	for _, path := range []string{"/api/v1/_flows", "/api/v1/_flows/runs/RUN-1"} {
		w := doRequest(t, app, http.MethodGet, path)
		require.Equal(t, http.StatusNotFound, w.Code)
		require.True(t, json.Valid(w.Body.Bytes()))
	}
	require.Error(t, SetFlows(app, nil))
}

// TestSchemaCapabilities_Flows pins capabilities.flows: the SPA asks for
// GET /_flows only when the schema says the project has flows, so a project
// without them gets no 404 on every page.
func TestSchemaCapabilities_Flows(t *testing.T) {
	flowsCapability := func(app *App) bool {
		t.Helper()
		rec := httptest.NewRecorder()
		app.schemaAPI.handleV1Schema(rec, httptest.NewRequest(http.MethodGet, "/api/v1/_schema", http.NoBody))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var got struct {
			Capabilities struct {
				Flows *bool `json:"flows"`
			} `json:"capabilities"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		require.NotNil(t, got.Capabilities.Flows, "capabilities.flows is always present")
		return *got.Capabilities.Flows
	}

	app := newHandlerTestApp(t)
	require.False(t, flowsCapability(app), "no flows declared")

	require.NoError(t, SetFlows(app, &fakeFlows{}))
	require.True(t, flowsCapability(app), "flows declared")
}

// TestFlowsAPI_BodyLimit pins the 413 for a body past the limit.
func TestFlowsAPI_BodyLimit(t *testing.T) {
	app := newHandlerTestApp(t)
	fake := &fakeFlows{}
	require.NoError(t, SetFlows(app, fake))
	body := `{"generation":1,"properties":{"x":"` + strings.Repeat("a", flowBodyLimit) + `"}}`
	w := flowRequest(t, app, http.MethodPost, "/api/v1/_flows/runs/RUN-1/_complete", body)
	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	require.Empty(t, fake.calls)
}

// TestFlowsAPI_SubjectRunsHiddenPanel pins that `entity_panels.flow_runs`
// withholds the runs and startable flows on the server, per space and per
// permission, and leaves the run list alone.
func TestFlowsAPI_SubjectRunsHiddenPanel(t *testing.T) {
	cases := []struct {
		name, path string
		wantCalls  string
	}{
		{"hidden in the space", "/api/v1/_flows/runs?subject=PER-1&space=portal", ""},
		{"shown in another space", "/api/v1/_flows/runs?subject=PER-1&space=office", "subject PER-1; startable PER-1"},
		{"run list unaffected", "/api/v1/_flows/runs?space=portal", "list   1 25"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newHandlerTestApp(t)
			cur := app.State()
			next := *cur
			cfg := *cur.Cfg
			cfg.Spaces = []dataentryconfig.Space{
				{ID: "portal", Label: "Portal",
					EntityPanels: dataentryconfig.EntityPanelsConfig{FlowRuns: dataentryconfig.ChromeToggle{Off: true}}},
				{ID: "office", Label: "Office"},
			}
			next.Cfg = &cfg
			app.schema.Publish(&next)
			fake := &fakeFlows{}
			require.NoError(t, SetFlows(app, fake))
			w := flowRequest(t, app, http.MethodGet, tc.path, "")
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			require.Equal(t, tc.wantCalls, strings.Join(fake.calls, "; "))
			if tc.wantCalls == "" {
				require.JSONEq(t, `{"runs":[],"total":0,"truncated":false,"startable":[]}`, w.Body.String())
			}
		})
	}
}

// TestFlowsAPI_SubjectRunsPanelPermission pins the permission form against a
// real policy: the holder gets the runs, a principal without the permission
// gets none, and with no acl.yaml the gated panel shows.
func TestFlowsAPI_SubjectRunsPanelPermission(t *testing.T) {
	cases := []struct {
		name      string
		policy    bool
		principal string // with policy: alice holds admin:read, bob does not
		wantCalls string
	}{
		{"holder", true, "alice", "subject PER-1; startable PER-1"},
		{"non-holder", true, "bob", ""},
		{"no acl.yaml", false, "", "subject PER-1; startable PER-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			cur := app.State()
			next := *cur
			cfg := *cur.Cfg
			cfg.EntityPanels.FlowRuns = dataentryconfig.ChromeToggle{Permission: "admin:read"}
			next.Cfg = &cfg
			app.schema.Publish(&next)
			ctx := t.Context()
			if tc.policy {
				d := mustNewACL(t, gatedNavPolicy(), app.store)
				setTestACL(t, app, d)
				ctx = gateCtxFor(principalCtx(tc.principal), t, d)
			} else {
				setTestACL(t, app, acl.NopACL{})
			}
			fake := &fakeFlows{}
			require.NoError(t, SetFlows(app, fake))
			r := httptest.NewRequest(http.MethodGet, "/api/v1/_flows/runs?subject=PER-1", http.NoBody).WithContext(ctx)
			w := httptest.NewRecorder()
			app.flows.handleV1Flows(w, r)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			require.Equal(t, tc.wantCalls, strings.Join(fake.calls, "; "))
		})
	}
}
