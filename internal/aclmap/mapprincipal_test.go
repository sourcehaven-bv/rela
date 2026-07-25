package aclmap_test

import (
	"context"
	"sort"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclmap"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// groundingTypes is the entity-type universe of the grounding world,
// passed to MapPrincipal the way the CLI passes svc.Meta.EntityTypes().
var groundingTypes = []string{"person", "team", "folder", "incident", "ticket", "project"}

// mapAll runs MapPrincipal over all verbs and all grounding types.
func mapAll(t *testing.T, w *world, prin string) *aclmap.MapPrincipalResult {
	t.Helper()
	res, err := w.eng.MapPrincipal(context.Background(), prin, "", "", groundingTypes)
	if err != nil {
		t.Fatalf("MapPrincipal(%s): %v", prin, err)
	}
	return res
}

// typeAccess returns the TypeAccess for a type in a result, or nil.
func typeOf(res *aclmap.MapPrincipalResult, typ string) *aclmap.TypeAccess {
	for i := range res.Types {
		if res.Types[i].Type == typ {
			return &res.Types[i]
		}
	}
	return nil
}

func TestMapPrincipal_GlobalBaseline(t *testing.T) {
	t.Parallel()
	// PERS-ALICE is a global editor: read/create/update/delete on ticket,
	// read on incident — all as a type-level [global] baseline, no
	// per-entity exceptions.
	res := mapAll(t, groundingWorld(t), "PERS-ALICE")
	if res.EveryoneOnly {
		t.Errorf("Alice has an editor assignment; EveryoneOnly must be false")
	}
	tk := typeOf(res, "ticket")
	if tk == nil {
		t.Fatalf("expected a ticket TypeAccess for Alice; got types %v", typeNames(res))
	}
	for _, verb := range []string{"read", "create", "update", "delete"} {
		if len(tk.Baseline[verb]) == 0 {
			t.Errorf("ticket baseline missing %s for editor Alice", verb)
		}
	}
	if len(tk.Exceptions) != 0 {
		t.Errorf("Alice's ticket access is type-level; want no exceptions, got %v", tk.Exceptions)
	}
}

func TestMapPrincipal_GroupBaseline(t *testing.T) {
	t.Parallel()
	// PERS-BOB inherits security via ROLE-SECURITY: incident access shows
	// as a [group] baseline, not a per-entity exception.
	res := mapAll(t, groundingWorld(t), "PERS-BOB")
	inc := typeOf(res, "incident")
	if inc == nil || len(inc.Baseline["read"]) == 0 {
		t.Fatalf("Bob should have an incident read baseline via group; got %+v", inc)
	}
	found := false
	for _, r := range inc.Baseline["read"] {
		if r.Kind == "group" && r.Group == "ROLE-SECURITY" {
			found = true
		}
	}
	if !found {
		t.Errorf("Bob's incident read baseline should name group ROLE-SECURITY; got %+v", inc.Baseline["read"])
	}
}

func TestMapPrincipal_InheritanceIsAnException(t *testing.T) {
	t.Parallel()
	// PERS-DAVE is editor-of FOLDER-Q3, which contains INC-042 via
	// belongs-to. His incident access is NOT a type baseline — it applies
	// only to INC-042, so it must surface as a per-entity exception and
	// INC-999 (not contained) must NOT appear.
	res := mapAll(t, groundingWorld(t), "PERS-DAVE")
	inc := typeOf(res, "incident")
	if inc == nil {
		t.Fatalf("Dave should have incident access via inheritance")
	}
	if len(inc.Baseline) != 0 {
		t.Errorf("Dave has no type-level incident grant; want empty baseline, got %+v", inc.Baseline)
	}
	var exEntities []string
	for _, ex := range inc.Exceptions {
		exEntities = append(exEntities, ex.Entity)
	}
	if !contains(exEntities, "INC-042") {
		t.Errorf("INC-042 (contained in Dave's folder) must be an exception; got %v", exEntities)
	}
	if contains(exEntities, "INC-999") {
		t.Errorf("INC-999 (not contained) must NOT be an exception; got %v", exEntities)
	}
}

func TestMapPrincipal_EveryoneOnlyIsCutOff(t *testing.T) {
	t.Parallel()
	// A principal with no assignment, no group, no edge has only the
	// everyone baseline — the offboarding "fully cut off" signal. The
	// everyone grant (read project) still shows as a baseline.
	res := mapAll(t, groundingWorld(t), "PERS-GHOST")
	if !res.EveryoneOnly {
		t.Errorf("a principal with no personal grant must be EveryoneOnly; types=%v", typeNames(res))
	}
	proj := typeOf(res, "project")
	if proj == nil || len(proj.Baseline["read"]) == 0 {
		t.Errorf("everyone read:[project] baseline should still show; got %+v", proj)
	}
}

func TestMapPrincipal_EveryoneBaselineShownForEveryone(t *testing.T) {
	t.Parallel()
	// Even a privileged principal must see the everyone baseline (a
	// per-principal map is that principal's COMPLETE access, incl. what
	// they get for free). AccessRoutes omits everyone, so the map seeds it
	// explicitly — this guards that seeding.
	res := mapAll(t, groundingWorld(t), "PERS-ALICE")
	proj := typeOf(res, "project")
	if proj == nil || len(proj.Baseline["read"]) == 0 {
		t.Fatalf("Alice must also see the everyone read:[project] baseline; got %+v", proj)
	}
	if proj.Baseline["read"][0].Role != acl.EveryoneRole {
		t.Errorf("project read baseline should be the everyone role; got %+v", proj.Baseline["read"])
	}
}

// TestMapPrincipal_MatchesRuntime is the anti-false-negative guard for
// the per-principal view (the RR-7UXWNA guarantee, applied to map): for
// every principal, type, verb, and entity, the map's verdict (is this
// grant present in baseline-or-exception?) must agree with the runtime
// AuthorizeWrite / PermitsRead decision.
func TestMapPrincipal_MatchesRuntime(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	ctx := context.Background()

	principalsUnderTest := []string{
		"PERS-ALICE", "PERS-BOB", "PERS-CAROL", "PERS-DAVE", "PERS-EVE",
	}
	// (type, id) pairs spanning the grounding graph.
	entities := []struct{ id, typ string }{
		{"INC-042", "incident"}, {"INC-999", "incident"}, {"TKT-1", "ticket"}, {"PROJ", "project"},
	}
	verbs := []acl.Verb{acl.VerbRead, acl.VerbUpdate, acl.VerbDelete}

	for _, prin := range principalsUnderTest {
		res := mapAll(t, w, prin)
		req, err := w.decl.ForPrincipal(principal.Principal{User: prin, Tool: principal.ToolCLI})
		if err != nil {
			t.Fatalf("ForPrincipal %s: %v", prin, err)
		}
		for _, e := range entities {
			for _, verb := range verbs {
				mapGrants := mapVerdict(res, e.typ, e.id, string(verb))
				runtime := runtimeVerdict(ctx, t, req, verb, e.typ, e.id)
				// The map counts the everyone baseline as "granted"; the
				// runtime read gate also grants via everyone, so they align
				// for read. For writes, everyone doesn't grant in the
				// grounding policy, so no divergence.
				if mapGrants != runtime {
					t.Errorf("disagreement: %s %s %s — map=%v runtime=%v",
						prin, verb, e.id, mapGrants, runtime)
				}
			}
		}
	}
}

// mapVerdict reports whether the map result grants (verb) on entity id of
// type typ — via a type baseline OR a per-entity exception.
func mapVerdict(res *aclmap.MapPrincipalResult, typ, id, verb string) bool {
	ta := typeOf(res, typ)
	if ta == nil {
		return false
	}
	if len(ta.Baseline[verb]) > 0 {
		return true
	}
	for _, ex := range ta.Exceptions {
		if ex.Entity == id && len(ex.Extra[verb]) > 0 {
			return true
		}
	}
	return false
}

// runtimeVerdict asks the real runtime whether the principal may perform
// verb on the entity — PermitsRead for read, AuthorizeWrite otherwise.
func runtimeVerdict(ctx context.Context, t *testing.T, req *acl.Request, verb acl.Verb, typ, id string) bool {
	t.Helper()
	if verb == acl.VerbRead {
		ok, err := req.PermitsRead(ctx, typ, id)
		if err != nil {
			t.Fatalf("PermitsRead: %v", err)
		}
		return ok
	}
	var op acl.Op
	switch verb {
	case acl.VerbUpdate:
		op = acl.OpUpdate
	case acl.VerbDelete:
		op = acl.OpDelete
	case acl.VerbCreate:
		op = acl.OpCreate
	case acl.VerbRead:
		// handled above; unreachable here.
		return false
	}
	d := req.AuthorizeWrite(ctx, acl.WriteRequest{
		Op:      op,
		Subject: acl.EntitySubject{Type: typ, ID: id},
	})
	return d.Allow
}

func TestMapPrincipal_VerbAndTypeFilters(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	ctx := context.Background()

	// --verb read restricts the verb set.
	res, err := w.eng.MapPrincipal(ctx, "PERS-ALICE", acl.VerbRead, "", groundingTypes)
	if err != nil {
		t.Fatalf("MapPrincipal verb filter: %v", err)
	}
	if len(res.Verbs) != 1 || res.Verbs[0] != "read" {
		t.Errorf("--verb read should yield verbs [read]; got %v", res.Verbs)
	}
	for _, ta := range res.Types {
		for verb := range ta.Baseline {
			if verb != "read" {
				t.Errorf("verb filter leaked %q into baseline", verb)
			}
		}
	}

	// --type ticket restricts to one type.
	res2, err := w.eng.MapPrincipal(ctx, "PERS-ALICE", "", "ticket", groundingTypes)
	if err != nil {
		t.Fatalf("MapPrincipal type filter: %v", err)
	}
	for _, ta := range res2.Types {
		if ta.Type != "ticket" {
			t.Errorf("--type ticket leaked type %q", ta.Type)
		}
	}
}

func TestMapPrincipal_UnknownVerbErrors(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	_, err := w.eng.MapPrincipal(context.Background(), "PERS-ALICE", acl.Verb("frobnicate"), "", groundingTypes)
	if err == nil {
		t.Fatal("unknown verb must error")
	}
}

// ---- helpers ----

func typeNames(res *aclmap.MapPrincipalResult) []string {
	out := make([]string, 0, len(res.Types))
	for _, ta := range res.Types {
		out = append(out, ta.Type)
	}
	sort.Strings(out)
	return out
}
