package entitymanager_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg/transitions"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg/triggers"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const currentUserMeta = `version: "1.0"
entities:
  note:
    label: Note
    id_prefix: NOTE
    properties:
      body: {type: string}
  person:
    label: Person
    id_prefix: PERS
    properties:
      login: {type: string}
relations:
  authored-by:
    from: [note]
    to: [person]
`

// TestCreate_CurrentUserTriggerRelatesAuthor runs {{current_user.id}}
// (TKT-CY0NTFS) through a real create and its cascade: the note is linked to
// the person the principal resolved to, and to nobody without one.
func TestCreate_CurrentUserTriggerRelatesAuthor(t *testing.T) {
	t.Parallel()
	author := principal.Principal{User: "PERS-1", RawUser: "ann@example.com", Tool: principal.ToolDataEntry}
	tests := []struct {
		name       string
		ctx        context.Context //nolint:containedctx // table fixture
		wantAuthor bool
	}{
		{name: "resolved user", ctx: principal.With(context.Background(), author), wantAuthor: true},
		{name: "raw principal", ctx: principal.With(context.Background(),
			principal.Principal{User: "ann", Tool: principal.ToolCLI})},
		{name: "no principal", ctx: context.Background()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mgr, st := newCurrentUserManager(t, author.User)
			res, err := mgr.CreateEntity(tc.ctx, entity.New("", "note"), entity.CreateOptions{})
			if err != nil {
				t.Fatalf("CreateEntity: %v", err)
			}
			if len(res.AutomationErrors) > 0 {
				t.Fatalf("automation errors: %v", res.AutomationErrors)
			}
			key := entity.RelationKey{From: res.Entity.ID, Type: "authored-by", To: author.User}
			_, gotErr := st.GetRelation(context.Background(), key)
			if tc.wantAuthor && gotErr != nil {
				t.Fatalf("authored-by relation missing: %v", gotErr)
			}
			if !tc.wantAuthor {
				n := 0
				for _, err := range st.ListRelations(context.Background(), store.RelationQuery{Type: "authored-by"}) {
					if err != nil {
						t.Fatalf("list: %v", err)
					}
					n++
				}
				if n != 0 {
					t.Fatalf("got %d authored-by relations, want none", n)
				}
			}
		})
	}
}

func newCurrentUserManager(t *testing.T, personID string) (*entitymanager.Manager, store.Store) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(currentUserMeta))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	st := memstore.New()
	if seedErr := st.CreateEntity(context.Background(), entity.New(personID, "person")); seedErr != nil {
		t.Fatalf("seed: %v", seedErr)
	}
	engine := triggers.NewEngine([]triggers.Rule{{
		Name: "note-author",
		On:   triggers.Trigger{Entity: []string{"note"}, Created: true},
		Do: []triggers.Action{{CreateRelation: &triggers.CreateRelationAction{
			Relation: "authored-by", To: "{{current_user.id}}",
		}}},
	}})
	runner, err := autocascade.New(autocascade.Deps{Engine: engine})
	if err != nil {
		t.Fatalf("autocascade.New: %v", err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:       st,
		Meta:        meta,
		Templater:   nopTemplater{},
		Audit:       audit.Nop{},
		ACL:         acl.NopACL{},
		Automations: engine,
		Cascade:     runner,
		Transitions: transitions.NoTransitions(),
		FieldGate:   entitymanager.AllowAllFieldGate{},
		AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr, st
}
