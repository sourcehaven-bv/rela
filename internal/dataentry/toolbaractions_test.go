package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// List toolbar actions: an entity-less action offered as a button on a list.
// Each case checks BOTH halves: the `_actions["action:<id>"]` key on the list
// response and the POST /_action/{id} run without an entity.

// newNoteScript runs without an entity: it fails if one is in scope, so a
// test notices when the toolbar path starts sending a row.
const newNoteScript = `
if entity ~= nil then error("toolbar action received an entity") end
local note = rela.create_entity("note", { title = "ran:toolbar" })
return { redirect = "/entity/note/" .. note.id }
`

func newToolbarActionApp(t *testing.T, action dataentryconfig.Action) detailApp {
	t.Helper()
	actions := map[string]dataentryconfig.Action{"new-note": action}
	d := newDetailActionApp(t, actions)
	if err := os.WriteFile(filepath.Join(d.app.ProjectRoot(), "actions", "new_note.lua"),
		[]byte(newNoteScript), 0o644); err != nil {
		t.Fatal(err)
	}
	d.app.schema.Publish(&Schema{
		Cfg: &Config{
			Actions: actions,
			Lists: map[string]dataentryconfig.List{
				"all_notes": {EntityType: "note", ToolbarActions: []string{"new-note"}},
			},
		},
		Meta: d.app.Meta(),
	})
	return d
}

// listActions returns the `_actions` map of GET /api/v1/<plural> as user.
func (d detailApp) listActions(t *testing.T, user, typeName, plural string) map[string]bool {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/"+plural, http.NoBody).
		WithContext(gateCtxFor(principalCtx(user), t, d.decl))
	rec := httptest.NewRecorder()
	d.app.handleV1ListEntities(rec, req, typeName, plural)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s as %s: %d %s", plural, user, rec.Code, rec.Body)
	}
	var got struct {
		Actions map[string]bool `json:"_actions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return got.Actions
}

func TestToolbarAction_AffordanceAndGateAgree(t *testing.T) {
	newNote := dataentryconfig.Action{
		Label: "New note", Script: "new_note.lua", Permission: "documents:regenerate",
	}
	tests := []struct {
		name       string
		action     dataentryconfig.Action
		user       string
		wantStatus int
	}{
		{name: "holds the permission", action: newNote, user: "alice", wantStatus: http.StatusOK},
		{name: "lacks the permission", action: newNote, user: "bob", wantStatus: http.StatusForbidden},
		{
			name:   "no permission configured",
			action: dataentryconfig.Action{Label: "New note", Script: "new_note.lua"},
			user:   "bob", wantStatus: http.StatusOK,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := newToolbarActionApp(t, tc.action)
			wantOffered := tc.wantStatus == http.StatusOK
			if got := d.listActions(t, tc.user, "note", "notes")["action:new-note"]; got != wantOffered {
				t.Errorf("affordance = %v, want %v", got, wantOffered)
			}

			// The SPA posts no body, as the sidebar does.
			rec := postAction(principalCtx(tc.user), t, d.app, d.decl, "new-note", "")
			if rec.Code != tc.wantStatus {
				t.Fatalf("POST status = %d, want %d: %s", rec.Code, tc.wantStatus, rec.Body)
			}
			if ran := len(d.markers(t)) > 0; ran != wantOffered {
				t.Errorf("script ran = %v, want %v", ran, wantOffered)
			}
			if wantOffered && !strings.Contains(rec.Body.String(), `"redirect":"/entity/note/`) {
				t.Errorf("response lacks the script's redirect: %s", rec.Body)
			}
		})
	}
}

// The keys belong to the lists over the requested type only.
func TestToolbarAction_OnlyOnListsOfThatType(t *testing.T) {
	d := newToolbarActionApp(t, dataentryconfig.Action{Label: "New note", Script: "new_note.lua"})
	if got := d.listActions(t, "alice", "document", "documents"); got["action:new-note"] {
		t.Errorf("documents list offers a note list's toolbar action: %v", got)
	}
}

func TestAddToolbarActions(t *testing.T) {
	cfg := &Config{
		Actions: map[string]dataentryconfig.Action{
			"plain": {Label: "Plain", Script: "p.lua"},
			"bound": {Label: "Bound", Script: "b.lua", AvailableOn: &dataentryconfig.ActionScope{
				EntityTypes: []string{"note"},
			}},
		},
		Lists: map[string]dataentryconfig.List{
			"notes": {EntityType: "note", ToolbarActions: []string{"plain", "bound", "missing"}},
		},
	}
	ctx := context.Background()
	got := addToolbarActions(ctx, cfg, "note", map[string]bool{"create": true})
	want := map[string]bool{"create": true, "action:plain": true}
	if len(got) != len(want) || !got["create"] || !got["action:plain"] {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := addToolbarActions(ctx, cfg, "document", nil); got != nil {
		t.Errorf("type without toolbar actions: got %v, want nil", got)
	}
	if got := addToolbarActions(ctx, nil, "note", nil); got != nil {
		t.Errorf("nil cfg: got %v, want nil", got)
	}
}
