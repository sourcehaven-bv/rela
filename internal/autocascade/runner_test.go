package autocascade_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// --- New / construction tests ---

func TestNew_RejectsNilEngine(t *testing.T) {
	if _, err := autocascade.New(autocascade.Deps{
		Engine:  nil,
		Scripts: autocascade.NopExecutor,
	}); err == nil {
		t.Fatal("expected error for nil Engine, got nil")
	}
}

func TestNew_RejectsNilScripts(t *testing.T) {
	if _, err := autocascade.New(autocascade.Deps{
		Engine:  automation.NewEngine(nil),
		Scripts: nil,
	}); err == nil {
		t.Fatal("expected error for nil Scripts, got nil")
	}
}

// --- Stub Host for unit tests ---

// stubHost records calls and returns canned results. Only the methods
// each test actually exercises are programmed; the rest return zero
// values that would surface as test failures if Runner called them.
//
// Calls is a single ordered log so action-order tests can assert
// dispatch sequence across the four action paths.
type stubHost struct {
	t *testing.T

	meta  *metamodel.Metamodel
	store store.Store

	// Programmed behavior.
	createErr      error // returned by CreateEntityNoCascade
	writeRelErr    error // returned by WriteRelation
	existingTarget *entity.Entity
	createCounter  int // suffix for unique auto-IDs

	// Call log.
	Calls []string
}

func (h *stubHost) Meta() *metamodel.Metamodel { return h.meta }
func (h *stubHost) Store() store.Store         { return h.store }

func (h *stubHost) CreateEntityNoCascade(entityType string, opts autocascade.CreateEntityOptions) (*entity.Entity, error) {
	h.Calls = append(h.Calls, "CreateEntityNoCascade:"+entityType)
	if h.createErr != nil {
		return nil, h.createErr
	}
	h.createCounter++
	id := opts.ID
	if id == "" {
		id = fmt.Sprintf("AUTO-%03d", h.createCounter)
	}
	e := entity.New(id, entityType)
	for k, v := range opts.Properties {
		e.Properties[k] = v
	}
	if h.store != nil {
		if err := h.store.CreateEntity(context.Background(), e); err != nil {
			h.t.Fatalf("stub CreateEntity: %v", err)
		}
	}
	return e, nil
}

func (h *stubHost) WriteEntity(e *entity.Entity) error {
	h.Calls = append(h.Calls, "WriteEntity:"+e.ID)
	if h.store == nil {
		return nil
	}
	// WriteEntity is an upsert in workspace; reproduce that here by
	// trying Create first, then Update if-and-only-if the failure is
	// store.ErrConflict. Any other error propagates.
	if err := h.store.CreateEntity(context.Background(), e); err != nil {
		if !errors.Is(err, store.ErrConflict) {
			return err
		}
		return h.store.UpdateEntity(context.Background(), e)
	}
	return nil
}

func (h *stubHost) WriteRelation(r *entity.Relation) error {
	h.Calls = append(h.Calls, "WriteRelation:"+r.From+"--"+r.Type+"-->"+r.To)
	if h.writeRelErr != nil {
		return h.writeRelErr
	}
	if h.store != nil {
		if _, err := h.store.CreateRelation(context.Background(), r.From, r.Type, r.To, nil); err != nil {
			return err
		}
	}
	return nil
}

func (h *stubHost) DeleteEntity(_ context.Context, entityType, id string, cascade bool) error {
	h.Calls = append(h.Calls, fmt.Sprintf("DeleteEntity:%s:%s:%v", entityType, id, cascade))
	return nil
}

func (h *stubHost) FindExistingRelationTarget(sourceID, relationType, targetType string) *entity.Entity {
	h.Calls = append(h.Calls, "FindExistingRelationTarget:"+sourceID+":"+relationType+":"+targetType)
	return h.existingTarget
}

// --- Helpers ---

// newRunner constructs a Runner with a freshly built no-rules Engine.
// Tests that need automation rules build the engine inline.
func newRunner(t *testing.T, engine *automation.Engine, scripts autocascade.Executor) *autocascade.Runner {
	t.Helper()
	if engine == nil {
		engine = automation.NewEngine(nil)
	}
	if scripts == nil {
		scripts = autocascade.NopExecutor
	}
	r, err := autocascade.New(autocascade.Deps{Engine: engine, Scripts: scripts})
	if err != nil {
		t.Fatalf("autocascade.New: %v", err)
	}
	return r
}

// emptyRequest returns a Request that triggers no work.
func emptyRequest(trigger *entity.Entity) autocascade.Request {
	return autocascade.Request{
		Trigger: trigger,
		Result:  &automation.Result{},
	}
}

// --- AC5 tests ---

func TestRunnerEmptyResult(t *testing.T) {
	r := newRunner(t, nil, nil)
	trigger := entity.New("REQ-001", "requirement")

	host := &stubHost{t: t}
	outcome, err := r.Process(context.Background(), host, emptyRequest(trigger))
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(outcome.EntitiesCreated) != 0 || len(outcome.RelationsCreated) != 0 ||
		len(outcome.Errors) != 0 || len(outcome.Warnings) != 0 {

		t.Errorf("expected empty outcome, got %+v", outcome)
	}
	if len(host.Calls) != 0 {
		t.Errorf("expected no Host calls, got %v", host.Calls)
	}
}

// TestRunnerDepthLimit pins the depth-limit warning wording.
// The format string lived in workspace.go:1082–1086 before the move.
func TestRunnerDepthLimit(t *testing.T) {
	auto := automation.Automation{
		Name: "chain-recursion",
		On: automation.Trigger{
			Entity:  []string{"chain"},
			Created: true,
		},
		Do: []automation.Action{
			{
				CreateEntity: &automation.CreateEntityAction{
					Type: "chain",
				},
			},
		},
	}
	r := newRunner(t, automation.NewEngine([]automation.Automation{auto}), nil)
	host := &stubHost{t: t, store: memstore.New()}

	starter := entity.New("CHAIN-000", "chain")
	req := autocascade.Request{
		Trigger: starter,
		Result: &automation.Result{
			EntitiesToCreate: []automation.EntityToCreate{
				{Type: "chain"},
			},
		},
	}
	outcome, err := r.Process(context.Background(), host, req)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	expectedWarning := fmt.Sprintf("automation iteration limit (%d) reached; 1 pending items skipped", autocascade.MaxDepth)
	found := false
	for _, w := range outcome.Warnings {
		if w == expectedWarning {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning %q, got warnings: %v", expectedWarning, outcome.Warnings)
	}
}

// TestRunnerIfExistsSkip — existing target found, IfExistsSkip: no
// CreateEntityNoCascade.
func TestRunnerIfExistsSkip(t *testing.T) {
	r := newRunner(t, nil, nil)
	existing := entity.New("EXISTING-1", "checklist")
	host := &stubHost{t: t, existingTarget: existing}

	trigger := entity.New("REQ-001", "requirement")
	req := autocascade.Request{
		Trigger: trigger,
		Result: &automation.Result{
			EntitiesToCreate: []automation.EntityToCreate{
				{
					Type:                "checklist",
					RelationFromTrigger: "has-checklist",
					IfExists:            automation.IfExistsSkip,
				},
			},
		},
	}
	outcome, err := r.Process(context.Background(), host, req)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	// Existing target reported as "created" (preserves workspace
	// behavior: skipped-existing entities still appear in
	// EntitiesCreated so callers can link to them).
	if len(outcome.EntitiesCreated) != 1 || outcome.EntitiesCreated[0].ID != existing.ID {
		t.Errorf("expected EntitiesCreated=[%s], got %v", existing.ID, outcome.EntitiesCreated)
	}

	for _, c := range host.Calls {
		if strings.HasPrefix(c, "CreateEntityNoCascade:") {
			t.Errorf("unexpected CreateEntityNoCascade call: %v", host.Calls)
		}
	}
}

// TestRunnerIfExistsError — existing target found, IfExistsError:
// error recorded, no create.
func TestRunnerIfExistsError(t *testing.T) {
	r := newRunner(t, nil, nil)
	existing := entity.New("EXISTING-1", "checklist")
	host := &stubHost{t: t, existingTarget: existing}

	trigger := entity.New("REQ-001", "requirement")
	req := autocascade.Request{
		Trigger: trigger,
		Result: &automation.Result{
			EntitiesToCreate: []automation.EntityToCreate{
				{
					Type:                "checklist",
					RelationFromTrigger: "has-checklist",
					IfExists:            automation.IfExistsError,
				},
			},
		},
	}
	outcome, err := r.Process(context.Background(), host, req)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(outcome.Errors) == 0 {
		t.Fatal("expected an error in outcome.Errors")
	}
	if !strings.Contains(outcome.Errors[0], "entity already exists") {
		t.Errorf("expected 'entity already exists' error, got %q", outcome.Errors[0])
	}
}

// TestRunnerEntityCreateError — CreateEntityNoCascade returns error:
// recorded, cascade continues.
func TestRunnerEntityCreateError(t *testing.T) {
	r := newRunner(t, nil, nil)
	host := &stubHost{t: t, createErr: errors.New("simulated create failure"), store: memstore.New()}

	trigger := entity.New("REQ-001", "requirement")
	req := autocascade.Request{
		Trigger: trigger,
		Result: &automation.Result{
			EntitiesToCreate: []automation.EntityToCreate{
				{Type: "checklist"},
			},
		},
	}
	outcome, err := r.Process(context.Background(), host, req)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(outcome.Errors) != 1 {
		t.Fatalf("expected exactly one error, got %v", outcome.Errors)
	}
	if !strings.Contains(outcome.Errors[0], "failed to create automation entity") {
		t.Errorf("expected 'failed to create automation entity' error, got %q", outcome.Errors[0])
	}
	if len(outcome.EntitiesCreated) != 0 {
		t.Errorf("expected no entities created, got %v", outcome.EntitiesCreated)
	}
}

// TestRunnerRelationCreateError — WriteRelation returns error: recorded,
// cascade continues.
func TestRunnerRelationCreateError(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  requirement:
    label: Requirement
    plural: requirements
    id_prefix: "REQ-"
    id_type: short
    properties:
      title:
        type: string
  decision:
    label: Decision
    plural: decisions
    id_prefix: "DEC-"
    id_type: short
    properties:
      title:
        type: string
relations:
  references:
    label: references
    from: [requirement]
    to: [decision]
`))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	mem := memstore.New()
	target := entity.New("DEC-001", "decision")
	if err := mem.CreateEntity(context.Background(), target); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	host := &stubHost{
		t:           t,
		meta:        meta,
		store:       mem,
		writeRelErr: errors.New("simulated write failure"),
	}

	r := newRunner(t, nil, nil)
	trigger := entity.New("REQ-001", "requirement")
	req := autocascade.Request{
		Trigger: trigger,
		Result: &automation.Result{
			RelationsToCreate: []*entity.Relation{
				entity.NewRelation("", "references", "DEC-001"),
			},
		},
	}
	outcome, _ := r.Process(context.Background(), host, req)
	if len(outcome.Errors) != 1 {
		t.Fatalf("expected exactly one error, got %v", outcome.Errors)
	}
	if !strings.Contains(outcome.Errors[0], "failed to create automation relation") {
		t.Errorf("expected 'failed to create automation relation' error, got %q", outcome.Errors[0])
	}
	if len(outcome.RelationsCreated) != 0 {
		t.Errorf("expected no relations created, got %v", outcome.RelationsCreated)
	}
}

// TestRunnerActionOrder pins per-iteration action order: Lua → relations → entities.
func TestRunnerActionOrder(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  requirement:
    label: Requirement
    plural: requirements
    id_prefix: "REQ-"
    id_type: short
    properties:
      title:
        type: string
  decision:
    label: Decision
    plural: decisions
    id_prefix: "DEC-"
    id_type: short
    properties:
      title:
        type: string
  checklist:
    label: Checklist
    plural: checklists
    id_prefix: "CL-"
    id_type: short
    properties:
      title:
        type: string
relations:
  references:
    label: references
    from: [requirement]
    to: [decision]
`))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	mem := memstore.New()
	if seedErr := mem.CreateEntity(context.Background(), entity.New("DEC-001", "decision")); seedErr != nil {
		t.Fatalf("seed target: %v", seedErr)
	}

	recordingScripts := &recordingExecutor{}
	r := newRunner(t, nil, recordingScripts)

	host := &stubHost{t: t, meta: meta, store: mem}

	trigger := entity.New("REQ-001", "requirement")
	req := autocascade.Request{
		Trigger: trigger,
		Result: &automation.Result{
			LuaToExecute: []automation.LuaToExecute{
				{Code: "print('hi')", AutomationName: "test-lua"},
			},
			RelationsToCreate: []*entity.Relation{
				entity.NewRelation("", "references", "DEC-001"),
			},
			EntitiesToCreate: []automation.EntityToCreate{
				{Type: "checklist"},
			},
		},
	}
	outcome, err := r.Process(context.Background(), host, req)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(outcome.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", outcome.Errors)
	}

	if recordingScripts.executeCodeCalls != 1 {
		t.Errorf("expected 1 ExecuteCode call, got %d", recordingScripts.executeCodeCalls)
	}
	if len(host.Calls) < 2 {
		t.Fatalf("expected at least 2 host calls (WriteRelation, CreateEntityNoCascade), got %v", host.Calls)
	}
	if !strings.HasPrefix(host.Calls[0], "WriteRelation:") {
		t.Errorf("expected WriteRelation first among host calls, got %v", host.Calls)
	}
	if !strings.HasPrefix(host.Calls[1], "CreateEntityNoCascade:") {
		t.Errorf("expected CreateEntityNoCascade second among host calls, got %v", host.Calls)
	}
}

// TestRunnerLuaErrorPath pins lua.ScriptError.Path patching for inline
// Lua code: should be overwritten with "automation:<name>".
func TestRunnerLuaErrorPath(t *testing.T) {
	scripts := &failingExecutor{
		err: &lua.ScriptError{
			Surface:    lua.SurfaceAutomation,
			Path:       "", // empty: simulates inline `lua: |` block
			LuaMessage: "boom",
		},
	}
	r := newRunner(t, nil, scripts)

	trigger := entity.New("REQ-001", "requirement")
	req := autocascade.Request{
		Trigger: trigger,
		Result: &automation.Result{
			LuaToExecute: []automation.LuaToExecute{
				{Code: "error('boom')", AutomationName: "my-automation"},
			},
		},
	}
	outcome, err := r.Process(context.Background(), &stubHost{t: t}, req)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(outcome.Errors) != 1 {
		t.Fatalf("expected exactly one error, got %v", outcome.Errors)
	}
	if !strings.Contains(outcome.Errors[0], "automation:my-automation") {
		t.Errorf("expected error to contain 'automation:my-automation', got %q", outcome.Errors[0])
	}
}

// --- Script executor stubs ---

type recordingExecutor struct {
	executeCodeCalls int
	executeFileCalls int
}

func (r *recordingExecutor) ExecuteCode(_ string, _ lua.WriteDeps, _, _ *entity.Entity) error {
	r.executeCodeCalls++
	return nil
}

func (r *recordingExecutor) ExecuteFile(_ string, _ lua.WriteDeps, _, _ *entity.Entity) error {
	r.executeFileCalls++
	return nil
}

func (r *recordingExecutor) LuaCache() *lua.Cache { return nil }

type failingExecutor struct {
	err error
}

func (f *failingExecutor) ExecuteCode(_ string, _ lua.WriteDeps, _, _ *entity.Entity) error {
	return f.err
}

func (f *failingExecutor) ExecuteFile(_ string, _ lua.WriteDeps, _, _ *entity.Entity) error {
	return f.err
}

func (f *failingExecutor) LuaCache() *lua.Cache { return nil }
