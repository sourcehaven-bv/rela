package entitymanager_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// Two types declare the basecamp system; one declares jira.
const xrefMetamodelYAML = `version: "1.0"
entities:
  ticket:
    label: Ticket
    id_type: manual
    properties:
      title:
        type: string
      basecamp:
        type: external_ref
        system: basecamp
      jira:
        type: external_ref
        system: jira
  todo:
    label: Todo
    id_type: manual
    properties:
      bc:
        type: external_ref
        system: basecamp
`

func newXrefManager(t *testing.T) *entitymanager.Manager {
	t.Helper()
	meta, err := metamodel.Parse([]byte(xrefMetamodelYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:       memstore.New(),
		Meta:        meta,
		Templater:   nopTemplater{},
		Audit:       audit.Nop{},
		ACL:         acl.NopACL{},
		Transitions: statemachine.EmptySet(),
		FieldGate:   entitymanager.AllowAllFieldGate{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr
}

func createRef(ctx context.Context, mgr *entitymanager.Manager, typ, id, prop, extID string) error {
	e := entity.New(id, typ)
	e.Properties[prop] = map[string]any{"id": extID}
	_, err := mgr.CreateEntity(ctx, e, entity.CreateOptions{ID: id, WriteExternalRefs: true})
	return err
}

// AC5: one (system, id) links at most one entity, across every type that
// declares the system; the 422 names the property and not the holder.
func TestExternalRefUnique(t *testing.T) {
	tests := []struct {
		name               string
		typ, prop, id, ext string
		wantErr            bool
	}{
		{name: "same type same id", typ: "ticket", prop: "basecamp", id: "T-2", ext: "42", wantErr: true},
		{name: "other type same system", typ: "todo", prop: "bc", id: "D-1", ext: "42", wantErr: true},
		{name: "same id other system", typ: "ticket", prop: "jira", id: "T-3", ext: "42"},
		{name: "other id", typ: "ticket", prop: "basecamp", id: "T-4", ext: "43"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mgr := newXrefManager(t)
			if err := createRef(context.Background(), mgr, "ticket", "T-1", "basecamp", "42"); err != nil {
				t.Fatalf("first create: %v", err)
			}
			err := createRef(context.Background(), mgr, tc.typ, tc.id, tc.prop, tc.ext)
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !isValidationError(err) {
				t.Fatalf("want *ValidationError, got %T: %v", err, err)
			}
			if strings.Contains(err.Error(), "T-1") || strings.Contains(err.Error(), "42") {
				t.Fatalf("error discloses the holder or the id: %v", err)
			}
		})
	}
}

func TestExternalRefUnique_SelfAndPatch(t *testing.T) {
	mgr := newXrefManager(t)
	ctx := context.Background()
	if err := createRef(context.Background(), mgr, "ticket", "T-1", "basecamp", "42"); err != nil {
		t.Fatal(err)
	}
	if err := createRef(context.Background(), mgr, "ticket", "T-2", "basecamp", "43"); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.PatchEntity(ctx, "T-1", entity.Patch{Properties: map[string]any{"title": "x"}}); err != nil {
		t.Fatalf("re-save of own ref: %v", err)
	}
	_, err := mgr.PatchEntity(ctx, "T-2", entity.Patch{
		Properties: map[string]any{"basecamp": map[string]any{"id": "42"}}, WriteExternalRefs: true,
	})
	if !isValidationError(err) {
		t.Fatalf("patch onto a taken id: want *ValidationError, got %v", err)
	}
}

// A soft-deleted holder keeps its id, so an undo is never refused.
func TestExternalRefUnique_SoftDeletedHolderReserves(t *testing.T) {
	mgr := newXrefManager(t)
	ctx := context.Background()
	if err := createRef(context.Background(), mgr, "todo", "D-1", "bc", "42"); err != nil {
		t.Fatal(err)
	}
	if !entitymanager.SupportsSoftDelete(mgr) {
		t.Fatal("memstore must support soft delete")
	}
	if _, err := entitymanager.SoftDeleteEntity(ctx, mgr, "D-1"); err != nil {
		t.Fatal(err)
	}
	// Across types and within the holder's own type.
	if err := createRef(ctx, mgr, "ticket", "T-1", "basecamp", "42"); !isValidationError(err) {
		t.Fatalf("other type: want *ValidationError, got %v", err)
	}
	if err := createRef(ctx, mgr, "todo", "D-2", "bc", "42"); !isValidationError(err) {
		t.Fatalf("same type: want *ValidationError, got %v", err)
	}
}

// AC4: the faces of one entity may share an id; two entities may not, at
// the same face.
func TestExternalRefUnique_Faces(t *testing.T) {
	const yaml = `
version: "1"
entities:
  page:
    label: Page
    id_type: manual
    faces:
      live: {}
      review: {}
    properties:
      jira: {type: external_ref, system: jira}
`
	meta, err := metamodel.Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: memstore.New(), Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{}, ACL: acl.NopACL{},
		Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	create := func(id string, face entity.Face) error {
		e := entity.New(id, "page")
		e.Properties["jira"] = map[string]any{"id": "J-1"}
		_, cerr := mgr.CreateEntity(ctx, e, entity.CreateOptions{ID: id, Face: face, WriteExternalRefs: true})
		return cerr
	}
	if err := create("PAGE-1", "live"); err != nil {
		t.Fatalf("first holder: %v", err)
	}
	if err := create("PAGE-1", "review"); err != nil {
		t.Fatalf("another face of the same entity: %v", err)
	}
	for _, face := range []entity.Face{"live", "review"} {
		if err := create("PAGE-2", face); !isValidationError(err) {
			t.Fatalf("another entity at %s: want *ValidationError, got %v", face, err)
		}
	}
}

// Finding 12: system writes that no field gate sees never write a ref, and
// a recreate refuses one unless its caller opts in.
func TestExternalRef_SystemWritesRefused(t *testing.T) {
	ctx := context.Background()

	t.Run("automation set", func(t *testing.T) {
		mgr := newXrefManager(t)
		res, err := mgr.CreateEntity(ctx, entity.New("T-1", "ticket"), entity.CreateOptions{ID: "T-1"})
		if err != nil {
			t.Fatal(err)
		}
		_, err = entitymanager.WriteAutomationProperties(ctx, mgr, res.Entity, map[string]string{"basecamp": "x"})
		if !isValidationError(err) || !strings.Contains(err.Error(), "external ref") {
			t.Fatalf("want the external-ref refusal, got %v", err)
		}
	})
	t.Run("cascade create", func(t *testing.T) {
		mgr := newXrefManager(t)
		_, err := entitymanager.CascadeHostCreate(ctx, mgr, "ticket", "T-1",
			map[string]any{"title": "x", "basecamp": map[string]any{"id": "42"}})
		if !isValidationError(err) || !strings.Contains(err.Error(), "external ref") {
			t.Fatalf("want the external-ref refusal, got %v", err)
		}
		if _, err := entitymanager.CascadeHostCreate(ctx, mgr, "ticket", "T-2", map[string]any{"title": "x"}); err != nil {
			t.Fatalf("a cascade create without a ref: %v", err)
		}
	})
	t.Run("recreate", func(t *testing.T) {
		mgr := newXrefManager(t)
		back := func() *entity.Entity {
			e := entity.New("T-1", "ticket")
			e.Properties["basecamp"] = map[string]any{"id": "42"}
			return e
		}
		if _, err := entitymanager.RecreateEntity(ctx, mgr, back()); !isValidationError(err) ||
			!strings.Contains(err.Error(), "external ref") {

			t.Fatalf("data-entry restore: want the external-ref refusal, got %v", err)
		}
		if _, err := (entitymanager.Recreator{M: mgr}).RecreateEntity(ctx, back()); !isValidationError(err) {
			t.Fatalf("recreator without opt-in: want *ValidationError, got %v", err)
		}
		if _, err := (entitymanager.Recreator{M: mgr, WriteExternalRefs: true}).RecreateEntity(ctx, back()); err != nil {
			t.Fatalf("operator restore: %v", err)
		}
	})
}

// D3: interactive surfaces cannot write a ref; a script write can, and a
// whole-entity save carrying the stored ref unchanged passes.
func TestExternalRef_InteractiveWritesRefused(t *testing.T) {
	ctx := context.Background()
	ref := map[string]any{"id": "42"}

	t.Run("create", func(t *testing.T) {
		mgr := newXrefManager(t)
		e := entity.New("T-1", "ticket")
		e.Properties["basecamp"] = ref
		_, err := mgr.CreateEntity(ctx, e, entity.CreateOptions{ID: "T-1"})
		if !isValidationError(err) {
			t.Fatalf("want *ValidationError, got %v", err)
		}
		if _, _, err := mgr.ValidateCreate(ctx, e, entity.CreateOptions{ID: "T-1"}); !isValidationError(err) {
			t.Fatalf("dry run: want *ValidationError, got %v", err)
		}
	})
	t.Run("patch set and unset", func(t *testing.T) {
		mgr := newXrefManager(t)
		if err := createRef(ctx, mgr, "ticket", "T-1", "basecamp", "42"); err != nil {
			t.Fatal(err)
		}
		_, err := mgr.PatchEntity(ctx, "T-1", entity.Patch{Properties: map[string]any{"basecamp": map[string]any{"id": "9"}}})
		if !isValidationError(err) {
			t.Fatalf("set: want *ValidationError, got %v", err)
		}
		_, err = mgr.PatchEntity(ctx, "T-1", entity.Patch{MetaUnset: []string{"basecamp"}})
		if !isValidationError(err) {
			t.Fatalf("unset: want *ValidationError, got %v", err)
		}
		if _, err := mgr.PatchEntity(ctx, "T-1", entity.Patch{Properties: map[string]any{"basecamp": ref}}); err != nil {
			t.Fatalf("an unchanged value is not a write: %v", err)
		}
	})
	t.Run("whole-entity save", func(t *testing.T) {
		mgr := newXrefManager(t)
		if err := createRef(ctx, mgr, "ticket", "T-1", "basecamp", "42"); err != nil {
			t.Fatal(err)
		}
		e := entity.New("T-1", "ticket")
		e.Properties["basecamp"] = ref
		e.Properties["title"] = "kept"
		if _, err := mgr.UpdateEntity(ctx, e); err != nil {
			t.Fatalf("save carrying the ref: %v", err)
		}
		delete(e.Properties, "basecamp")
		if _, err := mgr.UpdateEntity(ctx, e); !isValidationError(err) {
			t.Fatalf("save dropping the ref: want *ValidationError, got %v", err)
		}
	})
}

// D3: a history restore keeps the live ref.
func TestCarryFileValues_KeepsLiveExternalRef(t *testing.T) {
	meta, err := metamodel.Parse([]byte(xrefMetamodelYAML))
	if err != nil {
		t.Fatal(err)
	}
	live := entity.New("T-1", "ticket")
	live.Properties["basecamp"] = map[string]any{"id": "new"}
	snap := map[string]any{"title": "old", "basecamp": map[string]any{"id": "old"}, "jira": map[string]any{"id": "j"}}

	got := entitymanager.CarryFileValues(meta, "ticket", snap, live)
	if got["basecamp"].(map[string]any)["id"] != "new" {
		t.Fatalf("basecamp = %v, want the live value", got["basecamp"])
	}
	if _, ok := got["jira"]; ok {
		t.Fatalf("jira = %v, want it absent like the live row", got["jira"])
	}
	if got := entitymanager.CarryFileValues(meta, "ticket", snap, nil); got["basecamp"].(map[string]any)["id"] != "old" {
		t.Fatalf("re-create: basecamp = %v, want the snapshot value", got["basecamp"])
	}
}

// D10: a copy never writes an external ref. The review face keeps its own
// ref, and a face the copy creates carries none.
func TestCopy_KeepsTargetExternalRefs(t *testing.T) {
	const yaml = `
version: "1"
entities:
  page:
    label: Page
    id_prefix: PAGE
    faces:
      live: {}
      review: {}
    properties:
      title: {type: string}
      jira: {type: external_ref, system: jira}
copies:
  stage-review:
    from: page@live
    to: page@review
    fields: all
    guard:
      permission: stage
`
	meta, err := metamodel.Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{}, ACL: acl.NopACL{},
		Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
		CopyGuard: allowGuard{allow: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	seed := func(id string, face entity.Face, props map[string]any) {
		t.Helper()
		if cerr := st.CreateEntity(ctx, &entity.Entity{ID: id, Type: "page", Face: face, Properties: props}); cerr != nil {
			t.Fatal(cerr)
		}
	}
	seed("PAGE-1", "live", map[string]any{"title": "a", "jira": map[string]any{"id": "J-1"}})
	seed("PAGE-1", "review", map[string]any{"title": "old", "jira": map[string]any{"id": "J-9"}})
	seed("PAGE-2", "live", map[string]any{"title": "b", "jira": map[string]any{"id": "J-2"}})

	for _, id := range []string{"PAGE-1", "PAGE-2"} {
		if _, cerr := mgr.CopyState(ctx, entitymanager.CopyRequest{Definition: "stage-review", SourceID: id}); cerr != nil {
			t.Fatalf("copy %s: %v", id, cerr)
		}
	}
	got, err := st.GetEntity(ctx, entity.Ref{ID: "PAGE-1", Face: "review"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Properties["title"] != "a" || metamodel.FormatExternalRef(got.Properties["jira"]) != "J-9" {
		t.Fatalf("PAGE-1@review = %v, want the copied title and its own ref", got.Properties)
	}
	got, err = st.GetEntity(ctx, entity.Ref{ID: "PAGE-2", Face: "review"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Properties["jira"]; ok {
		t.Fatalf("PAGE-2@review = %v, want no ref", got.Properties)
	}
}
