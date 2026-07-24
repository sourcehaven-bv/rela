package appbuild_test

import (
	"context"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TKT-ZF2DTV: these pin the SCHEDULER wiring — the path that carries
// ACL-bound LLM jobs. internal/lua proves the reader gates; this proves
// Services actually hands a gated reader to a scheduled task. Without it,
// repointing ScheduledLuaWriteDeps at the raw store passes silently
// (RR-QS4WQV).

const schedPolicy = `
roles:
  reporting:
    read: [ticket]
assignments:
  system:reports: reporting
`

func schedMeta() *metamodel.Metamodel {
	str := metamodel.PropertyDef{Type: metamodel.PropertyTypeString}
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{"title": str}},
			"secret": {Properties: map[string]metamodel.PropertyDef{"title": str}},
		},
	}
}

// TestScheduledLuaWriteDeps_ReadsAreACLBound is the role-scoped job proof:
// a task running as `system:reports` — an identity granted only `ticket`
// in acl.yaml — cannot read a `secret`, so that entity can never enter a
// prompt the job builds.
func TestScheduledLuaWriteDeps_ReadsAreACLBound(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "Readable"}},
		{ID: "SEC-1", Type: "secret", Properties: map[string]any{"title": "Classified"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}

	var policy acl.Policy
	if err := yaml.Unmarshal([]byte(schedPolicy), &policy); err != nil {
		t.Fatalf("unmarshal policy: %v", err)
	}
	d, err := acl.NewDeclarative(&policy, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	svc := appbuildtest.New(schedMeta(), appbuildtest.WithStore(st), appbuildtest.WithDeclarative(d))
	defer svc.Close()

	// The scheduler stamps the task's identity on the ctx; the deps bundle
	// is identity-agnostic and resolves per call.
	taskCtx := principal.With(ctx, principal.Principal{
		User: "system:reports", Tool: principal.ToolScheduler,
	})
	var out strings.Builder
	rt := lua.NewWriter(svc.ScheduledLuaWriteDeps(), &out,
		lua.WithContext(taskCtx), lua.WithPrincipal(principal.From(taskCtx)))
	defer rt.Close()

	if err := rt.RunString(`
rela.output("ticket=" .. tostring(rela.get_entity("TKT-1") ~= nil))
rela.output("secret=" .. tostring(rela.get_entity("SEC-1") ~= nil))
`); err != nil {
		t.Fatalf("script error: %v (output %q)", err, out.String())
	}

	got := out.String()
	if !strings.Contains(got, "ticket=true") {
		t.Errorf("granted type unreadable by the scheduled job: %q", got)
	}
	if !strings.Contains(got, "secret=false") {
		t.Errorf("ScheduledLuaWriteDeps is NOT ACL-bound — a scheduled job read an "+
			"entity its identity has no grant for; this is what would enter a prompt: %q", got)
	}
}

// TestScheduledLuaWriteDeps_WritePrepStaysRaw pins the write-prep half at
// the scheduler wiring level.
func TestScheduledLuaWriteDeps_WritePrepStaysRaw(t *testing.T) {
	st := memstore.New()
	var policy acl.Policy
	if err := yaml.Unmarshal([]byte(schedPolicy), &policy); err != nil {
		t.Fatalf("unmarshal policy: %v", err)
	}
	d, err := acl.NewDeclarative(&policy, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	svc := appbuildtest.New(schedMeta(), appbuildtest.WithStore(st), appbuildtest.WithDeclarative(d))
	defer svc.Close()

	deps := svc.ScheduledLuaWriteDeps()
	if deps.WritePrepStore == nil {
		t.Fatal("WritePrepStore is nil — rela.update_entity would fail")
	}
	if deps.VisibleReader == lua.EntityReader(deps.WritePrepStore) {
		t.Error("VisibleReader and WritePrepStore are the same handle under a configured " +
			"policy — either reads are ungated, or update_entity would erase hidden properties")
	}
}
