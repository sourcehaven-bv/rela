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

// prefixCaseYAML declares sequential types whose id_prefix is not upper
// case. entity.SequentialID mints an upper-case id for each of them.
const prefixCaseYAML = `version: "1.0"
entities:
  note:
    label: Note
    plural: notes
    id_prefix: "nt-"
    id_type: sequential
    properties:
      title:
        type: string
  memo:
    label: Memo
    plural: memos
    id_prefix: "Memo-"
    id_type: sequential
    properties:
      title:
        type: string
`

// TestCreate_MintsForNonUpperCasePrefix pins BUG-4OPP525: a sequential type
// with a lower- or mixed-case id_prefix mints an id the prefix check
// accepts. Before the fix the check compared case-sensitively, so the
// minted NT-001 was refused against "nt-" and no entity could be created.
func TestCreate_MintsForNonUpperCasePrefix(t *testing.T) {
	meta, err := metamodel.Parse([]byte(prefixCaseYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	tests := []struct {
		typ  string
		want []string
	}{
		{"note", []string{"NT-001", "NT-002"}},
		{"memo", []string{"MEMO-001", "MEMO-002"}},
	}
	for _, tc := range tests {
		t.Run(tc.typ, func(t *testing.T) {
			st := memstore.New()
			mgr, err := entitymanager.New(entitymanager.Deps{
				Store:       st,
				Meta:        meta,
				Templater:   nopTemplater{},
				Audit:       audit.Nop{},
				ACL:         acl.NopACL{},
				Transitions: statemachine.EmptySet(),
				FieldGate:   entitymanager.AllowAllFieldGate{},
				AnchorGuard: entitymanager.NopAnchorGuard{},
			})
			if err != nil {
				t.Fatalf("entitymanager.New: %v", err)
			}
			for _, want := range tc.want {
				res, err := mgr.CreateEntity(context.Background(), entity.New("", tc.typ), entity.CreateOptions{})
				if err != nil {
					t.Fatalf("create: %v", err)
				}
				if res.Entity.ID != want {
					t.Fatalf("minted %s, want %s", res.Entity.ID, want)
				}
			}
		})
	}
}
