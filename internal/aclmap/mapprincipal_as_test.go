package aclmap_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/aclmap"
)

// `rela acl map --as` (TKT-IAC8TX).
//
// The reason this needs its own coverage rather than trusting the acl-layer
// tests: aclmap builds its OWN principal, and before this feature it built a
// plain composite literal — which cannot carry the attenuation claims at all
// (they are unexported by design). So an attestation tool could silently report
// un-attenuated access for a client that is in fact restricted. That is the
// worst failure class for a tool whose entire job is answering "who can do
// what".

// attenuatedPolicy extends the grounding policy with a client baseline that
// makes app-type clients read-only, and a scope that hands back ticket writes.
const attenuatedPolicy = groundingPolicy + `
client_baselines:
  apps:
    applies_to: [app]
    deny_write: ["*"]
    deny_read: [incident]
scope_grants:
  rela.tickets.write:
    update: [ticket]
`

func attenuatedWorld(t *testing.T) *world {
	t.Helper()
	return buildWorld(t, attenuatedPolicy,
		[]ent{
			{"PERS-ALICE", "person"},
			{"INC-042", "incident"}, {"TKT-1", "ticket"}, {"PROJ", "project"},
		},
		nil,
	)
}

// mapAs maps PERS-ALICE under a client view. The principal is fixed because
// these tests vary the CLIENT, not the user — that is the whole point: one
// principal, different access depending on what it connects through.
func mapAs(t *testing.T, w *world, view aclmap.ClientView) *aclmap.MapPrincipalResult {
	t.Helper()
	res, err := w.eng.MapPrincipalAs(
		context.Background(), "PERS-ALICE", "", "", groundingTypes, view)
	if err != nil {
		t.Fatalf("MapPrincipalAs(PERS-ALICE, %+v): %v", view, err)
	}
	return res
}

// TestMapPrincipalAs_ReportsAttenuatedAccess is the headline: Alice is a global
// editor with full ticket CRUD, but an app-type client acting as her is
// read-only and cannot reach incidents at all.
func TestMapPrincipalAs_ReportsAttenuatedAccess(t *testing.T) {
	t.Parallel()
	w := attenuatedWorld(t)

	// Alice herself: full editor access, as the un-attenuated map reports.
	direct := mapAs(t, w, aclmap.ClientView{})
	tk := typeOf(direct, "ticket")
	if tk == nil {
		t.Fatal("precondition: Alice should have ticket access")
	}
	for _, verb := range []string{"read", "create", "update", "delete"} {
		if len(tk.Baseline[verb]) == 0 {
			t.Fatalf("precondition: Alice's ticket baseline is missing %s", verb)
		}
	}
	if typeOf(direct, "incident") == nil {
		t.Fatal("precondition: Alice should be able to read incidents")
	}

	// The same Alice, through an app client: reads survive, writes do not.
	viaApp := mapAs(t, w, aclmap.ClientView{PrincipalType: "app"})
	tkApp := typeOf(viaApp, "ticket")
	if tkApp == nil {
		t.Fatal("ticket access vanished entirely; deny_write should leave read intact")
	}
	if len(tkApp.Baseline["read"]) == 0 {
		t.Error("ticket read lost; the baseline only denies writes")
	}
	for _, verb := range []string{"create", "update", "delete"} {
		if len(tkApp.Baseline[verb]) > 0 {
			t.Errorf("ticket %s still reported under deny_write: [\"*\"]", verb)
		}
	}
	// deny_read on incident is row-level: the type drops out of the map.
	if typeOf(viaApp, "incident") != nil {
		t.Error("incident still reported despite deny_read")
	}
}

// TestMapPrincipalAs_ScopeReopens: the map must reflect scopes too, or an
// operator debugging "why can this token write?" gets a map that disagrees with
// the runtime.
func TestMapPrincipalAs_ScopeReopens(t *testing.T) {
	t.Parallel()
	w := attenuatedWorld(t)

	withScope := mapAs(t, w, aclmap.ClientView{
		PrincipalType: "app",
		Scopes:        []string{"rela.tickets.write"},
	})
	tk := typeOf(withScope, "ticket")
	if tk == nil {
		t.Fatal("no ticket access reported")
	}
	if len(tk.Baseline["update"]) == 0 {
		t.Error("ticket update missing; rela.tickets.write should re-open it")
	}
	// The scope named only update — create and delete stay denied.
	for _, verb := range []string{"create", "delete"} {
		if len(tk.Baseline[verb]) > 0 {
			t.Errorf("ticket %s reported; the scope re-opened only update", verb)
		}
	}
}

// TestMapPrincipalAs_EchoesTheClient: an artifact that does not say which
// client it describes is actively misleading, because the same principal
// produces different maps through different clients.
func TestMapPrincipalAs_EchoesTheClient(t *testing.T) {
	t.Parallel()
	w := attenuatedWorld(t)

	res := mapAs(t, w, aclmap.ClientView{
		PrincipalType: "app",
		Scopes:        []string{"rela.tickets.write"},
	})
	if res.As != "app" {
		t.Errorf("As = %q, want app", res.As)
	}
	if len(res.Scopes) != 1 || res.Scopes[0] != "rela.tickets.write" {
		t.Errorf("Scopes = %v, want [rela.tickets.write]", res.Scopes)
	}

	// And an un-attenuated map must not claim a client it did not use.
	plain := mapAs(t, w, aclmap.ClientView{})
	if plain.As != "" || len(plain.Scopes) != 0 {
		t.Errorf("un-attenuated map echoed As=%q Scopes=%v", plain.As, plain.Scopes)
	}
}

// TestMapPrincipalAs_ZeroViewMatchesMapPrincipal pins that the new entry point
// is a strict superset: existing callers must see byte-identical results.
func TestMapPrincipalAs_ZeroViewMatchesMapPrincipal(t *testing.T) {
	t.Parallel()
	w := attenuatedWorld(t)
	ctx := context.Background()

	old, err := w.eng.MapPrincipal(ctx, "PERS-ALICE", "", "", groundingTypes)
	if err != nil {
		t.Fatalf("MapPrincipal: %v", err)
	}
	zero := mapAs(t, w, aclmap.ClientView{})

	if len(old.Types) != len(zero.Types) {
		t.Fatalf("type count differs: %d vs %d", len(old.Types), len(zero.Types))
	}
	for i := range old.Types {
		if old.Types[i].Type != zero.Types[i].Type {
			t.Errorf("type[%d] = %q vs %q", i, old.Types[i].Type, zero.Types[i].Type)
		}
		for verb, routes := range old.Types[i].Baseline {
			if len(zero.Types[i].Baseline[verb]) != len(routes) {
				t.Errorf("%s.%s route count differs", old.Types[i].Type, verb)
			}
		}
	}
	if old.EveryoneOnly != zero.EveryoneOnly {
		t.Errorf("EveryoneOnly differs: %v vs %v", old.EveryoneOnly, zero.EveryoneOnly)
	}
}

// TestMapPrincipalAs_UnmatchedTypeIsUnrestricted: a principal_type no baseline
// covers must report the principal's own access unchanged (AC3), so an operator
// checking a type they have not configured is not misled into thinking it is
// restricted.
func TestMapPrincipalAs_UnmatchedTypeIsUnrestricted(t *testing.T) {
	t.Parallel()
	w := attenuatedWorld(t)

	plain := mapAs(t, w, aclmap.ClientView{})
	other := mapAs(t, w, aclmap.ClientView{PrincipalType: "some-other-type"})

	if len(other.Types) != len(plain.Types) {
		t.Errorf("unmatched principal_type changed the map: %d types vs %d",
			len(other.Types), len(plain.Types))
	}
	if typeOf(other, "incident") == nil {
		t.Error("incident dropped for an unmatched principal_type; no baseline covers it")
	}
}
