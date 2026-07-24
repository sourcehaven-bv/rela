package appbuild

import (
	"context"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// scriptEntityReader / scriptTracer back BOTH the Services accessors and
// the cascade (automation) read deps built in assemble. The cascade bundle
// is captured inside a LuaScriptRunner with no accessor, so these
// package-internal tests pin the shared helpers directly — if a wiring
// site stops calling them, the black-box tests in luawiring_test.go catch
// that for the paths they cover (RR-QS4WQV).

const cascadePolicy = `
roles:
  viewer:
    read: [ticket]
assignments:
  alice: viewer
`

func mustDeclarative(t *testing.T, st *memstore.MemStore) *acl.Declarative {
	t.Helper()
	var policy acl.Policy
	if err := yaml.Unmarshal([]byte(cascadePolicy), &policy); err != nil {
		t.Fatalf("unmarshal policy: %v", err)
	}
	d, err := acl.NewDeclarative(&policy, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	return d
}

func seedCascadeWorld(t *testing.T) *memstore.MemStore {
	t.Helper()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "Readable"}},
		{ID: "SEC-1", Type: "secret", Properties: map[string]any{"title": "Classified"}},
	} {
		if err := st.CreateEntity(context.Background(), e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	return st
}

// TestScriptEntityReader_GatesOnActingIdentity pins the helper the cascade
// path uses: an automation triggered by alice reads ALICE's view, so an
// entity she cannot see stays invisible to the automation (DEC-O59WM4).
func TestScriptEntityReader_GatesOnActingIdentity(t *testing.T) {
	st := seedCascadeWorld(t)
	rd := scriptEntityReader(st, mustDeclarative(t, st), nil)

	ctx := principal.With(context.Background(), principal.Principal{
		User: "alice", Tool: principal.ToolDataEntry,
	})

	if _, err := rd.GetEntity(ctx, "TKT-1"); err != nil {
		t.Errorf("granted entity unreadable: %v", err)
	}
	if _, err := rd.GetEntity(ctx, "SEC-1"); err == nil {
		t.Error("cascade reads are NOT gated — an automation read an entity the " +
			"triggering user cannot see")
	}
}

// TestScriptEntityReader_NoPolicyIsPassThrough pins the NopACL path:
// without a Declarative the helper returns the raw store, so behavior is
// byte-identical to pre-ACL.
func TestScriptEntityReader_NoPolicyIsPassThrough(t *testing.T) {
	st := seedCascadeWorld(t)
	if got := scriptEntityReader(st, nil, nil); got != any(st) {
		t.Errorf("with no policy the helper must pass the raw store through, got %T", got)
	}
	if got := scriptTracer(tracer.New(st), st, nil, nil); got == nil {
		t.Error("scriptTracer returned nil with no policy")
	}
}

// TestScriptTracer_GatesOnActingIdentity pins the traversal helper.
func TestScriptTracer_GatesOnActingIdentity(t *testing.T) {
	st := seedCascadeWorld(t)
	if _, err := st.CreateRelation(context.Background(), "TKT-1", "relates", "SEC-1", nil); err != nil {
		t.Fatalf("seed relation: %v", err)
	}
	tr := scriptTracer(tracer.New(st), st, mustDeclarative(t, st), nil)

	ctx := principal.With(context.Background(), principal.Principal{
		User: "alice", Tool: principal.ToolDataEntry,
	})
	got := tr.TraceFrom(ctx, "TKT-1", 2)
	if got == nil {
		t.Fatal("visible root pruned from trace")
	}
	for _, child := range got.Children {
		if child.ID == "SEC-1" {
			t.Error("cascade traversal is NOT gated — a hidden node reached the automation")
		}
	}
}
