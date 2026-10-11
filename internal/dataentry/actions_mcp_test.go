package dataentry

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/openapi"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// The MCP action operations (TKT-NW9QSN) run the POST /_action/{id} route's
// gates. These tests drive them through MCPActions with the ctx the remote
// endpoint's middleware stamps, and pin that each refusal carries the
// route's code.

func mcpActionOps(t *testing.T, app *App) *MCPActions {
	t.Helper()
	ops := mcpHost(app).Actions
	require.NotNil(t, ops, "the host must carry the action operations")
	return ops
}

// requireActionCode asserts err is an ActionError with code.
func requireActionCode(t *testing.T, err error, code string) *script.ActionError {
	t.Helper()
	var opErr *script.ActionError
	require.ErrorAs(t, err, &opErr)
	require.Equal(t, code, opErr.Code, opErr.Error())
	return opErr
}

func actionIDs(list []script.ActionInfo) []string {
	ids := make([]string, len(list))
	for i, a := range list {
		ids[i] = a.ID
	}
	return ids
}

// detailActionsWithUnbound is regenerate-soa plus two actions that are not
// entity-bound, one of them behind the permission only alice holds.
func detailActionsWithUnbound() map[string]dataentryconfig.Action {
	return map[string]dataentryconfig.Action{
		"regenerate-soa": regenerateSoA(),
		"ping":           {Label: "Ping", Script: "regenerate.lua"},
		"export":         {Label: "Export", Script: "regenerate.lua", Permission: "documents:regenerate"},
	}
}

func TestMCPActions_List(t *testing.T) {
	d := newDetailActionApp(t, detailActionsWithUnbound())
	ops := mcpActionOps(t, d.app)

	tests := []struct {
		name     string
		user     string
		address  string
		want     []string
		wantCode string
	}{
		{name: "unbound, permission held", user: "alice", want: []string{"export", "ping"}},
		{name: "unbound, permission missing", user: "bob", want: []string{"ping"}},
		{name: "entity where the action applies", user: "alice", address: "DOC-1@concept",
			want: []string{"regenerate-soa"}},
		{name: "entity where when is false", user: "alice", address: "DOC-2@concept", want: []string{}},
		{name: "entity, permission missing", user: "bob", address: "DOC-1@concept", want: []string{}},
		{name: "absent entity", user: "alice", address: "DOC-404@concept", wantCode: "entity_not_found"},
		{name: "unreadable entity", user: "mallory", address: "DOC-1@concept", wantCode: "entity_not_found"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			list, err := ops.ListActions(mcpCtx(t, tc.user, d.decl), tc.address)
			if tc.wantCode != "" {
				requireActionCode(t, err, tc.wantCode)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, append([]string{}, actionIDs(list)...))
		})
	}

	list, err := ops.ListActions(mcpCtx(t, "alice", d.decl), "DOC-1@concept")
	require.NoError(t, err)
	require.Equal(t, script.ActionInfo{
		ID: "regenerate-soa", Label: "Regenerate", Confirm: true, ConfirmText: "Overwrite the concept?",
		EntityBound: true,
	}, list[0])
}

// The MCP run answers what the HTTP route answers for the same caller and
// entity (TestDetailAction_AffordanceAndGateAgree is the route's half).
func TestMCPActions_RunGates(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		address  string
		wantCode string
	}{
		{name: "all four match", user: "alice", address: "DOC-1@concept"},
		{name: "wrong type", user: "alice", address: "NOTE-9", wantCode: "action_not_available"},
		{name: "wrong face", user: "alice", address: "DOC-1@approved", wantCode: "action_not_available"},
		{name: "when false", user: "alice", address: "DOC-2@concept", wantCode: "action_not_available"},
		{name: "permission missing", user: "bob", address: "DOC-1@concept", wantCode: "permission_required"},
		{name: "unreadable entity", user: "mallory", address: "DOC-1@concept", wantCode: "permission_required"},
		{name: "absent entity", user: "alice", address: "DOC-404@concept", wantCode: "entity_not_found"},
		{name: "no address", user: "alice", wantCode: "entity_not_found"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := newDetailActionApp(t, map[string]dataentryconfig.Action{"regenerate-soa": regenerateSoA()})
			resp, err := mcpActionOps(t, d.app).RunAction(mcpCtx(t, tc.user, d.decl), "regenerate-soa",
				script.ActionRunInput{Address: tc.address, Confirmed: true})
			if tc.wantCode != "" {
				requireActionCode(t, err, tc.wantCode)
				require.Empty(t, d.markers(t), "the script must not run")
				return
			}
			require.NoError(t, err)
			require.Equal(t, "Regenerated", resp.Message)
			require.Equal(t, []string{"ran:DOC-1@concept:SECRET-OWNER:redacted=false"}, d.markers(t))
		})
	}
}

func TestMCPActions_ConfirmRequired(t *testing.T) {
	d := newDetailActionApp(t, map[string]dataentryconfig.Action{"regenerate-soa": regenerateSoA()})
	ops := mcpActionOps(t, d.app)
	ctx := mcpCtx(t, "alice", d.decl)

	_, err := ops.RunAction(ctx, "regenerate-soa", script.ActionRunInput{Address: "DOC-1@concept"})
	opErr := requireActionCode(t, err, "confirmation_required")
	require.Contains(t, opErr.Title, "Overwrite the concept?")
	require.Empty(t, d.markers(t), "an unconfirmed action must not run")

	_, err = ops.RunAction(ctx, "regenerate-soa", script.ActionRunInput{Address: "DOC-1@concept", Confirmed: true})
	require.NoError(t, err)
	require.Len(t, d.markers(t), 1)
}

func TestMCPActions_UnknownAndInvalid(t *testing.T) {
	app := newActionTestApp(t, nil)
	app.Cfg().Actions = map[string]dataentryconfig.Action{}
	ops := mcpActionOps(t, app)
	ctx := mcpCtx(t, "alice", nil)

	_, err := ops.RunAction(ctx, "missing", script.ActionRunInput{})
	requireActionCode(t, err, "action_not_found")
	_, err = ops.RunAction(ctx, "../etc", script.ActionRunInput{})
	requireActionCode(t, err, "invalid_action_id")

	list, err := ops.ListActions(ctx, "")
	require.NoError(t, err)
	require.Empty(t, list)
}

func TestMCPActions_Responses(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"hello.lua": `return {redirect = "/done", message = "ok", message_type = "success"}`,
		"alert.lua": `return {status = 202, body = rela.request.body.host .. "/" .. rela.request.method,
			content_type = "text/plain"}`,
		"boom.lua": `print("captured")
error("kaboom")`,
	})
	app.Cfg().Actions = map[string]dataentryconfig.Action{
		"hello": {Script: "hello.lua"},
		"alert": {Script: "alert.lua", Request: &dataentryconfig.ActionRequest{Body: true, MaxBodyBytes: 64}},
		"boom":  {Script: "boom.lua"},
	}
	ops := mcpActionOps(t, app)
	ctx := mcpCtx(t, "alice", nil)

	t.Run("envelope", func(t *testing.T) {
		resp, err := ops.RunAction(ctx, "hello", script.ActionRunInput{})
		require.NoError(t, err)
		require.Equal(t, &script.ActionResponse{Redirect: "/done", Message: "ok", MessageType: "success"}, resp)
	})
	t.Run("request body and rich response", func(t *testing.T) {
		resp, err := ops.RunAction(ctx, "alert", script.ActionRunInput{Body: map[string]any{"host": "web1"}})
		require.NoError(t, err)
		require.Equal(t, 202, resp.Status)
		require.Equal(t, "web1/POST", resp.Body)
		require.Equal(t, "text/plain", resp.ContentType)
	})
	t.Run("body over the cap", func(t *testing.T) {
		big := map[string]any{"host": string(make([]byte, 100))}
		_, err := ops.RunAction(ctx, "alert", script.ActionRunInput{Body: big})
		requireActionCode(t, err, "body_too_large")
	})
	t.Run("body for an action without a request block", func(t *testing.T) {
		_, err := ops.RunAction(ctx, "hello", script.ActionRunInput{Body: map[string]any{"x": 1}})
		requireActionCode(t, err, "invalid_body")
	})
	t.Run("script error is degraded", func(t *testing.T) {
		_, err := ops.RunAction(ctx, "boom", script.ActionRunInput{})
		opErr := requireActionCode(t, err, "script_error")
		require.NotEmpty(t, opErr.CorrelationID)
		require.Contains(t, opErr.Detail, "actions/boom.lua:2")
		require.Contains(t, opErr.Detail, "kaboom")
		require.NotContains(t, opErr.Error(), "captured", "captured output is loopback-only detail")
	})
}

// actionOverMCPMount runs regenerate-soa through the remote MCP mount as the
// verified subject, as a tools/call would, and returns the status and body.
func actionOverMCPMount(t *testing.T, app *App, subject string) (code int, body string) {
	t.Helper()
	// The router reads the OpenAPI generator, which newDetailActionApp's
	// schema leaves out.
	s := *app.schema.Current()
	s.OpenAPIGen = openapi.New(s.Meta, openapi.Config{Title: "test", Version: "1.0.0"})
	app.schema.Publish(&s)
	require.NoError(t, app.SetJWTGate(JWTGateConfig{
		Verifier:   gateVerifier{validToken: "good.jwt.token", subject: subject},
		HeaderName: "X-Auth-Assertion",
	}))
	require.NoError(t, app.SetRemoteMCP(func(host MCPHost) (http.Handler, error) {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp, err := host.Actions.RunAction(r.Context(), "regenerate-soa",
				script.ActionRunInput{Address: "DOC-1@concept", Confirmed: true})
			if err != nil {
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte(resp.Message))
		}), nil
	}))
	req := httptest.NewRequest(http.MethodPost, MCPPath, strings.NewReader("{}"))
	req.Header.Set("X-Auth-Assertion", "good.jwt.token")
	// newDetailActionApp binds the security config to loopback.
	req.Host = "127.0.0.1:8080"
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func TestMCPActions_ThroughTheRemoteMount(t *testing.T) {
	t.Run("runs as the verified user", func(t *testing.T) {
		d := newDetailActionApp(t, map[string]dataentryconfig.Action{"regenerate-soa": regenerateSoA()})
		code, body := actionOverMCPMount(t, d.app, "alice")
		require.Equal(t, http.StatusOK, code, body)
		require.Equal(t, "Regenerated", body)
		recs := d.audit.Records()
		require.NotEmpty(t, recs)
		for _, r := range recs {
			require.Equal(t, "alice", r.Principal.User)
			require.Equal(t, principal.ToolMCP, r.Principal.Tool)
		}
	})
	t.Run("the request's grants apply", func(t *testing.T) {
		d := newDetailActionApp(t, map[string]dataentryconfig.Action{"regenerate-soa": regenerateSoA()})
		code, body := actionOverMCPMount(t, d.app, "bob")
		require.Equal(t, http.StatusForbidden, code, body)
		require.Contains(t, body, "[permission_required]")
		require.Empty(t, d.markers(t))
	})
}

// The confirmation question comes only after every gate has passed, so a
// caller is never asked to confirm a run that would be refused anyway.
func TestMCPActions_ConfirmAfterGates(t *testing.T) {
	d := newDetailActionApp(t, map[string]dataentryconfig.Action{"regenerate-soa": regenerateSoA()})
	ops := mcpActionOps(t, d.app)

	_, err := ops.RunAction(mcpCtx(t, "bob", d.decl), "regenerate-soa",
		script.ActionRunInput{Address: "DOC-1@concept"})
	requireActionCode(t, err, "permission_required")
	_, err = ops.RunAction(mcpCtx(t, "alice", d.decl), "regenerate-soa",
		script.ActionRunInput{Address: "DOC-404@concept"})
	requireActionCode(t, err, "entity_not_found")
	_, err = ops.RunAction(mcpCtx(t, "alice", d.decl), "regenerate-soa",
		script.ActionRunInput{Address: "DOC-2@concept"})
	requireActionCode(t, err, "action_not_available")
}

// A bound action judged against a config that has since been reloaded is
// refused, on every transport.
func TestMCPActions_ConfigReloaded(t *testing.T) {
	d := newDetailActionApp(t, map[string]dataentryconfig.Action{"regenerate-soa": regenerateSoA()})
	stale := d.app.schema.Current()
	fresh := *stale
	d.app.schema.Publish(&fresh)

	_, fail := actionOps{h: d.app.write}.run(mcpCtx(t, "alice", d.decl), stale, "regenerate-soa",
		regenerateSoA(), actionPayload{EntityID: "DOC-1@concept"}, nil)
	require.NotNil(t, fail)
	requireActionCode(t, fail.actionError(t.Context()), "config_reloaded")
	require.Empty(t, d.markers(t))
}

// An action that is not entity-bound still runs when its optional entity is
// unreadable, exactly as when none was named: the entity is not the resource,
// and refusing would make the action an existence oracle.
func TestMCPActions_UnboundRunsWithoutUnreadableEntity(t *testing.T) {
	d := newDetailActionApp(t, map[string]dataentryconfig.Action{
		"probe": {Script: "probe.lua"},
	})
	require.NoError(t, os.WriteFile(filepath.Join(d.app.paths.Root, "actions", "probe.lua"),
		[]byte(`return {message = entity == nil and "none" or entity.id}`), 0o644))
	ops := mcpActionOps(t, d.app)

	for user, want := range map[string]string{"alice": "DOC-1", "mallory": "none"} {
		resp, err := ops.RunAction(mcpCtx(t, user, d.decl), "probe",
			script.ActionRunInput{Address: "DOC-1@concept"})
		require.NoError(t, err, user)
		require.Equal(t, want, resp.Message, user)
	}
	resp, err := ops.RunAction(mcpCtx(t, "alice", d.decl), "probe",
		script.ActionRunInput{Address: "DOC-404@concept"})
	require.NoError(t, err)
	require.Equal(t, "none", resp.Message)
}
