package entitymanager_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/flowcfg/transitions"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// copyValidationModeYAML has a copy whose automations change the target:
// one sets an invalid enum value, one makes the target break a rule
// enforced on write. The source face is valid in both cases.
const copyValidationModeYAML = `
version: "1"
types:
  niveau:
    values: [laag, hoog]
entities:
  doc:
    label: Doc
    id_prefix: DOC
    faces:
      draft: {}
      adopted: {}
    properties:
      title: {type: string}
      niveau: {type: niveau}
      status: {type: string}
      rechtsgrond: {type: string}
copies:
  bad-enum:
    from: doc@draft
    to: doc@adopted
    fields: all
    guard:
      permission: adopt
  breaks-rule:
    from: doc@draft
    to: doc@adopted
    fields: all
    guard:
      permission: adopt
automations:
  - name: set-bad-enum
    on: {entity: doc, copy: bad-enum}
    do: [{set: niveau, value: middel}]
  - name: set-status
    on: {entity: doc, copy: breaks-rule}
    do: [{set: status, value: vastgesteld}]
validations:
  - name: rechtsgrond-bij-vaststelling
    entity_type: doc
    enforce: write
    when: ["status=vastgesteld"]
    then: ["rechtsgrond!="]
`

// TestCopyAutomations_ValidationMode pins that a copy checks the state its
// automations produce: strict mode refuses an invalid value an automation
// sets, and a rule enforced on write refuses the target in both modes.
// Nothing is written when the copy is refused.
func TestCopyAutomations_ValidationMode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		mode       entitymanager.ValidationMode
		definition string
		wantRefuse bool
	}{
		{name: "advisory keeps an invalid enum", mode: entitymanager.ValidationAdvisory, definition: "bad-enum"},
		{name: "strict refuses an invalid enum", mode: entitymanager.ValidationStrict, definition: "bad-enum",
			wantRefuse: true},
		{name: "advisory refuses a rule violation", mode: entitymanager.ValidationAdvisory,
			definition: "breaks-rule", wantRefuse: true},
		{name: "strict refuses a rule violation", mode: entitymanager.ValidationStrict,
			definition: "breaks-rule", wantRefuse: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mgr, st := newCopyModeManager(t, tc.mode)
			ctx := context.Background()
			if err := st.CreateEntity(ctx, &entity.Entity{
				ID: "DOC-1", Type: "doc", Face: "draft", Properties: map[string]any{"title": "t"},
			}); err != nil {
				t.Fatalf("seed: %v", err)
			}
			_, err := mgr.CopyState(ctx, entitymanager.CopyRequest{Definition: tc.definition, SourceID: "DOC-1"})
			_, getErr := st.GetEntity(ctx, entity.Ref{ID: "DOC-1", Face: "adopted"})
			if !tc.wantRefuse {
				if err != nil {
					t.Fatalf("copy: %v", err)
				}
				if getErr != nil {
					t.Fatalf("read adopted: %v", getErr)
				}
				return
			}
			var verr *entitymanager.ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("copy err = %v, want a ValidationError", err)
			}
			if getErr == nil {
				t.Error("the refused copy wrote the adopted face")
			}
		})
	}
}

func newCopyModeManager(t *testing.T, mode entitymanager.ValidationMode) (*entitymanager.Manager, store.Store) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(copyValidationModeYAML))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	engine, err := automation.NewEngineFromMetamodel(meta, meta.Automations)
	if err != nil {
		t.Fatalf("automation engine: %v", err)
	}
	runner, err := autocascade.New(autocascade.Deps{Engine: engine})
	if err != nil {
		t.Fatalf("autocascade.New: %v", err)
	}
	machines, err := transitions.CompileTransitions(meta)
	if err != nil {
		t.Fatalf("transitions.CompileTransitions: %v", err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:          st,
		Meta:           meta,
		Templater:      nopTemplater{},
		Audit:          audit.Nop{},
		ACL:            acl.NopACL{},
		Transitions:    machines,
		FieldGate:      entitymanager.AllowAllFieldGate{},
		CopyGuard:      allowGuard{allow: true},
		AnchorGuard:    entitymanager.NopAnchorGuard{},
		Automations:    engine,
		Cascade:        runner,
		ValidationMode: mode,
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr, st
}
