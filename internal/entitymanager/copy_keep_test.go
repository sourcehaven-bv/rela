package entitymanager_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const keepMetaYAML = `
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
      signed_by: {type: string}
copies:
  stage-review:
    from: page@live
    to: page@review
    fields: all
    keep: [signed_by]
    guard:
      permission: stage
`

// TestCopyKeep pins `keep:` on a non-file property (TKT-2Z2IA6): the target
// face keeps its own value, and a new target face stays without one even
// when the source holds a value.
func TestCopyKeep(t *testing.T) {
	meta, err := metamodel.Parse([]byte(keepMetaYAML))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:       st,
		Meta:        meta,
		Templater:   nopTemplater{},
		Audit:       audit.Nop{},
		ACL:         acl.NopACL{},
		Transitions: statemachine.EmptySet(),
		FieldGate:   entitymanager.AllowAllFieldGate{},
		CopyGuard:   allowGuard{allow: true},
		AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	ctx := context.Background()
	seed := func(id string, face entity.Face, props map[string]any) {
		t.Helper()
		if err := st.CreateEntity(ctx, &entity.Entity{ID: id, Type: "page", Face: face, Properties: props}); err != nil {
			t.Fatalf("seed %s@%s: %v", id, face, err)
		}
	}
	seed("PAGE-1", "live", map[string]any{"title": "new", "signed_by": "source"})
	seed("PAGE-1", "review", map[string]any{"title": "old", "signed_by": "alice"})
	seed("PAGE-2", "live", map[string]any{"title": "new", "signed_by": "source"})

	tests := []struct {
		id         string
		wantSigned any
	}{
		{"PAGE-1", "alice"},
		{"PAGE-2", nil},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			if _, err := mgr.CopyState(ctx, entitymanager.CopyRequest{
				Definition: "stage-review", SourceID: tc.id,
			}); err != nil {
				t.Fatalf("copy: %v", err)
			}
			got, err := st.GetEntity(ctx, entity.Ref{ID: tc.id, Face: "review"})
			if err != nil {
				t.Fatalf("read review: %v", err)
			}
			if got.Properties["title"] != "new" {
				t.Errorf("title = %v, want the source's value", got.Properties["title"])
			}
			if signed := got.Properties["signed_by"]; signed != tc.wantSigned {
				t.Errorf("signed_by = %v, want %v", signed, tc.wantSigned)
			}
		})
	}
}
