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

// TestCreate_FaceMessagesUseDeclarationOrder pins that a refused create lists
// the type's faces in the schema's declaration order, not by name
// (RR-V3UH7K).
func TestCreate_FaceMessagesUseDeclarationOrder(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`
entities:
  page:
    label: Page
    id_prefix: PG
    faces:
      published: {}
      draft: {}
    properties:
      title: {type: string}
`))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: memstore.New(), Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(),
		FieldGate: entitymanager.AllowAllFieldGate{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}

	for name, face := range map[string]entity.Face{"no face": "", "undeclared face": "nonsuch"} {
		t.Run(name, func(t *testing.T) {
			_, err := mgr.CreateEntity(context.Background(), &entity.Entity{
				Type: "page", Properties: map[string]any{"title": "x"},
			}, entity.CreateOptions{Face: face})
			if err == nil || !strings.Contains(err.Error(), "published, draft") {
				t.Fatalf("err = %v, want the faces in declaration order (published, draft)", err)
			}
		})
	}
}
