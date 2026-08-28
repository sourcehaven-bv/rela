package aclmap_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclmap"
)

// mapAllGraph runs MapAll over all verbs and all grounding types.
func mapAllGraph(t *testing.T, w *world) *aclmap.MapAllResult {
	t.Helper()
	res, err := w.eng.MapAll(context.Background(), "", "", groundingTypes)
	if err != nil {
		t.Fatalf("MapAll: %v", err)
	}
	return res
}

// principalOf returns the per-principal result for id in a MapAllResult, or nil.
func principalOf(res *aclmap.MapAllResult, id string) *aclmap.MapPrincipalResult {
	for i := range res.Principals {
		if res.Principals[i].Principal == id {
			return &res.Principals[i]
		}
	}
	return nil
}

// TestMapAll_EnumeratesEveryPrincipal: the whole-graph inventory covers
// every enumerated principal (assignment keys ∪ membership/role-relation
// edge sources), each appearing once, sorted.
func TestMapAll_EnumeratesEveryPrincipal(t *testing.T) {
	t.Parallel()
	res := mapAllGraph(t, groundingWorld(t))

	want := []string{
		"PERS-ALICE", "PERS-BOB", "PERS-CAROL", "PERS-DAVE", "PERS-EVE",
		"ROLE-IR", "ROLE-SECURITY",
	}
	var got []string
	for _, p := range res.Principals {
		got = append(got, p.Principal)
	}
	if !equal(got, want) {
		t.Errorf("principals:\n got: %v\nwant: %v", got, want)
	}
	if res.PrincipalCount != len(want) {
		t.Errorf("PrincipalCount = %d, want %d", res.PrincipalCount, len(want))
	}
	// Sorted, no duplicates.
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Errorf("principals not strictly sorted at %d: %v", i, got)
		}
	}
}

// TestMapAll_EveryoneBaselineLiftedOnce: the everyone grant is reported
// once at the top level, not repeated inside each principal's baseline.
func TestMapAll_EveryoneBaselineLiftedOnce(t *testing.T) {
	t.Parallel()
	res := mapAllGraph(t, groundingWorld(t))

	// everyone grants read:project — expect exactly one EveryoneBaseline entry.
	var projFound bool
	for _, et := range res.EveryoneBaseline {
		if et.Type == "project" && contains(et.Verbs, "read") {
			projFound = true
		}
	}
	if !projFound {
		t.Errorf("everyone read:project must appear in EveryoneBaseline; got %+v", res.EveryoneBaseline)
	}

	// No per-principal type baseline may carry the everyone role — it was
	// stripped.
	for _, p := range res.Principals {
		for _, ta := range p.Types {
			for verb, routes := range ta.Baseline {
				for _, r := range routes {
					if r.Role == acl.EveryoneRole {
						t.Errorf("%s: everyone route leaked into per-principal baseline %s/%s",
							p.Principal, ta.Type, verb)
					}
				}
			}
		}
	}
}

// TestMapAll_EqualsUnionOfPerPrincipal is the keystone conformance test:
// the whole-graph map must equal the union of per-principal MapPrincipal
// runs. For every enumerated principal, its access in MapAll (after the
// everyone baseline is stripped and re-added) must reproduce the verdict
// MapPrincipal gives standalone.
func TestMapAll_EqualsUnionOfPerPrincipal(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	ctx := context.Background()
	all := mapAllGraph(t, w)

	entities := []struct{ id, typ string }{
		{"INC-042", "incident"}, {"INC-999", "incident"}, {"TKT-1", "ticket"}, {"PROJ", "project"},
	}
	verbs := []string{"read", "update", "delete"}

	for _, p := range all.Principals {
		solo, err := w.eng.MapPrincipal(ctx, p.Principal, "", "", groundingTypes)
		if err != nil {
			t.Fatalf("MapPrincipal(%s): %v", p.Principal, err)
		}
		allEntry := principalOf(all, p.Principal)
		for _, e := range entities {
			for _, verb := range verbs {
				// The whole-graph entry strips the everyone baseline, so add it
				// back via the top-level EveryoneBaseline to compare like-for-like.
				allGrants := mapVerdict(allEntry, e.typ, e.id, verb) ||
					everyoneGrantsInAll(all, e.typ, verb)
				soloGrants := mapVerdict(solo, e.typ, e.id, verb)
				if allGrants != soloGrants {
					t.Errorf("%s %s %s: whole-graph=%v per-principal=%v",
						p.Principal, verb, e.id, allGrants, soloGrants)
				}
			}
		}
	}
}

// everyoneGrantsInAll reports whether the top-level everyone baseline
// grants verb on type typ.
func everyoneGrantsInAll(res *aclmap.MapAllResult, typ, verb string) bool {
	for _, et := range res.EveryoneBaseline {
		if et.Type == typ && contains(et.Verbs, verb) {
			return true
		}
	}
	return false
}

// TestMapAll_EveryoneOnlyPrincipalPreserved: a principal whose only access
// is the everyone baseline is kept in the inventory with EveryoneOnly set,
// not dropped — an offboarding review must see "cut off except everyone",
// not a missing row.
func TestMapAll_EveryoneOnlyPrincipalPreserved(t *testing.T) {
	t.Parallel()
	// ROLE-SECURITY is an assignment key (security role), so it holds
	// personal grants — not everyone-only. To get an everyone-only enumerated
	// principal we add a member with no other grant.
	w := buildWorld(t, groundingPolicy,
		[]ent{
			{"PERS-ALICE", "person"}, {"ROLE-SECURITY", "team"},
			{"PERS-LONE", "person"}, {"ROLE-EMPTY", "team"},
			{"TKT-1", "ticket"}, {"PROJ", "project"},
		},
		[]rel{
			// PERS-LONE is a member of ROLE-EMPTY, which has NO assignment — so
			// LONE is enumerated (a membership source) but holds only everyone.
			{"PERS-LONE", "member-of", "ROLE-EMPTY"},
		},
	)
	res := mapAllGraph(t, w)
	lone := principalOf(res, "PERS-LONE")
	if lone == nil {
		t.Fatalf("PERS-LONE (enumerated via membership) must appear in the inventory")
	}
	if !lone.EveryoneOnly {
		t.Errorf("PERS-LONE has no personal grant; EveryoneOnly must be true")
	}
}

// TestMapAll_GrantSourceCount counts non-everyone grant-sources across all
// principals; it must be > 0 in the grounding world and exclude the lifted
// everyone baseline.
func TestMapAll_GrantSourceCount(t *testing.T) {
	t.Parallel()
	res := mapAllGraph(t, groundingWorld(t))
	if res.GrantSourceCount == 0 {
		t.Errorf("grounding world has personal grants; GrantSourceCount must be > 0")
	}
	// Independently recount from the (everyone-stripped) principal results.
	recount := 0
	for _, p := range res.Principals {
		for _, ta := range p.Types {
			for _, routes := range ta.Baseline {
				recount += len(routes)
			}
			for _, ex := range ta.Exceptions {
				for _, routes := range ex.Extra {
					recount += len(routes)
				}
			}
		}
	}
	if recount != res.GrantSourceCount {
		t.Errorf("GrantSourceCount = %d, recount = %d", res.GrantSourceCount, recount)
	}
}

// TestMapAll_VerbAndTypeFilters: filters narrow the whole-graph output the
// same way they narrow the per-principal view.
func TestMapAll_VerbAndTypeFilters(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	ctx := context.Background()

	res, err := w.eng.MapAll(ctx, acl.VerbRead, "incident", groundingTypes)
	if err != nil {
		t.Fatalf("MapAll filtered: %v", err)
	}
	if len(res.Verbs) != 1 || res.Verbs[0] != "read" {
		t.Errorf("--verb read should yield verbs [read]; got %v", res.Verbs)
	}
	for _, p := range res.Principals {
		for _, ta := range p.Types {
			if ta.Type != "incident" {
				t.Errorf("--type incident leaked type %q for %s", ta.Type, p.Principal)
			}
			for verb := range ta.Baseline {
				if verb != "read" {
					t.Errorf("--verb read leaked verb %q for %s", verb, p.Principal)
				}
			}
		}
	}
}

// TestMapAll_BlankKeyDoesNotAbort: a malformed (blank) assignment key must
// not abort the whole-graph inventory — the rest of the principals must
// still be reported. A hard failure on one bad key is worse than a partial
// attestation for a compliance tool.
func TestMapAll_BlankKeyDoesNotAbort(t *testing.T) {
	t.Parallel()
	// A policy with a blank assignment key alongside a real one.
	const policy = `
membership_relation: member-of
roles:
  editor: { read: [incident], update: [incident] }
assignments:
  PERS-REAL: editor
  "       ": editor
role_relations: {}
`
	w := buildWorld(t, policy,
		[]ent{{"PERS-REAL", "person"}, {"INC-1", "incident"}},
		nil,
	)
	res, err := w.eng.MapAll(context.Background(), "", "", []string{"person", "incident"})
	if err != nil {
		t.Fatalf("a blank assignment key must not abort MapAll: %v", err)
	}
	// PERS-REAL still reported; the blank key contributes no phantom row.
	if principalOf(res, "PERS-REAL") == nil {
		t.Errorf("PERS-REAL must survive a blank sibling key; got principals %+v", res.Principals)
	}
	for _, p := range res.Principals {
		if p.Principal == "" {
			t.Errorf("a blank key must not add an empty-principal row")
		}
	}
}

// TestMapAll_UnknownVerbErrors: an invalid verb filter errors.
func TestMapAll_UnknownVerbErrors(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	_, err := w.eng.MapAll(context.Background(), acl.Verb("nope"), "", groundingTypes)
	if err == nil {
		t.Errorf("unknown verb must error")
	}
}
