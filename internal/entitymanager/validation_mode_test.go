package entitymanager_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// validationModeYAML has a required property, an enum, an automation that
// fills the required property on update, and a write-enforced rule.
const validationModeYAML = `
version: "1.0"
types:
  versiestatus:
    values: [concept, ter_beoordeling, goedgekeurd]
entities:
  verwerking:
    label: Verwerking
    id_prefix: "VW-"
    id_type: sequential
    properties:
      naam: {type: string, required: true}
      versiestatus: {type: versiestatus}
      rechtsgrond: {type: string}
      autofill: {type: string}
relations: {}
automations:
  - name: fill-naam
    on: {entity: verwerking, updated: true, when: ["autofill=yes"]}
    do:
      - set: naam
        value: "filled"
  - name: submit-on-create
    on: {entity: verwerking, created: true, when: ["autofill=submit"]}
    do:
      - set: versiestatus
        value: ter_beoordeling
validations:
  - name: rechtsgrond-bij-beoordeling
    description: Een verwerking ter beoordeling heeft een rechtsgrond.
    entity_type: verwerking
    enforce: write
    when: ["versiestatus=ter_beoordeling"]
    then: ["rechtsgrond!="]
  - name: advisory-only
    entity_type: verwerking
    then: ["rechtsgrond!=never"]
`

type modeFixture struct {
	mgr   *entitymanager.Manager
	store store.Store
}

func newModeManager(t *testing.T, mode entitymanager.ValidationMode) modeFixture {
	t.Helper()
	meta, err := metamodel.Parse([]byte(validationModeYAML))
	if err != nil {
		t.Fatal(err)
	}
	set, err := statemachine.Compile(meta)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := automation.NewEngineFromMetamodel(meta, meta.Automations)
	if err != nil {
		t.Fatal(err)
	}
	runner, err := autocascade.New(autocascade.Deps{Engine: engine})
	if err != nil {
		t.Fatal(err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL: acl.NopACL{}, Transitions: set,
		Automations: engine, Cascade: runner,
		FieldGate:      entitymanager.AllowAllFieldGate{},
		AnchorGuard:    entitymanager.NopAnchorGuard{},
		ValidationMode: mode,
	})
	if err != nil {
		t.Fatal(err)
	}
	return modeFixture{mgr: mgr, store: st}
}

// refusedTypes returns the error types of a refused write, or nil when err
// is not a validation refusal.
func refusedTypes(err error) []metamodel.ValidationErrorType {
	var invalid *entitymanager.ValidationError
	if !errors.As(err, &invalid) {
		return nil
	}
	out := make([]metamodel.ValidationErrorType, len(invalid.Errors))
	for i, e := range invalid.Errors {
		out[i] = e.Type
	}
	return out
}

// TestValidationMode_Create pins DEC-OYV2AK on create: strict refuses what
// advisory reports as a warning; structural success is unchanged.
func TestValidationMode_Create(t *testing.T) {
	tests := []struct {
		name        string
		mode        entitymanager.ValidationMode
		props       map[string]any
		wantRefused metamodel.ValidationErrorType // "" means the write succeeds
		wantWarning string
	}{
		{"advisory missing required", entitymanager.ValidationAdvisory,
			map[string]any{}, "", "required_property_unset"},
		{"strict missing required", entitymanager.ValidationStrict,
			map[string]any{}, metamodel.ValidationErrorRequired, ""},
		{"advisory invalid enum", entitymanager.ValidationAdvisory,
			map[string]any{"naam": "x", "versiestatus": "bogus"}, "", "property_value_invalid"},
		{"strict invalid enum", entitymanager.ValidationStrict,
			map[string]any{"naam": "x", "versiestatus": "bogus"}, metamodel.ValidationErrorInvalidValue, ""},
		{"strict valid", entitymanager.ValidationStrict,
			map[string]any{"naam": "x", "versiestatus": "concept"}, "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newModeManager(t, tc.mode)
			res, err := f.mgr.CreateEntity(context.Background(),
				&entity.Entity{Type: "verwerking", Properties: tc.props}, entity.CreateOptions{})
			if tc.wantRefused != "" {
				got := refusedTypes(err)
				if len(got) != 1 || got[0] != tc.wantRefused {
					t.Fatalf("CreateEntity err = %v, want one %q refusal", err, tc.wantRefused)
				}
				for e, lerr := range f.store.ListEntities(context.Background(),
					store.EntityQuery{Type: "verwerking", Faces: store.AllFaces()}) {
					if lerr != nil {
						t.Fatal(lerr)
					}
					t.Fatalf("a refused create stored %s", e.ID)
				}
				return
			}
			if err != nil {
				t.Fatalf("CreateEntity: %v", err)
			}
			if tc.wantWarning == "" {
				if len(res.Warnings) != 0 {
					t.Fatalf("warnings = %v, want none", res.Warnings)
				}
				return
			}
			if len(res.Warnings) != 1 || res.Warnings[0].Code != tc.wantWarning {
				t.Fatalf("warnings = %v, want one %q", res.Warnings, tc.wantWarning)
			}
		})
	}
}

// TestValidationMode_Update pins that strict judges the post-automation
// state: an automation that fills a required property lets the write pass.
func TestValidationMode_Update(t *testing.T) {
	tests := []struct {
		name        string
		mode        entitymanager.ValidationMode
		props       map[string]any
		wantRefused bool
	}{
		{"advisory clears required", entitymanager.ValidationAdvisory, map[string]any{}, false},
		{"strict clears required", entitymanager.ValidationStrict, map[string]any{}, true},
		{"strict automation fills required", entitymanager.ValidationStrict,
			map[string]any{"autofill": "yes"}, false},
		{"strict invalid enum", entitymanager.ValidationStrict,
			map[string]any{"naam": "x", "versiestatus": "bogus"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newModeManager(t, tc.mode)
			ctx := context.Background()
			created, err := f.mgr.CreateEntity(ctx,
				&entity.Entity{Type: "verwerking", Properties: map[string]any{"naam": "start"}},
				entity.CreateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			next := &entity.Entity{ID: created.Entity.ID, Type: "verwerking", Properties: tc.props}
			_, err = f.mgr.UpdateEntity(ctx, next)
			if tc.wantRefused {
				if refusedTypes(err) == nil {
					t.Fatalf("UpdateEntity err = %v, want a validation refusal", err)
				}
				stored, gerr := f.store.GetEntity(ctx, created.Entity.Ref())
				if gerr != nil {
					t.Fatal(gerr)
				}
				if stored.GetString("naam") != "start" {
					t.Fatalf("a refused update changed the row: naam = %q", stored.GetString("naam"))
				}
				return
			}
			if err != nil {
				t.Fatalf("UpdateEntity: %v", err)
			}
		})
	}
}

func TestValidationMode_StrictPatch(t *testing.T) {
	f := newModeManager(t, entitymanager.ValidationStrict)
	ctx := context.Background()
	created, err := f.mgr.CreateEntity(ctx,
		&entity.Entity{Type: "verwerking", Properties: map[string]any{"naam": "start"}}, entity.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.mgr.PatchEntity(ctx, created.Entity.ID, entity.Patch{
		Properties: map[string]any{"versiestatus": "bogus"},
	})
	if got := refusedTypes(err); len(got) != 1 || got[0] != metamodel.ValidationErrorInvalidValue {
		t.Fatalf("PatchEntity err = %v, want one invalid_value refusal", err)
	}
}

// TestValidationMode_DryRunStaysAdvisory pins that the create dry run
// reports, never refuses: the create form renders from it.
func TestValidationMode_DryRunStaysAdvisory(t *testing.T) {
	f := newModeManager(t, entitymanager.ValidationStrict)
	_, warnings, err := f.mgr.ValidateCreate(context.Background(),
		&entity.Entity{Type: "verwerking", Properties: map[string]any{}}, entity.CreateOptions{})
	if err != nil {
		t.Fatalf("ValidateCreate: %v", err)
	}
	if len(warnings) != 1 || warnings[0].Code != "required_property_unset" {
		t.Fatalf("warnings = %v, want one required_property_unset", warnings)
	}
}

// TestWriteRule_RefusesWrite pins TKT-5CESLHD: a rule declared
// `enforce: write` refuses a violating write on every write path, in
// either mode, and a rule without it does not.
func TestWriteRule_RefusesWrite(t *testing.T) {
	beoordeling := map[string]any{"naam": "x", "versiestatus": "ter_beoordeling"}
	complete := map[string]any{"naam": "x", "versiestatus": "ter_beoordeling", "rechtsgrond": "art. 6"}

	tests := []struct {
		name        string
		write       func(ctx context.Context, f modeFixture, id string) error
		wantRefused bool
	}{
		{"create violating", func(ctx context.Context, f modeFixture, _ string) error {
			_, err := f.mgr.CreateEntity(ctx, &entity.Entity{Type: "verwerking", Properties: beoordeling},
				entity.CreateOptions{})
			return err
		}, true},
		{"create satisfying", func(ctx context.Context, f modeFixture, _ string) error {
			_, err := f.mgr.CreateEntity(ctx, &entity.Entity{Type: "verwerking", Properties: complete},
				entity.CreateOptions{})
			return err
		}, false},
		{"update violating", func(ctx context.Context, f modeFixture, id string) error {
			_, err := f.mgr.UpdateEntity(ctx, &entity.Entity{ID: id, Type: "verwerking", Properties: beoordeling})
			return err
		}, true},
		{"patch transition violating", func(ctx context.Context, f modeFixture, id string) error {
			_, err := f.mgr.PatchEntity(ctx, id, entity.Patch{
				Properties: map[string]any{"versiestatus": "ter_beoordeling"}})
			return err
		}, true},
		{"patch transition satisfying", func(ctx context.Context, f modeFixture, id string) error {
			_, err := f.mgr.PatchEntity(ctx, id, entity.Patch{
				Properties: map[string]any{"versiestatus": "ter_beoordeling", "rechtsgrond": "art. 6"}})
			return err
		}, false},
		{"patch outside the rule", func(ctx context.Context, f modeFixture, id string) error {
			_, err := f.mgr.PatchEntity(ctx, id, entity.Patch{
				Properties: map[string]any{"versiestatus": "goedgekeurd"}})
			return err
		}, false},
	}
	for _, mode := range []entitymanager.ValidationMode{entitymanager.ValidationAdvisory, entitymanager.ValidationStrict} {
		for _, tc := range tests {
			t.Run(mode.String()+"/"+tc.name, func(t *testing.T) {
				f := newModeManager(t, mode)
				ctx := context.Background()
				seed, err := f.mgr.CreateEntity(ctx,
					&entity.Entity{Type: "verwerking", Properties: map[string]any{"naam": "x", "versiestatus": "concept"}},
					entity.CreateOptions{})
				if err != nil {
					t.Fatal(err)
				}
				err = tc.write(ctx, f, seed.Entity.ID)
				if !tc.wantRefused {
					if err != nil {
						t.Fatalf("write: %v", err)
					}
					return
				}
				got := refusedTypes(err)
				if len(got) != 1 || got[0] != metamodel.ValidationErrorRule {
					t.Fatalf("write err = %v, want one rule refusal", err)
				}
				stored, gerr := f.store.GetEntity(ctx, seed.Entity.Ref())
				if gerr != nil {
					t.Fatal(gerr)
				}
				if stored.GetString("versiestatus") != "concept" {
					t.Fatalf("a refused write changed the row: versiestatus = %q", stored.GetString("versiestatus"))
				}
				if n := countRows(ctx, t, f.store); n != 1 {
					t.Fatalf("rows = %d after a refused write, want only the seed", n)
				}
			})
		}
	}
}

func countRows(ctx context.Context, t *testing.T, st store.Store) int {
	t.Helper()
	n := 0
	for _, err := range st.ListEntities(ctx,
		store.EntityQuery{Type: "verwerking", Faces: store.AllFaces()}) {
		if err != nil {
			t.Fatal(err)
		}
		n++
	}
	return n
}

// TestWriteRule_PostCreateAutomation pins what a create returns when its
// on-create automation sets values that break a write-enforced rule: the
// automation's write is refused, the row createCore stored stays without
// those values, and the error says the entity was created, so a caller
// does not retry into a duplicate.
func TestWriteRule_PostCreateAutomation(t *testing.T) {
	f := newModeManager(t, entitymanager.ValidationAdvisory)
	ctx := context.Background()
	_, err := f.mgr.CreateEntity(ctx, &entity.Entity{Type: "verwerking",
		Properties: map[string]any{"naam": "x", "autofill": "submit"}}, entity.CreateOptions{})

	var kept *entitymanager.KeptEntityError
	if !errors.As(err, &kept) {
		t.Fatalf("CreateEntity err = %v, want a KeptEntityError", err)
	}
	if got := refusedTypes(err); len(got) != 1 || got[0] != metamodel.ValidationErrorRule {
		t.Fatalf("CreateEntity err = %v, want one rule refusal inside", err)
	}
	stored, err := f.store.GetEntity(ctx, kept.Entity.Ref())
	if err != nil {
		t.Fatal(err)
	}
	if v := stored.GetString("versiestatus"); v != "" {
		t.Fatalf("stored versiestatus = %q, want the automation's value refused", v)
	}
	if n := countRows(ctx, t, f.store); n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}
}

func TestParseValidationMode(t *testing.T) {
	tests := []struct {
		in      string
		want    entitymanager.ValidationMode
		wantErr bool
	}{
		{"", entitymanager.ValidationAdvisory, false},
		{"advisory", entitymanager.ValidationAdvisory, false},
		{"strict", entitymanager.ValidationStrict, false},
		{" strict ", entitymanager.ValidationAdvisory, true},
		{"blocking", entitymanager.ValidationAdvisory, true},
	}
	for _, tc := range tests {
		got, err := entitymanager.ParseValidationMode(tc.in)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("ParseValidationMode(%q) = %v, %v; want %v, err=%v", tc.in, got, err, tc.want, tc.wantErr)
		}
	}
}
