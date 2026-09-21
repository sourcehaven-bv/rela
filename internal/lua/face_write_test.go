package lua

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// runScript executes src against a writer runtime over the shared mock
// workspace and returns the manager double, so a test can assert the value
// that reached the manager BOUNDARY rather than merely that the call
// succeeded. A binding that parsed a face and discarded it would satisfy
// every error-free assertion; only the boundary value distinguishes the two.
func runScript(t *testing.T, src string) (*mockManager, error) {
	t.Helper()
	ws := newMockWorkspace(t)
	mgr := &mockManager{ws: ws}
	deps := ws.services(t.TempDir())
	deps.EntityManager = mgr
	var buf bytes.Buffer
	r := NewWriter(deps, &buf)
	defer r.Close()
	return mgr, r.RunString(src)
}

// runElevatedScript is runScript with the same mockManager wired as the
// ELEVATED handle, so rela.bypass_acl is registered and admin.* routes to a
// double whose boundary we can assert on.
func runElevatedScript(t *testing.T, src string) (*mockManager, error) {
	t.Helper()
	ws := newMockWorkspace(t)
	mgr := &mockManager{ws: ws}
	deps := ws.services(t.TempDir())
	deps.EntityManager = mgr
	deps.ElevatedManager = mgr
	var buf bytes.Buffer
	r := NewWriter(deps, &buf)
	defer r.Close()
	return mgr, r.RunString(src)
}

// TestCreateEntity_FaceReachesTheManager is AC 1: the face a script names is
// the face the manager is asked to write.
func TestCreateEntity_FaceReachesTheManager(t *testing.T) {
	mgr, err := runScript(t, `
rela.create_entity("ticket", {title = "T"}, "body", nil, { face = "draft" })
`)
	if err != nil {
		t.Fatalf("RunString: %v", err)
	}
	if got := mgr.lastCreateFace; got != entity.Face("draft") {
		t.Errorf("CreateOptions.Face = %q, want %q — the binding must thread "+
			"the face through, not parse and drop it", got, "draft")
	}
}

// TestCreateEntity_FaceIsReadableBack is the round-trip AC 1's second half
// and the reason the read side is in scope: a script that cannot observe a
// face cannot derive one from what it read, so every create would have to
// hardcode it.
func TestCreateEntity_FaceIsReadableBack(t *testing.T) {
	mgr, err := runScript(t, `
local e = rela.create_entity("ticket", {title = "T"}, "", nil, { face = "draft" })
if e.face ~= "draft" then
  error("expected e.face == 'draft', got " .. tostring(e.face))
end
`)
	if err != nil {
		t.Fatalf("RunString: %v", err)
	}
	if mgr.lastCreateFace != entity.Face("draft") {
		t.Errorf("face did not reach the manager: %q", mgr.lastCreateFace)
	}
}

// TestCreateRelation_FaceReachesTheManager is AC 6's first half at the
// binding level; the scope rule itself is the manager's and is covered by
// TestCreateRelation_FaceMustMatchScope in internal/entitymanager.
func TestCreateRelation_FaceReachesTheManager(t *testing.T) {
	mgr, err := runScript(t, `
rela.create_entity("ticket", {title = "A"}, "", "TICK-1")
rela.create_entity("ticket", {title = "B"}, "", "TICK-2")
rela.create_relation("TICK-1", "blocks", "TICK-2", { face = "draft" })
`)
	if err != nil {
		t.Fatalf("RunString: %v", err)
	}
	if got := mgr.lastRelationFace; got != entity.Face("draft") {
		t.Errorf("RelationOptions.FromFace = %q, want %q", got, "draft")
	}
}

// TestWriteOpts_MalformedRaises is AC 5. Every case here is a way of naming
// a face that must NOT quietly become "no face" — a dropped face writes to a
// row the caller did not name, which is BUG-HC6I2T's shape.
func TestWriteOpts_MalformedRaises(t *testing.T) {
	tests := []struct {
		name    string
		script  string
		wantMsg string
	}{
		{
			// The mistake a real script author makes: passing the face
			// positionally because they half-remember the signature.
			// GetTop() sees a present argument, so "ignore unless it is a
			// table" would silently drop it.
			name:    "non-table opts argument on create_entity",
			script:  `rela.create_entity("ticket", {title = "T"}, "", nil, "draft")`,
			wantMsg: "options must be a table",
		},
		{
			name:    "non-table opts argument on create_relation",
			script:  `rela.create_relation("A", "blocks", "B", "draft")`,
			wantMsg: "options must be a table",
		},
		{
			name:    "wrong-typed face value",
			script:  `rela.create_entity("ticket", {title = "T"}, "", nil, { face = 42 })`,
			wantMsg: `option "face" must be a string`,
		},
		{
			// A typo must not read as "no face": on a faced type it would
			// surface as a confusing face_required naming an argument the
			// caller believes they passed.
			name:    "unknown key",
			script:  `rela.create_entity("ticket", {title = "T"}, "", nil, { fce = "draft" })`,
			wantMsg: `unknown option "fce"`,
		},
		{
			// Empty is NOT a synonym for the default face: the caller named
			// a face and it is not a valid one. Deliberately unlike the HTTP
			// path, which trims and treats "" as absent.
			name:    "empty face string",
			script:  `rela.create_entity("ticket", {title = "T"}, "", nil, { face = "" })`,
			wantMsg: "empty face",
		},
		{
			name:    "face carrying the ref separator",
			script:  `rela.create_entity("ticket", {title = "T"}, "", nil, { face = "a@b" })`,
			wantMsg: "face",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mgr, err := runScript(t, tc.script)
			if err == nil {
				t.Fatalf("expected an error; malformed options must raise "+
					"rather than silently mean 'no face' (script: %s)", tc.script)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error = %q, want it to mention %q", err.Error(), tc.wantMsg)
			}
			// Nothing may have reached the manager on a refusal. Assert the
			// CALL COUNT, not the recorded face: the face fields are zero
			// both when nothing was called and when the binding called with
			// a dropped face — and the dropped face is the bug under test,
			// so a face-only assertion could not fail on it.
			if mgr.createCalls != 0 || mgr.relationCalls != 0 {
				t.Errorf("a refused call still reached the manager "+
					"(create=%d relation=%d); the binding must raise BEFORE "+
					"the write, not after", mgr.createCalls, mgr.relationCalls)
			}
		})
	}
}

// TestWriteOpts_AbsentAndNilMeanNoFace is AC 8's compatibility half plus the
// nil edge case. gopher-lua's GetTop() counts explicit trailing nils, so
// these two are distinguishable — treating them alike is a choice, not a
// limitation, and the choice is what keeps every pre-existing call working.
func TestWriteOpts_AbsentAndNilMeanNoFace(t *testing.T) {
	tests := []struct {
		name   string
		script string
	}{
		{"two args", `rela.create_entity("ticket", {title = "T"})`},
		{"three args", `rela.create_entity("ticket", {title = "T"}, "body")`},
		{"four args", `rela.create_entity("ticket", {title = "T"}, "body", "TICK-9")`},
		{"explicit nil opts", `rela.create_entity("ticket", {title = "T"}, "body", nil, nil)`},
		{"empty opts table", `rela.create_entity("ticket", {title = "T"}, "body", nil, {})`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mgr, err := runScript(t, tc.script)
			if err != nil {
				t.Fatalf("RunString: %v", err)
			}
			if mgr.lastCreateFace != "" {
				t.Errorf("Face = %q, want the zero face", mgr.lastCreateFace)
			}
		})
	}
}

// TestRelationTable_CarriesFromFace pins the relation read side. There is no
// to_face: targets are faceless by construction, which is what makes
// cross-world dangling references inexpressible.
func TestRelationTable_CarriesFromFace(t *testing.T) {
	_, err := runScript(t, `
rela.create_entity("ticket", {title = "A"}, "", "TICK-1")
rela.create_entity("ticket", {title = "B"}, "", "TICK-2")
local rel = rela.create_relation("TICK-1", "blocks", "TICK-2", { face = "draft" })
if rel.from_face ~= "draft" then
  error("expected rel.from_face == 'draft', got " .. tostring(rel.from_face))
end
if rel.to_face ~= nil then
  error("relations must not expose a to_face")
end
`)
	if err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

// TestCreateRelation_ContentRoundTrips covers the second opts key. It exists
// because `content` was the key with no test: the old signature documented a
// positional `content?` the body never read, so the option had to be both
// wired to the manager AND readable back, or it would be a documented feature
// that silently does nothing.
func TestCreateRelation_ContentRoundTrips(t *testing.T) {
	_, err := runScript(t, `
rela.create_entity("ticket", {title = "A"}, "", "TICK-1")
rela.create_entity("ticket", {title = "B"}, "", "TICK-2")
local r = rela.create_relation("TICK-1", "blocks", "TICK-2", { content = "why it blocks" })
if r.content ~= "why it blocks" then
  error("expected the body to round-trip, got " .. tostring(r.content))
end
`)
	if err != nil {
		t.Fatalf("RunString: %v", err)
	}
}

// TestElevatedCreateRelation_TakesAFace is AC 7's elevated half, and the
// reason RR-3NNWNK made the manager check a prerequisite: authorizeAndAudit
// returns early under bypassACL, so on this path the manager check is the
// ONLY thing between a Lua string and store.RelationData.FromFace. The
// binding must reach it, and it must reject there.
//
// Asserts at the manager boundary rather than on a returned table:
// admin.create_relation pushes a bare boolean, so there is nothing to read a
// face off.
func TestElevatedCreateRelation_TakesAFace(t *testing.T) {
	t.Run("threads a face through", func(t *testing.T) {
		mgr, err := runElevatedScript(t, `
rela.bypass_acl(function(admin)
  admin.create_relation("TICK-1", "blocks", "TICK-2", { face = "draft" })
end)
`)
		if err != nil {
			t.Fatalf("RunString: %v", err)
		}
		if got := mgr.lastRelationFace; got != entity.Face("draft") {
			t.Errorf("elevated FromFace = %q, want %q — the elevated binding "+
				"must thread the face too, or it is the one path that cannot "+
				"write a faced edge", got, "draft")
		}
	})

	t.Run("rejects a malformed options table", func(t *testing.T) {
		mgr, err := runElevatedScript(t, `
rela.bypass_acl(function(admin)
  admin.create_relation("TICK-1", "blocks", "TICK-2", { fce = "draft" })
end)
`)
		if err == nil {
			t.Fatal("expected a raise; the elevated path must validate its " +
				"options exactly as the gated one does")
		}
		if mgr.relationCalls != 0 {
			t.Errorf("a refused elevated call still reached the manager (%d calls)",
				mgr.relationCalls)
		}
	})
}

// TestEntityTable_FaceKeyAlwaysPresent pins the shape decision from RR-QR7BH0:
// `face` is set unconditionally, like mod_time and redacted, so a script
// never needs `e.face or "..."` and the table's shape does not depend on the
// data in it.
func TestEntityTable_FaceKeyAlwaysPresent(t *testing.T) {
	_, err := runScript(t, `
local e = rela.create_entity("ticket", {title = "T"})
if e.face == nil then
  error("face must be present even on the default face, not nil")
end
if e.face ~= "" then
  error("default face must read as the empty string, got " .. tostring(e.face))
end
`)
	if err != nil {
		t.Fatalf("RunString: %v", err)
	}
}
