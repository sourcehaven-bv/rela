package aclmap_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// createEdgePolicy: the `editor` role grants create on incident and is
// conferred by the `editor-of` edge (a LOCAL, per-entity route).
const createEdgePolicy = `
membership_relation: member-of
roles:
  editor: { read: [incident], create: [incident], update: [incident], delete: [incident] }
assignments:
  PERS-GLOBAL: editor
role_relations:
  editor-of: { confers: editor }
inherit_roles_through: [belongs-to]
`

// createWorld: PERS-GLOBAL is a global editor; PERS-EDGE is editor-of
// FOLDER-A which contains INC-A via belongs-to.
func createWorld(t *testing.T) *world {
	t.Helper()
	return buildWorld(t, createEdgePolicy,
		[]ent{
			{"PERS-GLOBAL", "person"}, {"PERS-EDGE", "person"},
			{"FOLDER-A", "folder"}, {"INC-A", "incident"},
		},
		[]rel{
			{"PERS-EDGE", "editor-of", "FOLDER-A"},
			{"INC-A", "belongs-to", "FOLDER-A"},
		},
	)
}

// TestCreate_MatchesRuntime is the create-verb conformance guard. The
// production create path authorizes with the new entity's concrete ID
// (entitymanager.ApplyEntity: EntitySubject{ID: e.ID}), so the runtime
// FOLDS local-role-via-edge routes into a create decision — create is NOT
// globals-only in production. The report must therefore credit an
// edge-conferred create exactly as the runtime does: reporting create as
// globals-only would be a FALSE DENY for a principal the runtime lets
// create via an edge. This test pins Can(create) == AuthorizeWrite(create)
// for both a global and an edge-only editor so the report can never drift
// from the runtime in either direction.
func TestCreate_MatchesRuntime(t *testing.T) {
	t.Parallel()
	w := createWorld(t)
	ctx := context.Background()

	for _, prin := range []string{"PERS-GLOBAL", "PERS-EDGE"} {
		req, rErr := w.decl.ForPrincipal(principal.Principal{User: prin, Tool: principal.ToolCLI})
		if rErr != nil {
			t.Fatalf("ForPrincipal %s: %v", prin, rErr)
		}
		res, cErr := w.eng.Can(ctx, prin, acl.VerbCreate, "INC-A")
		if cErr != nil {
			t.Fatalf("Can(%s create): %v", prin, cErr)
		}
		runtime := runtimeVerdict(ctx, t, req, acl.VerbCreate, "incident", "INC-A")
		if res.Allowed != runtime {
			t.Errorf("%s create INC-A: can=%v runtime=%v (report must match the "+
				"runtime create decision, which folds edge routes)", prin, res.Allowed, runtime)
		}
	}
}

// TestCreate_EdgeConferredCreateReported: an edge-only editor CAN create
// (the runtime folds the edge for create), and the map surfaces that create
// route as a per-entity exception — the same route it surfaces for update.
func TestCreate_EdgeConferredCreateReported(t *testing.T) {
	t.Parallel()
	w := createWorld(t)
	ctx := context.Background()

	edge, err := w.eng.Can(ctx, "PERS-EDGE", acl.VerbCreate, "INC-A")
	if err != nil {
		t.Fatalf("Can edge create: %v", err)
	}
	if !edge.Allowed {
		t.Fatalf("PERS-EDGE holds editor via the edge; the runtime folds edge " +
			"routes for create (concrete id), so create must be ALLOWED")
	}
	if len(edge.Routes) == 0 {
		t.Errorf("an edge-conferred create allow must carry its edge route")
	}

	// In the per-principal map, create surfaces as an exception on INC-A,
	// mirroring the update route on the same edge.
	res := mapAll(t, w, "PERS-EDGE")
	inc := typeOf(res, "incident")
	if inc == nil {
		t.Fatalf("PERS-EDGE should have incident access via the edge")
	}
	sawCreate := false
	for _, ex := range inc.Exceptions {
		if ex.Entity == "INC-A" && len(ex.Extra["create"]) > 0 {
			sawCreate = true
		}
	}
	if !sawCreate {
		t.Errorf("edge-conferred create should surface as an INC-A exception; got %+v", inc.Exceptions)
	}
	// Create is edge-conferred here, not type-wide, so it must NOT be a
	// type-level baseline (that would falsely imply create on every incident).
	if len(inc.Baseline["create"]) != 0 {
		t.Errorf("edge-conferred create must not appear as a type baseline; got %+v", inc.Baseline["create"])
	}
}
