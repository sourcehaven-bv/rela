package lua

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// listFaceFixture declares the faces draft and published on ticket and seeds
// two drafts and one published row beside newMockWorkspace's two bare ones.
func listFaceFixture(t *testing.T, elevated bool) *Runtime {
	t.Helper()
	ws := newMockWorkspace(t)
	def := ws.meta.Entities["ticket"]
	def.Faces = map[string]metamodel.FaceDef{"draft": {}, "published": {}}
	ws.meta.Entities["ticket"] = def
	for _, e := range []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Face: "draft", Properties: map[string]any{"title": "d1"}},
		{ID: "TKT-003", Type: "ticket", Face: "draft", Properties: map[string]any{"title": "d3"}},
		{ID: "TKT-004", Type: "ticket", Face: "published", Properties: map[string]any{"title": "p4"}},
	} {
		ws.seedEntity(e)
	}
	deps := ws.services("/tmp")
	if elevated {
		deps.ElevatedReader = rawAddressReader{ws.store}
		deps.ElevationRecorder = &recordingElevationRecorder{}
	}
	var buf bytes.Buffer
	r := NewWriter(deps, &buf)
	t.Cleanup(r.Close)
	return r
}

// TestListEntities_FaceOption pins GitHub #1761: a script lists the rows of
// one named face, each carrying that face, instead of the face the world
// serves.
func TestListEntities_FaceOption(t *testing.T) {
	t.Parallel()
	r := listFaceFixture(t, false)
	script := `
		local drafts = rela.list_entities("ticket", {face = "draft"})
		if #drafts ~= 2 then error("drafts: got " .. #drafts .. ", want 2") end
		for _, e in ipairs(drafts) do
			if e.face ~= "draft" then error(e.id .. " has face " .. e.face) end
		end
		local open = rela.list_entities("ticket", {face = "draft", filter = "title=d3"})
		if #open ~= 1 or open[1].id ~= "TKT-003" then error("face with filter did not compose") end
		if #rela.list_entities("ticket", {face = "draft", limit = 1}) ~= 1 then
			error("face with limit did not compose")
		end
		if #rela.list_entities("ticket") ~= 2 then error("no option must keep the world's rows") end
	`
	if err := r.RunString(script); err != nil {
		t.Fatal(err)
	}
}

func TestListEntities_FaceOptionRejectsBadValues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, call, want string }{
		{"undeclared face", `rela.list_entities("ticket", {face = "archived"})`, `declares no face "archived"`},
		{"faceless type", `rela.list_entities("feature", {face = "draft"})`, `declares no face "draft"`},
		{"unknown type", `rela.list_entities("tikcet", {face = "draft"})`, `unknown entity type "tikcet"`},
		{"malformed face", `rela.list_entities("ticket", {face = "Draft"})`, "must be lowercase"},
		{"not a string", `rela.list_entities("ticket", {face = 1})`, "must be a string"},
		{"admin undeclared face", `rela.bypass_acl(function(a) a.list_entities("ticket", {face = "archived"}) end)`,
			`declares no face "archived"`},
		{"admin malformed face", `rela.bypass_acl(function(a) a.list_entities("ticket", {face = "Draft"}) end)`,
			"must be lowercase"},
		{"admin not a table", `rela.bypass_acl(function(a) a.list_entities("ticket", "x") end)`,
			"options must be a table"},
		{"admin unknown key", `rela.bypass_acl(function(a) a.list_entities("ticket", {filter = "x"}) end)`,
			`unknown option "filter"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := listFaceFixture(t, true).RunString(tc.call)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%s: got error %v, want one containing %q", tc.call, err, tc.want)
			}
		})
	}
}

// TestElevatedListEntities_FaceOption pins the same option on
// admin.list_entities.
func TestElevatedListEntities_FaceOption(t *testing.T) {
	t.Parallel()
	r := listFaceFixture(t, true)
	script := `
		rela.bypass_acl(function(admin)
			local pub = admin.list_entities("ticket", {face = "published"})
			if #pub ~= 1 or pub[1].id ~= "TKT-004" or pub[1].face ~= "published" then
				error("admin face option did not list the published row")
			end
			if #admin.list_entities("ticket") ~= 2 then error("no option must keep the world's rows") end
		end)
	`
	if err := r.RunString(script); err != nil {
		t.Fatal(err)
	}
}
