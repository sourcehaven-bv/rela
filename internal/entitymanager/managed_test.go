package entitymanager_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// newFlowRunManager wires a Manager over a schema with flow runs and the
// given ACL. The managed-type rule must hold whatever the ACL says.
func newFlowRunManager(t *testing.T, a acl.ACL, sink audit.Audit) (*entitymanager.Manager, store.Store) {
	t.Helper()
	meta, err := metamodel.WithFlowRuns(parseMeta(t))
	if err != nil {
		t.Fatalf("WithFlowRuns: %v", err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:       st,
		Meta:        meta,
		Templater:   nopTemplater{},
		Audit:       sink,
		ACL:         a,
		Transitions: flowcfg.NoTransitions(),
		FieldGate:   entitymanager.AllowAllFieldGate{},
		AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr, st
}

func flowRun() *entity.Entity {
	e := entity.New("", entity.FlowRunType)
	e.SetString(metamodel.FlowRunPropFlow, "review")
	e.SetString(metamodel.FlowRunPropVersion, "abc")
	e.SetString(metamodel.FlowRunPropStatus, "working")
	return e
}

func requireManagedDenial(t *testing.T, what string, err error) {
	t.Helper()
	var fe *acl.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("%s: got %v, want *acl.ForbiddenError", what, err)
	}
	if fe.Decision.RuleKind != "managed-type" {
		t.Fatalf("%s: RuleKind = %q, want managed-type", what, fe.Decision.RuleKind)
	}
}

// TestManagedTypeGuard pins the write-path integrity rule of flow runs
// (TKT-BC5G8RP): only system:flow writes a `_flow-run` or a `_carries`
// edge, on a NopACL project and through an elevated handle too, and nobody
// renames a run. Deleting a run's subject removes the run's edge.
func TestManagedTypeGuard(t *testing.T) {
	t.Parallel()
	sink := audit.NewMemory()
	mgr, st := newFlowRunManager(t, acl.NopACL{}, sink)
	alice := principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	engine := principal.With(context.Background(),
		principal.Principal{User: principal.UserFlow, Tool: principal.ToolFlow})

	subject := entity.New("", "requirement")
	subject.SetString("title", "Subject")
	sres, err := mgr.CreateEntity(alice, subject, entity.CreateOptions{})
	if err != nil {
		t.Fatalf("create subject: %v", err)
	}
	subjectID := sres.Entity.ID

	before := len(sink.Records())
	_, err = mgr.CreateEntity(alice, flowRun(), entity.CreateOptions{})
	requireManagedDenial(t, "alice creates a run", err)
	denied := 0
	for _, rec := range sink.Records()[before:] {
		if rec.Op == audit.OpDeniedWrite {
			denied++
		}
	}
	if denied != 1 {
		t.Errorf("denied-write rows = %d, want 1", denied)
	}

	elevated := any(mgr).(elevatedProvider).Elevated()
	_, err = elevated.CreateEntity(alice, flowRun(), entity.CreateOptions{})
	requireManagedDenial(t, "alice creates a run with bypass_acl", err)

	rres, err := mgr.CreateEntity(engine, flowRun(), entity.CreateOptions{})
	if err != nil {
		t.Fatalf("engine creates a run: %v", err)
	}
	runID := rres.Entity.ID
	carries := entity.RelationKey{From: runID, Type: entity.CarriesRelation, To: subjectID}

	_, err = mgr.CreateRelation(alice, carries, entity.RelationOptions{})
	requireManagedDenial(t, "alice links a run", err)
	if _, err = mgr.CreateRelation(engine, carries, entity.RelationOptions{}); err != nil {
		t.Fatalf("engine links a run: %v", err)
	}

	_, err = mgr.PatchEntity(alice, runID, entity.Patch{Properties: map[string]any{"status": "failed"}})
	requireManagedDenial(t, "alice updates a run", err)
	_, err = elevated.DeleteEntity(alice, runID, true)
	requireManagedDenial(t, "alice deletes a run with bypass_acl", err)
	err = mgr.DeleteRelation(alice, carries)
	requireManagedDenial(t, "alice unlinks a run", err)

	// A cascade write reaches the store without authorizeAndAudit; the
	// rule holds there too.
	_, err = entitymanager.CascadeHostCreate(alice, mgr, entity.FlowRunType, "", nil)
	requireManagedDenial(t, "a cascade creates a run", err)
	err = entitymanager.CascadeHostWriteRelation(alice, mgr,
		&entity.Relation{From: runID, Type: entity.ProducedRelation, To: subjectID})
	requireManagedDenial(t, "a cascade links a run", err)
	// A run's id is generated, so the id_type check refuses the rename
	// first; the managed-type rule backs it up (TestManagedWriteDenial).
	if _, err := mgr.RenameEntity(engine, runID, "RUN-RENAMED", entity.RenameOptions{}); err == nil {
		t.Error("engine renamed a run")
	}

	// The run records its subject; it does not keep the subject alive.
	if _, err := mgr.DeleteEntity(alice, subjectID, true); err != nil {
		t.Fatalf("alice deletes the subject: %v", err)
	}
	if _, err := st.GetEntity(engine, rres.Entity.Ref()); err != nil {
		t.Errorf("run after its subject was deleted: %v", err)
	}
	if _, err := st.GetRelation(engine, carries); err == nil {
		t.Error("the run still carries the deleted subject")
	}
}

// TestFlowProvenance pins that a write made in a flow step records the
// flow, run and step, and that both it and an ordinary write carry a
// source (TKT-BC5G8RP).
func TestFlowProvenance(t *testing.T) {
	t.Parallel()
	sink := audit.NewMemory()
	mgr, _ := newFlowRunManager(t, acl.NopACL{}, sink)
	alice := principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	engine := principal.With(context.Background(),
		principal.Principal{User: principal.UserFlow, Tool: principal.ToolFlow})
	step := audit.WithFlowStep(engine, audit.FlowStep{Flow: "review", Run: "RUN-1", Step: "open"})

	e := entity.New("", "requirement")
	e.SetString("title", "By hand")
	if _, err := mgr.CreateEntity(alice, e, entity.CreateOptions{}); err != nil {
		t.Fatalf("alice create: %v", err)
	}
	if _, err := mgr.CreateEntity(step, flowRun(), entity.CreateOptions{}); err != nil {
		t.Fatalf("engine create: %v", err)
	}
	recs := sink.Records()
	if len(recs) != 2 {
		t.Fatalf("records = %d, want 2: %+v", len(recs), recs)
	}
	if r := recs[0]; r.Flow != "" || r.Source != principal.ToolDataEntry {
		t.Errorf("direct write: flow=%q source=%q, want no flow and data-entry", r.Flow, r.Source)
	}
	r := recs[1]
	if r.Flow != "review" || r.Run != "RUN-1" || r.Step != "open" || r.Source != audit.SourceFlow {
		t.Errorf("flow write = flow %q run %q step %q source %q, want review/RUN-1/open/flow",
			r.Flow, r.Run, r.Step, r.Source)
	}
	if r.Principal.User != principal.UserFlow {
		t.Errorf("flow write principal = %q, want %q", r.Principal.User, principal.UserFlow)
	}
}
