package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// elevatedActionApp is a data-entry app on a real manager and ACL in which
// bob may read tickets, may not update them, and holds the "submit"
// permission an elevated action asks for.
type elevatedActionApp struct {
	app   *App
	decl  *acl.Declarative
	audit *audit.Memory
	root  string
}

func newElevatedActionApp(t *testing.T) *elevatedActionApp {
	t.Helper()
	return newElevatedActionAppWithACL(t, nil)
}

// newElevatedActionAppWithACL is newElevatedActionApp with writeACL in place
// of the declarative policy on the write path and the app; nil keeps the
// policy. The request still carries bob's policy read gate.
func newElevatedActionAppWithACL(t *testing.T, writeACL acl.ACL) *elevatedActionApp {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "actions"), 0o755); err != nil {
		t.Fatal(err)
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string"},
					"status": {Type: "string"},
				},
			},
			"secret": {
				IDPrefix:   "SEC-",
				Properties: map[string]metamodel.PropertyDef{"status": {Type: "string"}},
			},
		},
	}
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths := &project.Context{Root: root, CacheDir: filepath.Join(root, ".rela")}
	st := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths)).Store()
	for _, e := range []*entity.Entity{
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "T1", "status": "draft"}},
		{ID: "SEC-1", Type: "secret", Properties: map[string]any{"status": "draft"}},
	} {
		if err := st.CreateEntity(context.Background(), e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	decl := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"submitter": {Read: []string{"ticket"}, Permissions: []string{"submit"}},
		},
		Assignments: map[string]string{"bob": "submitter"},
	}, st)
	mem := audit.NewMemory()
	aclOpt := appbuildtest.WithDeclarative(decl)
	var appACL acl.ACL = decl
	if writeACL != nil {
		aclOpt, appACL = appbuildtest.WithACL(writeACL), writeACL
	}
	svc := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths), appbuildtest.WithStore(st),
		aclOpt, appbuildtest.WithAudit(mem))
	app := newAppFromParts(&Config{}, nil, newFixture())
	rebindApp(app, fs, paths, svc)
	setTestACL(t, app, appACL)
	app.schema.Publish(&Schema{Cfg: &Config{}, Meta: meta})
	if err := app.SetSecurityConfig(SecurityConfig{BindAddress: "127.0.0.1:8080"}); err != nil {
		t.Fatal(err)
	}
	return &elevatedActionApp{app: app, decl: decl, audit: mem, root: root}
}

// run installs body as action "submit" with the given config and runs it as
// bob.
func (a *elevatedActionApp) run(t *testing.T, action dataentryconfig.Action, body string) (code int, respBody string) {
	t.Helper()
	return a.runPayload(t, action, body, "")
}

// runPayload is run with payload as the request body.
func (a *elevatedActionApp) runPayload(
	t *testing.T, action dataentryconfig.Action, body, payload string,
) (code int, respBody string) {
	t.Helper()
	writeFile(t, filepath.Join(a.root, "actions", "submit.lua"), body)
	action.Script = "submit.lua"
	s := a.app.State()
	cfg := *s.Cfg
	cfg.Actions = map[string]dataentryconfig.Action{"submit": action}
	a.app.schema.Publish(&Schema{Cfg: &cfg, Meta: s.Meta})
	rec := postAction(principalCtx("bob"), t, a.app, a.decl, "submit", payload)
	if rec.Code != http.StatusOK {
		return rec.Code, rec.Body.String()
	}
	var resp script.ActionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return rec.Code, resp.Message
}

func (a *elevatedActionApp) prop(t *testing.T, id, name string) any {
	t.Helper()
	e, err := a.app.store.GetEntity(context.Background(), entity.Ref{ID: id})
	if err != nil {
		t.Fatalf("get %s: %v", id, err)
	}
	return e.Properties[name]
}

var elevatedSubmit = dataentryconfig.Action{
	Permission:      "submit",
	AllowACLBypass:  metamodel.ACLBypassWrite,
	ElevatedUpdates: map[string][]string{"ticket": {"status"}},
}

// TestAction_ElevatedUpdates pins `elevated_updates:` end to end
// (TKT-JA5SABG): bob may not update tickets, yet the elevated action sets the
// declared field; everything outside the scope is refused and writes nothing.
func TestAction_ElevatedUpdates(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		action     dataentryconfig.Action
		script     string
		wantCode   int
		wantBody   string
		wantStatus string
		wantTitle  string
	}{
		{
			name:   "declared field is written",
			action: elevatedSubmit,
			script: `local e = rela.bypass_acl(function(admin)
				return admin.update_entity("TKT-1", {status = "submitted"})
			end)
			return {message = e.properties.status}`,
			wantCode: http.StatusOK, wantBody: "submitted", wantStatus: "submitted", wantTitle: "T1",
		},
		{
			name:     "the gated binding is still denied",
			action:   elevatedSubmit,
			script:   `rela.update_entity("TKT-1", {status = "submitted"})`,
			wantCode: http.StatusUnprocessableEntity, wantStatus: "draft", wantTitle: "T1",
		},
		{
			name:   "undeclared property is refused",
			action: elevatedSubmit,
			script: `rela.bypass_acl(function(admin)
				admin.update_entity("TKT-1", {status = "submitted", title = "changed"})
			end)`,
			wantCode: http.StatusUnprocessableEntity, wantStatus: "draft", wantTitle: "T1",
		},
		{
			name:   "unreadable target is not found",
			action: elevatedSubmit,
			script: `local ok, err = pcall(function()
				rela.bypass_acl(function(admin) admin.update_entity("SEC-1", {status = "submitted"}) end)
			end)
			return {message = tostring(err)}`,
			wantCode: http.StatusOK, wantBody: "entity not found: SEC-1", wantStatus: "draft", wantTitle: "T1",
		},
		{
			name: "unreadable row of an in-scope type is not found",
			action: dataentryconfig.Action{
				Permission: "submit", AllowACLBypass: metamodel.ACLBypassWrite,
				ElevatedUpdates: map[string][]string{"ticket": {"status"}, "secret": {"status"}},
			},
			script: `local ok, err = pcall(function()
				rela.bypass_acl(function(admin) admin.update_entity("SEC-1", {status = "submitted"}) end)
			end)
			return {message = tostring(err)}`,
			wantCode: http.StatusOK, wantBody: "entity not found: SEC-1", wantStatus: "draft", wantTitle: "T1",
		},
		{
			name:     "no elevation without the opt-in",
			action:   dataentryconfig.Action{Permission: "submit"},
			script:   `return {message = tostring(rela.bypass_acl)}`,
			wantCode: http.StatusOK, wantBody: "nil", wantStatus: "draft", wantTitle: "T1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newElevatedActionApp(t)
			code, body := a.run(t, tc.action, tc.script)
			if code != tc.wantCode {
				t.Fatalf("code = %d, want %d; body=%s", code, tc.wantCode, body)
			}
			if tc.wantBody != "" && !strings.Contains(body, tc.wantBody) {
				t.Errorf("body = %q, want it to contain %q", body, tc.wantBody)
			}
			if got := a.prop(t, "TKT-1", "status"); got != tc.wantStatus {
				t.Errorf("status = %v, want %q", got, tc.wantStatus)
			}
			if got := a.prop(t, "TKT-1", "title"); got != tc.wantTitle {
				t.Errorf("title = %v, want %q", got, tc.wantTitle)
			}
			if got := a.prop(t, "SEC-1", "status"); got != "draft" {
				t.Errorf("SEC-1 status = %v, want draft", got)
			}
		})
	}
}

// TestAction_ElevatedUpdatesBound pins the wiring of the bound-row pin: an
// entity-bound action may update its own entity twice, because each elevated
// write moves the pin (the refusal on a concurrent change is pinned in
// entitymanager's TestScopedUpdates_Pin).
func TestAction_ElevatedUpdatesBound(t *testing.T) {
	t.Parallel()
	a := newElevatedActionApp(t)
	action := elevatedSubmit
	action.AvailableOn = &dataentryconfig.ActionScope{EntityTypes: []string{"ticket"}}
	code, body := a.runPayload(t, action, `rela.bypass_acl(function(admin)
		admin.update_entity(entity.id, {status = "submitted"})
		admin.update_entity(entity.id, {status = "approved"})
	end)`, `{"entity_id":"TKT-1"}`)
	if code != http.StatusNoContent {
		t.Fatalf("code = %d; body=%s", code, body)
	}
	if got := a.prop(t, "TKT-1", "status"); got != "approved" {
		t.Errorf("status = %v, want approved", got)
	}
}

// TestAction_ElevatedUpdatesAudit pins the audit trail of an elevated
// update: the bypass row and the update row name bob and the action.
func TestAction_ElevatedUpdatesAudit(t *testing.T) {
	t.Parallel()
	a := newElevatedActionApp(t)
	code, body := a.run(t, elevatedSubmit, `rela.bypass_acl(function(admin)
		admin.update_entity("TKT-1", {status = "submitted"})
	end)`)
	if code != http.StatusNoContent {
		t.Fatalf("code = %d; body=%s", code, body)
	}
	seen := map[string]bool{}
	for _, rec := range a.audit.Records() {
		if rec.Subject == nil || rec.Subject.ID != "TKT-1" {
			continue
		}
		if rec.Principal.User != "bob" {
			t.Errorf("%s row principal = %q, want bob", rec.Op, rec.Principal.User)
		}
		if rec.TriggeredBy != "action:submit" {
			t.Errorf("%s row triggered_by = %q, want action:submit", rec.Op, rec.TriggeredBy)
		}
		seen[rec.Op] = true
	}
	if !seen[audit.OpACLBypass] || !seen[audit.OpUpdateEntity] {
		t.Errorf("audit ops for TKT-1 = %v, want %s and %s", seen, audit.OpACLBypass, audit.OpUpdateEntity)
	}
}

// fakeElevatedUpdater stands in for the scoped writer in
// TestGrantElevatedUpdates; it is never called.
type fakeElevatedUpdater struct{}

func (fakeElevatedUpdater) PatchEntity(context.Context, string, entity.Patch) (*entity.UpdateResult, error) {
	return &entity.UpdateResult{}, nil
}

// TestGrantElevatedUpdates pins that an action without the opt-in gets no
// writer, and that an elevated action fails closed when no writer can be
// built.
func TestGrantElevatedUpdates(t *testing.T) {
	t.Parallel()
	ok := func(map[string][]string, string, *entity.Entity) (lua.ElevatedUpdater, error) {
		return fakeElevatedUpdater{}, nil
	}
	broken := func(map[string][]string, string, *entity.Entity) (lua.ElevatedUpdater, error) {
		return nil, errors.New("boom")
	}
	plain := dataentryconfig.Action{Script: "submit.lua", Permission: "submit"}

	tests := []struct {
		name       string
		factory    elevatedUpdaterFactory
		action     dataentryconfig.Action
		wantStatus int // 0: no failure
		wantWriter bool
	}{
		{name: "no opt-in, no writer", factory: ok, action: plain},
		{name: "no opt-in, nil factory", factory: nil, action: plain},
		{name: "opt-in gets writer", factory: ok, action: elevatedSubmit, wantWriter: true},
		{name: "opt-in, nil factory", factory: nil, action: elevatedSubmit, wantStatus: http.StatusInternalServerError},
		{
			name: "opt-in, factory error", factory: broken, action: elevatedSubmit,
			wantStatus: http.StatusInternalServerError,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var deps lua.WriteDeps
			fail := grantElevatedUpdates(tc.factory, &deps, "submit", tc.action, nil)
			gotStatus := 0
			if fail != nil {
				gotStatus = fail.status
			}
			if gotStatus != tc.wantStatus {
				t.Errorf("status = %d, want %d", gotStatus, tc.wantStatus)
			}
			if got := deps.ElevatedUpdater != nil; got != tc.wantWriter {
				t.Errorf("writer set = %v, want %v", got, tc.wantWriter)
			}
		})
	}
}

// TestAction_ElevatedUpdatesReadOnly pins that --read-only holds for an
// elevated action: the elevated update fails like an ordinary update on the
// shared run path that the HTTP route and the MCP tools use, and nothing
// changes.
func TestAction_ElevatedUpdatesReadOnly(t *testing.T) {
	t.Parallel()
	elevated := `rela.bypass_acl(function(admin)
		admin.update_entity("TKT-1", {status = "submitted"})
	end)`
	tests := []struct {
		name   string
		acl    acl.ACL
		script string
	}{
		{name: "elevated update", acl: acl.ReadOnlyACL{}, script: elevated},
		{name: "elevated update, pointer form", acl: &acl.ReadOnlyACL{}, script: elevated},
		{
			name: "ordinary update for comparison", acl: acl.ReadOnlyACL{},
			script: `rela.update_entity("TKT-1", {status = "submitted"})`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newElevatedActionAppWithACL(t, tc.acl)
			code, body := a.run(t, elevatedSubmit, tc.script)
			if code != http.StatusUnprocessableEntity {
				t.Fatalf("code = %d, want 422; body=%s", code, body)
			}
			if got := a.prop(t, "TKT-1", "status"); got != "draft" {
				t.Errorf("status = %v, want draft", got)
			}
			for _, rec := range a.audit.Records() {
				if rec.Op == audit.OpACLBypass || rec.Op == audit.OpUpdateEntity {
					t.Errorf("unexpected %s audit row", rec.Op)
				}
			}
		})
	}
}
