package entitymanager_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// newManagerWithAudit builds a Manager wired with the supplied Audit
// backend (typically [audit.NewMemory] for assertion). Automations
// are optional — pass nil to disable.
func newManagerWithAudit(
	t *testing.T, sink audit.Audit, automations []automation.Automation,
) *entitymanager.Manager {
	t.Helper()
	deps := entitymanager.Deps{
		Store:     memstore.New(),
		Meta:      parseMeta(t),
		Templater: nopTemplater{},
		Audit:     sink,
		ACL:       acl.NopACL{},
	}
	if automations != nil {
		engine := automation.NewEngine(automations)
		runner, err := autocascade.New(autocascade.Deps{Engine: engine})
		if err != nil {
			t.Fatalf("autocascade.New: %v", err)
		}
		deps.Automations = engine
		deps.Cascade = runner
	}
	mgr, err := entitymanager.New(deps)
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr
}

// ctxWithPrincipal is a helper for tests that need to verify audit
// records carry the right Principal.
func ctxWithPrincipal(user, tool string) context.Context {
	return principal.With(context.Background(), principal.Principal{User: user, Tool: tool})
}

// --- AC1: every entity write produces one audit record ---

func TestAudit_AC1_EntityCreateRecordsOnce(t *testing.T) {
	t.Parallel()
	mem := audit.NewMemory()
	mgr := newManagerWithAudit(t, mem, nil)

	e := entity.New("", "requirement")
	e.SetString("title", "AC1 entity")
	res, err := mgr.CreateEntity(ctxWithPrincipal("alice", principal.ToolCLI), e, entity.CreateOptions{})
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}

	records := mem.Records()
	if len(records) != 1 {
		t.Fatalf("want 1 record, got %d", len(records))
	}
	r := records[0]
	if r.Op != audit.OpCreateEntity {
		t.Errorf("Op = %q, want %q", r.Op, audit.OpCreateEntity)
	}
	if r.Subject.Kind != "entity" {
		t.Errorf("Subject.Kind = %q, want entity", r.Subject.Kind)
	}
	if r.Subject.Type != "requirement" {
		t.Errorf("Subject.Type = %q, want requirement", r.Subject.Type)
	}
	if r.Subject.ID != res.Entity.ID {
		t.Errorf("Subject.ID = %q, want %q", r.Subject.ID, res.Entity.ID)
	}
	if r.Principal.User != "alice" || r.Principal.Tool != principal.ToolCLI {
		t.Errorf("Principal = %+v, want alice/cli", r.Principal)
	}
	if r.Summary != "created" {
		t.Errorf("Summary = %q, want 'created'", r.Summary)
	}
	if r.Time.IsZero() {
		t.Error("Time should be stamped")
	}
}

func TestAudit_AC1_EntityUpdateRecordsChangedPropertyNames(t *testing.T) {
	t.Parallel()
	mem := audit.NewMemory()
	mgr := newManagerWithAudit(t, mem, nil)

	e := entity.New("", "requirement")
	e.SetString("title", "Initial")
	res, err := mgr.CreateEntity(context.Background(), e, entity.CreateOptions{})
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}

	// Update changes title and flips status to a non-default value.
	// (The metamodel default for status is "draft"; setting "draft"
	// again wouldn't show up as a diff.)
	updated := res.Entity.Clone()
	updated.SetString("title", "Modified")
	updated.SetString("status", "accepted")
	if _, err := mgr.UpdateEntity(context.Background(), updated); err != nil {
		t.Fatalf("UpdateEntity: %v", err)
	}

	records := mem.Records()
	if len(records) != 2 {
		t.Fatalf("want 2 records (create + update), got %d", len(records))
	}
	updateRec := records[1]
	if updateRec.Op != audit.OpUpdateEntity {
		t.Errorf("Op = %q, want update-entity", updateRec.Op)
	}
	if !strings.HasPrefix(updateRec.Summary, "updated: ") {
		t.Errorf("Summary = %q, want prefix 'updated: '", updateRec.Summary)
	}
	// Both keys must appear; order is deterministic (sorted).
	if !strings.Contains(updateRec.Summary, "status") {
		t.Errorf("Summary missing 'status': %q", updateRec.Summary)
	}
	if !strings.Contains(updateRec.Summary, "title") {
		t.Errorf("Summary missing 'title': %q", updateRec.Summary)
	}
	// Values must NOT appear (secret-leak defense).
	if strings.Contains(updateRec.Summary, "Modified") || strings.Contains(updateRec.Summary, "accepted") {
		t.Errorf("Summary leaks property values: %q", updateRec.Summary)
	}
}

func TestAudit_AC1_EntityDeleteRecords(t *testing.T) {
	t.Parallel()
	mem := audit.NewMemory()
	mgr := newManagerWithAudit(t, mem, nil)

	res, err := mgr.CreateEntity(context.Background(),
		entity.New("", "requirement"), entity.CreateOptions{})
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}

	if _, err := mgr.DeleteEntity(context.Background(), res.Entity.ID, false); err != nil {
		t.Fatalf("DeleteEntity: %v", err)
	}

	records := mem.Records()
	if len(records) != 2 {
		t.Fatalf("want 2 records (create + delete), got %d", len(records))
	}
	delRec := records[1]
	if delRec.Op != audit.OpDeleteEntity {
		t.Errorf("Op = %q, want delete-entity", delRec.Op)
	}
	if delRec.Subject.ID != res.Entity.ID {
		t.Errorf("Subject.ID = %q, want %q", delRec.Subject.ID, res.Entity.ID)
	}
	if delRec.Summary != "deleted" {
		t.Errorf("Summary = %q, want 'deleted'", delRec.Summary)
	}
}

func TestAudit_AC1_EntityRenameRecordsBeforeAfter(t *testing.T) {
	t.Parallel()
	mem := audit.NewMemory()
	mgr := newManagerWithAudit(t, mem, nil)

	res, err := mgr.CreateEntity(context.Background(),
		entity.New("REQ-OLD", "requirement"), entity.CreateOptions{ID: "REQ-OLD"})
	if err != nil {
		// Sequential IDs reject custom — use whatever ID was assigned.
		res, err = mgr.CreateEntity(context.Background(),
			entity.New("", "requirement"), entity.CreateOptions{})
		if err != nil {
			t.Fatalf("CreateEntity: %v", err)
		}
	}
	oldID := res.Entity.ID

	// Use a custom-ID-capable type (decision) for rename; both use
	// sequential IDs but rename takes the operator-supplied new ID.
	_, err = mgr.RenameEntity(context.Background(), oldID, oldID+"-renamed", entity.RenameOptions{})
	if err != nil {
		t.Fatalf("RenameEntity: %v", err)
	}

	records := mem.Records()
	// We expect: 1 create + rename records.
	// (Rename may add records for incident-relation rewrites; this entity has none.)
	var renameRec *audit.Record
	for i := range records {
		if records[i].Op == audit.OpRenameEntity {
			renameRec = &records[i]
		}
	}
	if renameRec == nil {
		t.Fatalf("expected rename-entity record; got: %+v", records)
	}
	if renameRec.Before.ID != oldID {
		t.Errorf("Before.ID = %q, want %q", renameRec.Before.ID, oldID)
	}
	if renameRec.After.ID != oldID+"-renamed" {
		t.Errorf("After.ID = %q, want %q-renamed", renameRec.After.ID, oldID)
	}
	if renameRec.Before.Type != "requirement" || renameRec.After.Type != "requirement" {
		t.Errorf("expected type=requirement in Before/After, got %q/%q",
			renameRec.Before.Type, renameRec.After.Type)
	}
	// Subject must be nil for rename (Before/After carry the diff).
	if renameRec.Subject != nil {
		t.Errorf("rename should leave Subject nil, got %+v", *renameRec.Subject)
	}
	if renameRec.Before == nil || renameRec.After == nil {
		t.Fatalf("rename must populate both Before and After: before=%v after=%v",
			renameRec.Before, renameRec.After)
	}

	// Pin the JSON wire contract too: encoding/json must omit subject
	// (Subject is *Subject specifically so omitempty fires) and emit
	// before / after. A regression that changed Subject back to
	// non-pointer would still pass the nil-check above (Subject would
	// be a zero struct, not nil) but fail this JSON assertion.
	data, err := json.Marshal(*renameRec)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(data), `"subject"`) {
		t.Errorf("rename JSON must not include subject, got: %s", data)
	}
	if !strings.Contains(string(data), `"before"`) {
		t.Errorf("rename JSON must include before, got: %s", data)
	}
	if !strings.Contains(string(data), `"after"`) {
		t.Errorf("rename JSON must include after, got: %s", data)
	}
}

// --- AC2: every relation write produces one audit record ---

func TestAudit_AC2_RelationCreateRecordsWithRelationSubject(t *testing.T) {
	t.Parallel()
	mem := audit.NewMemory()
	mgr := newManagerWithAudit(t, mem, nil)

	req, err := mgr.CreateEntity(context.Background(),
		entity.New("", "requirement"), entity.CreateOptions{})
	if err != nil {
		t.Fatalf("CreateEntity req: %v", err)
	}
	dec, err := mgr.CreateEntity(context.Background(),
		entity.New("", "decision"), entity.CreateOptions{})
	if err != nil {
		t.Fatalf("CreateEntity dec: %v", err)
	}

	startLen := len(mem.Records())

	rel, err := mgr.CreateRelation(context.Background(),
		dec.Entity.ID, "addresses", req.Entity.ID, entity.RelationOptions{})
	if err != nil {
		t.Fatalf("CreateRelation: %v", err)
	}

	records := mem.Records()
	if len(records) != startLen+1 {
		t.Fatalf("want 1 new record, got %d new (total=%d)", len(records)-startLen, len(records))
	}
	r := records[startLen]
	if r.Op != audit.OpCreateRelation {
		t.Errorf("Op = %q, want create-relation", r.Op)
	}
	if r.Subject.Kind != "relation" {
		t.Errorf("Subject.Kind = %q, want relation", r.Subject.Kind)
	}
	if r.Subject.RelationType != "addresses" {
		t.Errorf("Subject.RelationType = %q, want addresses", r.Subject.RelationType)
	}
	if r.Subject.FromID != rel.From || r.Subject.ToID != rel.To {
		t.Errorf("Subject endpoints = %s -> %s, want %s -> %s",
			r.Subject.FromID, r.Subject.ToID, rel.From, rel.To)
	}
}

func TestAudit_AC2_RelationDeleteRecords(t *testing.T) {
	t.Parallel()
	mem := audit.NewMemory()
	mgr := newManagerWithAudit(t, mem, nil)

	req, _ := mgr.CreateEntity(context.Background(),
		entity.New("", "requirement"), entity.CreateOptions{})
	dec, _ := mgr.CreateEntity(context.Background(),
		entity.New("", "decision"), entity.CreateOptions{})
	_, _ = mgr.CreateRelation(context.Background(),
		dec.Entity.ID, "addresses", req.Entity.ID, entity.RelationOptions{})

	startLen := len(mem.Records())
	if err := mgr.DeleteRelation(context.Background(),
		dec.Entity.ID, "addresses", req.Entity.ID); err != nil {
		t.Fatalf("DeleteRelation: %v", err)
	}

	records := mem.Records()
	if len(records) != startLen+1 {
		t.Fatalf("want 1 new record, got %d", len(records)-startLen)
	}
	r := records[startLen]
	if r.Op != audit.OpDeleteRelation {
		t.Errorf("Op = %q, want delete-relation", r.Op)
	}
	if r.Subject.FromID != dec.Entity.ID || r.Subject.ToID != req.Entity.ID {
		t.Errorf("Subject endpoints wrong: %+v", r.Subject)
	}
}

// --- AC3: Principal flows from ctx into the record ---

func TestAudit_AC3_PrincipalFromCtx(t *testing.T) {
	t.Parallel()
	mem := audit.NewMemory()
	mgr := newManagerWithAudit(t, mem, nil)

	ctx := ctxWithPrincipal("alice", principal.ToolMCP)
	_, err := mgr.CreateEntity(ctx, entity.New("", "requirement"), entity.CreateOptions{})
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}

	r := mem.Records()[0]
	if r.Principal.User != "alice" {
		t.Errorf("Principal.User = %q, want alice", r.Principal.User)
	}
	if r.Principal.Tool != principal.ToolMCP {
		t.Errorf("Principal.Tool = %q, want mcp", r.Principal.Tool)
	}
}

func TestAudit_AC3_PrincipalDefaultsUnknownWhenAbsent(t *testing.T) {
	t.Parallel()
	mem := audit.NewMemory()
	mgr := newManagerWithAudit(t, mem, nil)

	// ctx without WithPrincipal — should default to unknown/unknown.
	_, err := mgr.CreateEntity(context.Background(), entity.New("", "requirement"), entity.CreateOptions{})
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}

	r := mem.Records()[0]
	if r.Principal.User != "unknown" || r.Principal.Tool != "unknown" {
		t.Errorf("Principal = %+v, want unknown/unknown", r.Principal)
	}
}

// --- AC7: delete-cascade produces 1+N records ---

func TestAudit_AC7_DeleteCascadeProduces1PlusNRecords(t *testing.T) {
	t.Parallel()
	mem := audit.NewMemory()
	mgr := newManagerWithAudit(t, mem, nil)

	req, _ := mgr.CreateEntity(context.Background(),
		entity.New("", "requirement"), entity.CreateOptions{})
	dec1, _ := mgr.CreateEntity(context.Background(),
		entity.New("", "decision"), entity.CreateOptions{})
	dec2, _ := mgr.CreateEntity(context.Background(),
		entity.New("", "decision"), entity.CreateOptions{})

	_, _ = mgr.CreateRelation(context.Background(),
		dec1.Entity.ID, "addresses", req.Entity.ID, entity.RelationOptions{})
	_, _ = mgr.CreateRelation(context.Background(),
		dec2.Entity.ID, "addresses", req.Entity.ID, entity.RelationOptions{})

	startLen := len(mem.Records())

	if _, err := mgr.DeleteEntity(context.Background(), req.Entity.ID, true); err != nil {
		t.Fatalf("DeleteEntity cascade: %v", err)
	}

	// Expect 2 relation-delete records + 1 entity-delete = 3 new records.
	records := mem.Records()
	newRecords := records[startLen:]
	if len(newRecords) != 3 {
		t.Fatalf("want 3 new records (2 rel + 1 entity), got %d: %+v", len(newRecords), newRecords)
	}

	entityDeletes := 0
	relationDeletes := 0
	for _, r := range newRecords {
		switch r.Op {
		case audit.OpDeleteEntity:
			entityDeletes++
			if !strings.Contains(r.Summary, "cascade") {
				t.Errorf("entity-delete summary should mention cascade, got %q", r.Summary)
			}
		case audit.OpDeleteRelation:
			relationDeletes++
			expected := "cascade:delete-entity:" + req.Entity.ID
			if r.TriggeredBy != expected {
				t.Errorf("relation-delete TriggeredBy = %q, want %q", r.TriggeredBy, expected)
			}
		}
	}
	if entityDeletes != 1 {
		t.Errorf("want 1 entity-delete, got %d", entityDeletes)
	}
	if relationDeletes != 2 {
		t.Errorf("want 2 relation-deletes, got %d", relationDeletes)
	}
}

// --- AC5: triggered_by populated for automation-driven writes ---

func TestAudit_AC5_TriggeredByOnAutomationCascade(t *testing.T) {
	t.Parallel()
	mem := audit.NewMemory()

	// Automation: when a requirement is created, auto-create a checklist
	// related via has-checklist.
	autos := []automation.Automation{{
		Name: "create-checklist-for-req",
		On: automation.Trigger{
			Entity:  []string{"requirement"},
			Created: true,
		},
		Do: []automation.Action{{
			CreateEntity: &automation.CreateEntityAction{
				Type:     "checklist",
				Relation: "has-checklist",
			},
		}},
	}}
	mgr := newManagerWithAudit(t, mem, autos)

	ctx := ctxWithPrincipal("alice", principal.ToolCLI)
	_, err := mgr.CreateEntity(ctx, entity.New("", "requirement"), entity.CreateOptions{})
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}

	records := mem.Records()
	// Expect:
	//   - 1 create-entity record for the requirement (no triggered_by).
	//   - 1 create-entity record for the cascaded checklist (triggered_by=automation).
	//   - 1 create-relation record for has-checklist (triggered_by=automation).
	var direct, cascadedEntity, cascadedRelation int
	for _, r := range records {
		switch {
		case r.Op == audit.OpCreateEntity && r.TriggeredBy == "":
			direct++
		case r.Op == audit.OpCreateEntity && r.TriggeredBy != "":
			cascadedEntity++
		case r.Op == audit.OpCreateRelation && r.TriggeredBy != "":
			cascadedRelation++
		}
		// All records must inherit the user's Principal.
		if r.Principal.User != "alice" {
			t.Errorf("expected Principal.User=alice on every record, got %q on %s", r.Principal.User, r.Op)
		}
	}

	if direct != 1 {
		t.Errorf("want 1 direct create-entity, got %d", direct)
	}
	if cascadedEntity == 0 {
		t.Errorf("want >=1 cascaded create-entity records with TriggeredBy, got 0")
	}
	if cascadedRelation == 0 {
		t.Errorf("want >=1 cascaded create-relation records with TriggeredBy, got 0")
	}
}

// TestAudit_IfExistsReplaceUsesCascadeLabel verifies that the
// IfExistsReplace path through autocascade.Runner →
// cascadeHost.DeleteEntity labels the cascaded relation-deletes with
// `cascade:delete-entity:<id>` — symmetric with the direct
// Manager.DeleteEntity path. Without this, replace operations would
// be indistinguishable from automation-generated relation deletes in
// the audit trail.
//
// The test drives the full production path (Manager.UpdateEntity →
// engine → Runner → host) so a ctx-threading regression in any of
// the intermediate hops fails this test, not just a direct
// cascadeHost call.
func TestAudit_IfExistsReplaceUsesCascadeLabel(t *testing.T) {
	t.Parallel()
	mem := audit.NewMemory()

	// Automation: when requirement.status becomes "active", create a
	// new checklist via has-checklist with if_exists: replace. The
	// trigger is a status change so we can seed an existing checklist
	// first, then drive the replace.
	autos := []automation.Automation{{
		Name: "replace-checklist-on-active",
		On: automation.Trigger{
			Entity:   []string{"requirement"},
			Property: "status",
			Becomes:  "accepted",
		},
		Do: []automation.Action{{
			CreateEntity: &automation.CreateEntityAction{
				Type:     "checklist",
				Relation: "has-checklist",
				IfExists: automation.IfExistsReplace,
			},
		}},
	}}
	mgr := newManagerWithAudit(t, mem, autos)

	// Seed: create the requirement and an existing checklist with the
	// has-checklist relation in place. The automation's replace path
	// will delete the existing checklist, which cascades to the
	// has-checklist relation.
	req, err := mgr.CreateEntity(context.Background(),
		entity.New("", "requirement"), entity.CreateOptions{})
	if err != nil {
		t.Fatalf("CreateEntity req: %v", err)
	}
	cl, err := mgr.CreateEntity(context.Background(),
		entity.New("", "checklist"), entity.CreateOptions{})
	if err != nil {
		t.Fatalf("CreateEntity checklist: %v", err)
	}
	if _, err := mgr.CreateRelation(context.Background(),
		req.Entity.ID, "has-checklist", cl.Entity.ID, entity.RelationOptions{}); err != nil {
		t.Fatalf("CreateRelation: %v", err)
	}

	startLen := len(mem.Records())

	// Drive the replace via UpdateEntity → automation → cascade.
	updated := req.Entity.Clone()
	updated.SetString("status", "accepted")
	if _, err := mgr.UpdateEntity(context.Background(), updated); err != nil {
		t.Fatalf("UpdateEntity: %v", err)
	}

	newRecords := mem.Records()[startLen:]
	wantTrigger := "cascade:delete-entity:" + cl.Entity.ID

	var relDeletes int
	for _, r := range newRecords {
		if r.Op == audit.OpDeleteRelation {
			relDeletes++
			if r.TriggeredBy != wantTrigger {
				t.Errorf("relation-delete TriggeredBy = %q, want %q", r.TriggeredBy, wantTrigger)
			}
		}
	}
	if relDeletes == 0 {
		t.Fatalf("expected at least one relation-delete record from cascade; got records: %+v", newRecords)
	}
}

// TestAudit_CascadeWriteEntityIsSilent pins that
// autocascade.Host.WriteEntity is *not* audited from cascadeHost.
// The Runner uses WriteEntity to persist post-cascade property
// changes onto an entity that was already audited at CreateEntity
// time; emitting again would double-count the same entity creation.
// This test sets up an automation chain where a cascade-created
// entity itself triggers another automation that sets a property —
// which causes the Runner to call WriteEntity — and asserts no
// duplicate entity-create record appears.
func TestAudit_CascadeWriteEntityIsSilent(t *testing.T) {
	t.Parallel()
	mem := audit.NewMemory()

	autos := []automation.Automation{
		{
			Name: "create-checklist-for-req",
			On: automation.Trigger{
				Entity:  []string{"requirement"},
				Created: true,
			},
			Do: []automation.Action{{
				CreateEntity: &automation.CreateEntityAction{
					Type:     "checklist",
					Relation: "has-checklist",
				},
			}},
		},
		{
			// When the cascade-created checklist fires its own
			// created-event, set status — this is what makes the
			// Runner call host.WriteEntity to persist the property.
			Name: "stamp-status-on-checklist",
			On: automation.Trigger{
				Entity:  []string{"checklist"},
				Created: true,
			},
			Do: []automation.Action{{
				Set:   "status",
				Value: "draft",
			}},
		},
	}
	mgr := newManagerWithAudit(t, mem, autos)

	_, err := mgr.CreateEntity(context.Background(),
		entity.New("", "requirement"), entity.CreateOptions{})
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}

	records := mem.Records()

	// Expect exactly ONE create-entity record for the checklist
	// (not two: one for the create, one for the property-set
	// WriteEntity). The Set is silent by design.
	var checklistCreates int
	for _, r := range records {
		if r.Op == audit.OpCreateEntity && r.Subject != nil && r.Subject.Type == "checklist" {
			checklistCreates++
		}
	}
	if checklistCreates != 1 {
		t.Errorf("want exactly 1 create-entity record for checklist (WriteEntity from "+
			"property-set cascade should be silent), got %d", checklistCreates)
	}
}

// --- AC11: Nop is safe ---

func TestAudit_AC11_NopRecordsNothing(t *testing.T) {
	t.Parallel()
	// Construct with Nop — no panics, no observable side effects.
	mgr := newManagerWithAudit(t, audit.Nop{}, nil)
	_, err := mgr.CreateEntity(context.Background(),
		entity.New("", "requirement"), entity.CreateOptions{})
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}
}

// --- AC12: nil Audit is rejected at construction (already covered by
// TestNew_RejectsNilAudit in manager_test.go) ---
