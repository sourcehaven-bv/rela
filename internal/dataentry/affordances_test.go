package dataentry

import (
	"context"
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
			req, ok := translateVerb(c.verb, "ticket")
			if !ok {
				t.Fatalf("translateVerb(%q) returned ok=false", c.verb)
			}
			if req.Op != c.op {
				t.Errorf("Op = %q, want %q", req.Op, c.op)
			}
			if req.EntityType != "ticket" {
				t.Errorf("EntityType = %q, want %q", req.EntityType, "ticket")
			}
		})
	}
}

// TestTranslateVerb_Unknown asserts unknown verbs return ok=false so
// computeActions can skip them silently.
func TestTranslateVerb_Unknown(t *testing.T) {
	req, ok := translateVerb("transition:done", "ticket")
	if ok {
		t.Errorf("translateVerb(transition:done) returned ok=true; phase 1 doesn't support transition verbs yet")
	}
	if req != (acl.WriteRequest{}) {
		t.Errorf("expected zero WriteRequest on unknown verb, got %+v", req)
	}
}

// AC1: read-only principal sees all per-item verbs as false.
func TestComputeActions_ReadOnly(t *testing.T) {
	app := newTestAppV1(t)
	app.acl = acl.ReadOnlyACL{}

	ctx := authedCtx(t)
	e := &entity.Entity{ID: "TKT-001", Type: "ticket"}
	got := app.computeActions(ctx, e)

	if got == nil {
		t.Fatal("computeActions returned nil for authenticated principal")
	}
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

	ctx := authedCtx(t)
	e := &entity.Entity{ID: "TKT-001", Type: "ticket"}
	got := app.computeActions(ctx, e)

	if got == nil {
		t.Fatal("computeActions returned nil for authenticated principal")
	}
	for _, v := range []string{"update", "delete", "rename"} {
		if !got[v] {
			t.Errorf("_actions[%q] = false under NopACL, want true", v)
		}
	}
}

// TestComputeActions_AnonymousOmits asserts that an anonymous request
// (no Principal stamped) returns a nil map so the serializer omits the
// field — the SPA's fallback distinguishes anonymous (show all,
// silent) from authenticated-but-missing (show all, warn).
func TestComputeActions_AnonymousOmits(t *testing.T) {
	app := newTestAppV1(t)

	e := &entity.Entity{ID: "TKT-001", Type: "ticket"}
	got := app.computeActions(context.Background(), e)

	if got != nil {
		t.Errorf("expected nil _actions for anonymous principal, got %v", got)
	}
}

// AC4 part: collection actions returns nil for anonymous, expected
// verb set for authenticated.
func TestComputeCollectionActions_Anonymous(t *testing.T) {
	app := newTestAppV1(t)

	if got := app.computeCollectionActions(context.Background(), "ticket"); got != nil {
		t.Errorf("expected nil collection actions for anonymous, got %v", got)
	}
}

func TestComputeCollectionActions_Authenticated(t *testing.T) {
	app := newTestAppV1(t)
	app.acl = acl.ReadOnlyACL{}

	got := app.computeCollectionActions(authedCtx(t), "ticket")
	if got == nil {
		t.Fatal("expected non-nil collection actions for authenticated principal")
	}
	if _, ok := got["create"]; !ok {
		t.Errorf("expected 'create' key in collection actions; got %v", got)
	}
	if got["create"] {
		t.Errorf("expected create=false under ReadOnlyACL, got true")
	}
}

// authedCtx returns a context with a non-anonymous Principal stamped,
// so computeActions doesn't take the anonymous-fallback branch. Tool
// is ToolDataEntry to match production wiring; user is any non-empty
// string.
func authedCtx(t *testing.T) context.Context {
	t.Helper()
	return principal.With(context.Background(), principal.Principal{
		User: "test-user",
		Tool: principal.ToolDataEntry,
	})
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
