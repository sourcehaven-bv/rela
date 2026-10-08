package entitymanager_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// hiddenWritePolicy: blind may read and write requirements and components but
// cannot read decisions, so a decision is nonexistent to blind. seer reads
// everything and writes the same types.
const hiddenWritePolicy = `
roles:
  blind:
    read: [requirement, component]
    create: [requirement, component]
    update: [requirement, component]
    delete: [requirement, component]
    rename: [requirement, component]
  seer:
    read: ["*"]
    create: [requirement, component]
    update: [requirement, component]
    delete: [requirement, component]
    rename: ["*"]
assignments:
  blind: blind
  seer: seer
`

func hiddenWriteManager(t *testing.T, st store.Store) *entitymanager.Manager {
	t.Helper()
	p, err := acl.LoadPolicyBytes([]byte(hiddenWritePolicy))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	d, err := acl.NewDeclarative(p, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:          st,
		Meta:           parseMeta(t),
		Templater:      nopTemplater{},
		Audit:          audit.Nop{},
		ACL:            d,
		Transitions:    statemachine.EmptySet(),
		FieldGate:      entitymanager.AllowAllFieldGate{},
		CopyReadGate:   entitymanager.AllowAllCopyReadGate{},
		CopyVisibility: allowAllCopyVisibility(t, st),
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr
}

func seedHidden(t *testing.T, st store.Store, entities []*entity.Entity, rels []entity.RelationKey) {
	t.Helper()
	ctx := context.Background()
	for _, e := range entities {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	for _, k := range rels {
		if _, err := st.CreateRelation(ctx, k, nil); err != nil {
			t.Fatalf("seed %s --%s--> %s: %v", k.From, k.Type, k.To, err)
		}
	}
}

// assertNoTrace fails when err, or the decision a 403 body is built from,
// mentions any of secrets.
func assertNoTrace(t *testing.T, err error, secrets ...string) {
	t.Helper()
	texts := []string{err.Error()}
	var forbidden *acl.ForbiddenError
	if errors.As(err, &forbidden) {
		d := forbidden.Decision
		texts = append(texts, d.Reason, d.RuleKind, d.RuleID)
	}
	for _, text := range texts {
		for _, s := range secrets {
			if strings.Contains(text, s) {
				t.Errorf("LEAK: %q mentions %q", text, s)
			}
		}
	}
}

// A cascade delete that is refused because of an edge to an entity the caller
// cannot read reveals only that it was refused. It names neither the hidden
// neighbor nor the relation type nor the neighbor's type (BUG-1BXQDD).
func TestCascadeDelete_DenialOverHiddenEdgeNamesNothingHidden(t *testing.T) {
	t.Parallel()
	st := memstore.New()
	seedHidden(t, st,
		[]*entity.Entity{entity.New("REQ-1", "requirement"), entity.New("DEC-SECRET", "decision")},
		[]entity.RelationKey{{From: "DEC-SECRET", Type: "addresses", To: "REQ-1"}})
	mgr := hiddenWriteManager(t, st)

	_, err := mgr.DeleteEntity(asUser("blind"), "REQ-1", true)
	if err == nil {
		t.Fatal("cascade succeeded; blind holds no delete on the addresses edge from a decision")
	}
	if !errors.Is(err, acl.ErrForbidden) {
		t.Errorf("error %v is not a forbidden error, so callers cannot map it to a 403", err)
	}
	assertNoTrace(t, err, "DEC-SECRET", "decision", "addresses")

	// A caller who can read the neighbor gets the actionable message.
	_, err = mgr.DeleteEntity(asUser("seer"), "REQ-1", true)
	if err == nil || !strings.Contains(err.Error(), "DEC-SECRET") {
		t.Errorf("seer's denial = %v, want it to name DEC-SECRET", err)
	}
}

// Rename is offered only for types whose ids are typed by hand. A generated id
// (short or sequential) carries no meaning a rename could improve. The
// refusal does not cover the target id: see
// TestRename_OntoHiddenIDRevealsOnlyTheCollision.
func TestRename_RefusedForGeneratedIDs(t *testing.T) {
	t.Parallel()
	st := memstore.New()
	seedHidden(t, st, []*entity.Entity{entity.New("DEC-1", "decision")}, nil)
	mgr := hiddenWriteManager(t, st)

	for _, dryRun := range []bool{false, true} {
		_, err := mgr.RenameEntity(asUser("seer"), "DEC-1", "DEC-2", entity.RenameOptions{DryRun: dryRun})
		if !errors.Is(err, entitymanager.ErrRenameNotSupported) {
			t.Errorf("rename (dry run %v) of a sequential id = %v, want ErrRenameNotSupported", dryRun, err)
		}
	}
	if _, err := st.GetEntity(context.Background(), entity.Ref{ID: "DEC-1"}); err != nil {
		t.Errorf("DEC-1 after a refused rename: %v", err)
	}
}

// A hidden entity stays nonexistent to a refused rename: the caller learns
// the id is gone, not that its type has generated ids.
func TestRename_HiddenGeneratedIDReadsAsMissing(t *testing.T) {
	t.Parallel()
	st := memstore.New()
	seedHidden(t, st, []*entity.Entity{entity.New("DEC-SECRET", "decision")}, nil)
	mgr := hiddenWriteManager(t, st)

	_, err := mgr.RenameEntity(asUser("blind"), "DEC-SECRET", "DEC-2", entity.RenameOptions{})
	if !errors.Is(err, entitymanager.ErrEntityNotFound) {
		t.Errorf("rename of a hidden entity = %v, want ErrEntityNotFound", err)
	}
}

// RelationsUpdated counts only edges whose other end the caller can read, in
// the real rename and in the dry run alike (BUG-1BXQDD).
func TestRename_RelationsUpdatedCountsReadableNeighborsOnly(t *testing.T) {
	t.Parallel()
	for _, dryRun := range []bool{true, false} {
		st := memstore.New()
		seedHidden(t, st,
			[]*entity.Entity{
				entity.New("api", "component"),
				entity.New("db", "component"),
				entity.New("DEC-SECRET", "decision"),
			},
			[]entity.RelationKey{
				{From: "api", Type: "depends-on", To: "db"},
				{From: "api", Type: "depends-on", To: "DEC-SECRET"},
			})
		mgr := hiddenWriteManager(t, st)

		res, err := mgr.RenameEntity(asUser("blind"), "api", "gateway", entity.RenameOptions{DryRun: dryRun})
		if err != nil {
			t.Fatalf("rename (dry run %v): %v", dryRun, err)
		}
		if res.RelationsUpdated != 1 {
			t.Errorf("blind's RelationsUpdated (dry run %v) = %d, want 1: the edge to DEC-SECRET is hidden",
				dryRun, res.RelationsUpdated)
		}
	}

	st := memstore.New()
	seedHidden(t, st,
		[]*entity.Entity{entity.New("api", "component"), entity.New("DEC-SECRET", "decision")},
		[]entity.RelationKey{{From: "api", Type: "depends-on", To: "DEC-SECRET"}})
	res, err := hiddenWriteManager(t, st).RenameEntity(asUser("seer"), "api", "gateway", entity.RenameOptions{})
	if err != nil {
		t.Fatalf("seer rename: %v", err)
	}
	if res.RelationsUpdated != 1 {
		t.Errorf("seer's RelationsUpdated = %d, want 1", res.RelationsUpdated)
	}
}

// Renaming onto an id a hidden entity holds is refused as a conflict. That
// one bit is accepted and documented in docs/acl-security.md; the refusal
// says nothing else about the hidden entity.
func TestRename_OntoHiddenIDRevealsOnlyTheCollision(t *testing.T) {
	t.Parallel()
	st := memstore.New()
	seedHidden(t, st, []*entity.Entity{entity.New("api", "component"), entity.New("DEC-SECRET", "decision")}, nil)
	mgr := hiddenWriteManager(t, st)

	_, err := mgr.RenameEntity(asUser("blind"), "api", "DEC-SECRET", entity.RenameOptions{})
	if !errors.Is(err, entitymanager.ErrEntityAlreadyExists) {
		t.Fatalf("rename onto a hidden id = %v, want ErrEntityAlreadyExists", err)
	}
	assertNoTrace(t, err, "decision")
}

// facedHiddenMetaYAML has a faced policy whose content-scoped edges belong to
// one face each. A caller who reads only the published face cannot see an
// edge tailed at the draft face, though the policy itself is readable.
const facedHiddenMetaYAML = `version: "1.0"
entities:
  policy:
    label: Policy
    plural: policies
    id_type: manual
    faces:
      draft: {label: Draft}
      published: {label: Published}
    properties:
      title: {type: string}
  component:
    label: Component
    id_type: manual
    properties:
      title: {type: string}
relations:
  governs:
    from: [policy]
    to: [component]
    scope: content
`

// facedHiddenPolicy: viewer reads the published face only and may not
// delete governs edges, which are authorized against the policy.
const facedHiddenPolicy = `
roles:
  viewer:
    read: ["policy@published", component]
    delete: [component]
    rename: [policy, component]
assignments:
  viewer: viewer
`

func facedHiddenManager(t *testing.T, edges []entity.RelationKey) *entitymanager.Manager {
	t.Helper()
	st := memstore.New()
	meta, err := metamodel.Parse([]byte(facedHiddenMetaYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	p, err := acl.LoadPolicyBytes([]byte(facedHiddenPolicy))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	d, err := acl.NewDeclarative(p, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	var rows []*entity.Entity
	for _, id := range []string{"POL-A", "POL-B"} {
		for _, face := range []entity.Face{"draft", "published"} {
			rows = append(rows, &entity.Entity{ID: id, Type: "policy", Face: face})
		}
	}
	rows = append(rows, entity.New("c1", "component"), entity.New("c2", "component"))
	seedHidden(t, st, rows, edges)
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL: d, Transitions: statemachine.EmptySet(),
		FieldGate:      entitymanager.AllowAllFieldGate{},
		CopyReadGate:   entitymanager.AllowAllCopyReadGate{},
		CopyVisibility: allowAllCopyVisibility(t, st),
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr
}

// An edge is hidden when its tail face is, even though another face of the
// same entity is readable: the read path's rule (RR-2IK76Z), not "some face
// of the far entity is readable".
func TestCascadeDelete_EdgeFromHiddenFaceNamesNothing(t *testing.T) {
	t.Parallel()
	mgr := facedHiddenManager(t, []entity.RelationKey{
		{From: "POL-A", FromFace: "draft", Type: "governs", To: "c1"},
	})
	_, err := mgr.DeleteEntity(asUser("viewer"), "c1", true)
	if !errors.Is(err, acl.ErrForbidden) {
		t.Fatalf("cascade over a hidden-face edge = %v, want a forbidden error", err)
	}
	assertNoTrace(t, err, "POL-A", "draft", "governs", "policy")

	mgr = facedHiddenManager(t, []entity.RelationKey{
		{From: "POL-A", FromFace: "published", Type: "governs", To: "c1"},
	})
	_, err = mgr.DeleteEntity(asUser("viewer"), "c1", true)
	if err == nil || !strings.Contains(err.Error(), "POL-A@published") {
		t.Errorf("denial over a visible edge = %v, want it to name POL-A@published", err)
	}
}

// When a hidden and a visible edge are both denied, the caller gets the
// visible edge's denial whatever the store order. Answering with the first
// denial would tell them a hidden edge sorted before the one they can see.
func TestCascadeDelete_VisibleDenialWinsOverHidden(t *testing.T) {
	t.Parallel()
	for _, hidden := range []string{"POL-A", "POL-B"} {
		visible := "POL-A"
		if hidden == visible {
			visible = "POL-B"
		}
		mgr := facedHiddenManager(t, []entity.RelationKey{
			{From: hidden, FromFace: "draft", Type: "governs", To: "c1"},
			{From: visible, FromFace: "published", Type: "governs", To: "c1"},
		})
		_, err := mgr.DeleteEntity(asUser("viewer"), "c1", true)
		if err == nil || !strings.Contains(err.Error(), visible+"@published") {
			t.Errorf("hidden edge from %s: denial = %v, want it to name %s@published", hidden, err, visible)
		}
		if err != nil {
			assertNoTrace(t, err, hidden)
		}
	}
}

// A rename moves the edges of every face, but reports only the edges the
// caller can see: an edge tailed at the entity's own hidden face is not
// counted.
func TestRename_RelationsUpdatedSkipsEdgesOfHiddenFaces(t *testing.T) {
	t.Parallel()
	for _, dryRun := range []bool{true, false} {
		mgr := facedHiddenManager(t, []entity.RelationKey{
			{From: "POL-A", FromFace: "draft", Type: "governs", To: "c1"},
			{From: "POL-A", FromFace: "published", Type: "governs", To: "c2"},
		})
		res, err := mgr.RenameEntity(asUser("viewer"), "POL-A", "POL-Z", entity.RenameOptions{DryRun: dryRun})
		if err != nil {
			t.Fatalf("rename (dry run %v): %v", dryRun, err)
		}
		if res.RelationsUpdated != 1 {
			t.Errorf("RelationsUpdated (dry run %v) = %d, want 1: the draft edge is hidden",
				dryRun, res.RelationsUpdated)
		}
	}
}
