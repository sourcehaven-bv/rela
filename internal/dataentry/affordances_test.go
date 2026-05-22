package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestTranslateVerb_Roundtrip pins the phase-1 verb vocabulary against
// acl.Op constants. The grep test (AC10) only proves no one *else*
// constructs WriteRequest{Op:...}; this proves the central translation
// is correct.
func TestTranslateVerb_Roundtrip(t *testing.T) {
	cases := []struct {
		verb string
		op   acl.Op
	}{
		{"create", acl.OpCreate},
		{"update", acl.OpUpdate},
		{"delete", acl.OpDelete},
		{"rename", acl.OpRename},
	}
	for _, c := range cases {
		t.Run(c.verb, func(t *testing.T) {
			req := translateVerb(c.verb, "ticket")
			if req.Op != c.op {
				t.Errorf("Op = %q, want %q", req.Op, c.op)
			}
			if req.EntityType != "ticket" {
				t.Errorf("EntityType = %q, want %q", req.EntityType, "ticket")
			}
		})
	}
}

// TestTranslateVerb_UnknownPanics asserts the "unreachable for the
// closed set" contract. If a future change adds a verb to
// [perItemVerbs] / [perCollectionVerbs] without adding the matching
// translateVerb case, this is the test that fails loudly instead of
// the production deserializer silently returning the zero WriteRequest
// (which would map every misspelled verb to OpCreate).
func TestTranslateVerb_UnknownPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on unknown verb; got none")
		}
	}()
	translateVerb("transition:done", "ticket")
}

// AC1: read-only principal sees all per-item verbs as false.
func TestComputeActions_ReadOnly(t *testing.T) {
	app := newTestAppV1(t)
	app.acl = acl.ReadOnlyACL{}

	e := &entity.Entity{ID: "TKT-001", Type: "ticket"}
	got := app.computeActions(t.Context(), e)

	for _, v := range []string{"update", "delete", "rename"} {
		if got[v] {
			t.Errorf("_actions[%q] = true under ReadOnlyACL, want false", v)
		}
	}
}

// AC2: NopACL principal sees all per-item verbs as true.
func TestComputeActions_NopACL(t *testing.T) {
	app := newTestAppV1(t)
	// app.acl is already acl.NopACL via the test fixture wiring.

	e := &entity.Entity{ID: "TKT-001", Type: "ticket"}
	got := app.computeActions(t.Context(), e)

	for _, v := range []string{"update", "delete", "rename"} {
		if !got[v] {
			t.Errorf("_actions[%q] = false under NopACL, want true", v)
		}
	}
}

// AC4: collection-scope verb computed under ReadOnlyACL is false.
func TestComputeCollectionActions_ReadOnly(t *testing.T) {
	app := newTestAppV1(t)
	app.acl = acl.ReadOnlyACL{}

	got := app.computeCollectionActions(t.Context(), "ticket")
	if got["create"] {
		t.Errorf("_actions.create = true under ReadOnlyACL, want false")
	}
}

// TestComputeActions_MixedTypeDeclarative is AC12 (TKT-LFT2). A
// Declarative policy grants write on `ticket` but not on `feature`;
// `computeActions` should return all-true for the ticket and
// all-false for the feature, demonstrating cross-type variance.
// (Per-row within-type variance is gated on TKT-XZEY since ACL v0's
// WriteRequest carries no entity ID.)
func TestComputeActions_MixedTypeDeclarative(t *testing.T) {
	app := newTestAppV1(t)
	app.acl = acl.NewDeclarative(&acl.Policy{
		UserEntityType: "person",
		Roles: map[string]acl.RoleDef{
			"ticket-writer": {Write: []string{"ticket"}},
		},
		Assignments: map[string]string{
			"test-user": "ticket-writer",
		},
	})

	// Declarative looks up the principal by Principal.User against
	// Assignments. Stamp a matching User on ctx.
	ctx := principal.With(t.Context(), principal.Principal{
		User: "test-user",
		Tool: principal.ToolDataEntry,
	})

	ticket := &entity.Entity{ID: "TKT-001", Type: "ticket"}
	got := app.computeActions(ctx, ticket)
	for _, v := range []string{"update", "delete", "rename"} {
		if !got[v] {
			t.Errorf("ticket _actions[%q] = false under ticket-writer role, want true", v)
		}
	}

	feature := &entity.Entity{ID: "FEAT-001", Type: "feature"}
	got = app.computeActions(ctx, feature)
	for _, v := range []string{"update", "delete", "rename"} {
		if got[v] {
			t.Errorf("feature _actions[%q] = true under ticket-writer role, want false", v)
		}
	}
}

// TestComputeActions_NoAuditNoise is AC8 — read-time `AuthorizeWrite`
// calls in computeActions are not writes, so they must not produce
// audit records. The audit sink lives on entitymanager.Manager (the
// write path); the read path doesn't touch it. We wire a Memory audit
// sink into the EntityManager and assert that a GET (which triggers
// per-entity + per-collection affordance computation) records zero
// audit entries.
func TestComputeActions_NoAuditNoise(t *testing.T) {
	cases := []acl.ACL{acl.NopACL{}, acl.ReadOnlyACL{}}
	for _, a := range cases {
		t.Run("", func(t *testing.T) {
			sink := audit.NewMemory()
			app := buildAppWithACLAndAudit(t, a, sink)
			seedEntity(app, &entity.Entity{
				ID: "TKT-001", Type: "ticket",
				Properties: map[string]interface{}{"title": "audit-test"},
			})

			// Per-entity GET triggers computeActions +
			// computeCollectionActions — both read-only.
			_ = fetchActions(t, app, "ticket", "tickets", "TKT-001")

			if got := len(sink.Records()); got != 0 {
				t.Errorf("expected 0 audit records on read path, got %d: %+v", got, sink.Records())
			}
		})
	}
}
