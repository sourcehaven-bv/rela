package aclmap_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclmap"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestCan_AllowViaGlobal: a global editor is allowed and the deciding
// route is reported.
func TestCan_AllowViaGlobal(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	// PERS-ALICE is a global editor: update on ticket.
	res, err := w.eng.Can(context.Background(), "PERS-ALICE", acl.VerbUpdate, "TKT-1")
	if err != nil {
		t.Fatalf("Can: %v", err)
	}
	if !res.Allowed {
		t.Fatalf("Alice (global editor) must be allowed to update TKT-1")
	}
	if len(res.Routes) == 0 {
		t.Errorf("an allow must carry its deciding route(s); got none")
	}
	if res.Principal != "PERS-ALICE" {
		t.Errorf("principal = %q, want PERS-ALICE", res.Principal)
	}
}

// TestCan_AllowViaException: a grant conferred only by a graph edge (an
// exception in map terms) is still an allow for the spot-check.
func TestCan_AllowViaException(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	// PERS-DAVE is editor-of FOLDER-Q3, which contains INC-042 — read via
	// inheritance.
	res, err := w.eng.Can(context.Background(), "PERS-DAVE", acl.VerbRead, "INC-042")
	if err != nil {
		t.Fatalf("Can: %v", err)
	}
	if !res.Allowed {
		t.Fatalf("Dave must be allowed to read INC-042 via folder inheritance")
	}
	// And denied on a sibling incident he does not reach.
	res999, err := w.eng.Can(context.Background(), "PERS-DAVE", acl.VerbRead, "INC-999")
	if err != nil {
		t.Fatalf("Can INC-999: %v", err)
	}
	if res999.Allowed {
		t.Errorf("Dave must NOT reach INC-999 (not in his folder)")
	}
}

// TestCan_DenyEveryoneOnly: a principal with only the everyone baseline is
// denied a non-everyone verb, and the deny is distinguishable (Allowed
// false, no routes).
func TestCan_DenyEveryoneOnly(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	// PERS-GHOST has no assignment/group/edge. everyone grants read:project
	// only, so update on a ticket must be denied.
	res, err := w.eng.Can(context.Background(), "PERS-GHOST", acl.VerbUpdate, "TKT-1")
	if err != nil {
		t.Fatalf("Can: %v", err)
	}
	if res.Allowed {
		t.Errorf("PERS-GHOST must be denied update on TKT-1")
	}
	if len(res.Routes) != 0 {
		t.Errorf("a deny carries no routes; got %+v", res.Routes)
	}
	if res.Principal != "PERS-GHOST" {
		t.Errorf("a deny must still name the principal; got %q", res.Principal)
	}
}

// TestCan_AllowViaEveryone: when everyone grants the verb, a principal with
// no personal route is still allowed, flagged as an everyone grant.
func TestCan_AllowViaEveryone(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	// everyone can read:project; PERS-GHOST holds no personal grant.
	res, err := w.eng.Can(context.Background(), "PERS-GHOST", acl.VerbRead, "PROJ")
	if err != nil {
		t.Fatalf("Can: %v", err)
	}
	if !res.Allowed {
		t.Fatalf("everyone grants read:project — PERS-GHOST must be allowed")
	}
	if !res.Everyone {
		t.Errorf("an everyone-only allow must set Everyone=true")
	}
	if len(res.Routes) != 0 {
		t.Errorf("everyone grant carries no per-principal routes; got %+v", res.Routes)
	}
}

// TestCan_MissingEntity errors distinctly rather than silently denying — a
// typo'd entity ID must not read as "access denied".
func TestCan_MissingEntity(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	_, err := w.eng.Can(context.Background(), "PERS-ALICE", acl.VerbRead, "NO-SUCH")
	if !errors.Is(err, aclmap.ErrEntityNotFound) {
		t.Errorf("missing entity must error with ErrEntityNotFound; got %v", err)
	}
}

// TestCan_UnknownVerbErrors: an invalid verb is a hard error, not a deny.
func TestCan_UnknownVerbErrors(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	_, err := w.eng.Can(context.Background(), "PERS-ALICE", acl.Verb("frobnicate"), "TKT-1")
	if err == nil {
		t.Errorf("unknown verb must error")
	}
}

// TestCan_MatchesRuntime is the anti-false-negative guard for the
// spot-check: Can's verdict must equal the runtime PermitsRead /
// AuthorizeWrite decision for every (principal, verb, entity) probed.
func TestCan_MatchesRuntime(t *testing.T) {
	t.Parallel()
	w := groundingWorld(t)
	ctx := context.Background()

	principalsUnderTest := []string{
		"PERS-ALICE", "PERS-BOB", "PERS-CAROL", "PERS-DAVE", "PERS-EVE", "PERS-GHOST",
	}
	entities := []struct{ id, typ string }{
		{"INC-042", "incident"}, {"INC-999", "incident"}, {"TKT-1", "ticket"}, {"PROJ", "project"},
	}
	verbs := []acl.Verb{acl.VerbRead, acl.VerbUpdate, acl.VerbDelete}

	for _, prin := range principalsUnderTest {
		req, err := w.decl.ForPrincipal(principal.Principal{User: prin, Tool: principal.ToolCLI})
		if err != nil {
			t.Fatalf("ForPrincipal %s: %v", prin, err)
		}
		for _, e := range entities {
			for _, verb := range verbs {
				res, cErr := w.eng.Can(ctx, prin, verb, e.id)
				if cErr != nil {
					t.Fatalf("Can(%s,%s,%s): %v", prin, verb, e.id, cErr)
				}
				runtime := runtimeVerdict(ctx, t, req, verb, e.typ, e.id)
				if res.Allowed != runtime {
					t.Errorf("disagreement: %s %s %s — can=%v runtime=%v",
						prin, verb, e.id, res.Allowed, runtime)
				}
			}
		}
	}
}
