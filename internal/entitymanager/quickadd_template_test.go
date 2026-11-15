package entitymanager_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg/transitions"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/templating"
)

// oneTemplate serves one default entity template for "risk".
type oneTemplate struct{ tmpl *templating.Template }

func (o oneTemplate) EntityTemplate(_ context.Context, entityType, variant string) (*templating.Template, error) {
	if entityType == "risk" && variant == "" {
		return o.tmpl, nil
	}
	return nil, nil //nolint:nilnil // miss is not an error at this layer
}

func (oneTemplate) RelationTemplate(_ context.Context, _ string) (*templating.Template, error) {
	return nil, nil //nolint:nilnil // miss is not an error at this layer
}

// TestCreate_QuickAddShapeAppliesDefaultTemplate pins what the kanban
// quick-add relies on (TKT-IP4273S): a create carrying only a title and the
// column and swimlane values still gets the type's default template. The
// template fills what the card did not send, and loses where the card did.
func TestCreate_QuickAddShapeAppliesDefaultTemplate(t *testing.T) {
	meta, err := metamodel.Parse([]byte(`
version: "1.0"
entities:
  risk:
    label: Risk
    id_prefix: "RSK-"
    id_type: sequential
    properties:
      title: {type: string, required: true}
      stage: {type: enum, values: [open, discuss, decided]}
      owner: {type: string}
relations: {}
`))
	if err != nil {
		t.Fatal(err)
	}
	set, err := transitions.CompileTransitions(meta)
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: memstore.New(), Meta: meta, Audit: audit.Nop{}, ACL: acl.NopACL{}, Transitions: set,
		Templater: oneTemplate{&templating.Template{
			EntityType: "risk",
			Properties: map[string]any{"owner": "facilitator", "stage": "open"},
			Content:    "## Notes\n",
		}},
		FieldGate:   entitymanager.AllowAllFieldGate{},
		AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatal(err)
	}

	card := entity.New("", "risk")
	card.Properties = map[string]any{"title": "Data leak", "stage": "discuss"}
	res, err := mgr.CreateEntity(context.Background(), card, entity.CreateOptions{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got := res.Entity
	for prop, want := range map[string]string{"title": "Data leak", "stage": "discuss", "owner": "facilitator"} {
		if v := got.GetString(prop); v != want {
			t.Errorf("%s = %q, want %q", prop, v, want)
		}
	}
	if got.Content != "## Notes\n" {
		t.Errorf("content = %q, want the template body", got.Content)
	}
}
