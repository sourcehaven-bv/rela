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

// prefixCaseYAML declares types whose id_prefix is not upper case. For the
// sequential ones entity.SequentialID mints an upper-case id.
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
  doc:
    label: Doc
    plural: docs
    id_prefix: "doc-"
    id_type: manual
    properties:
      title:
        type: string
`

func newPrefixCaseManager(t *testing.T) *entitymanager.Manager {
	t.Helper()
	meta, err := metamodel.Parse([]byte(prefixCaseYAML))
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
		AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr
}

// TestCreate_MintsForNonUpperCasePrefix pins BUG-4OPP525: a sequential type
// with a lower- or mixed-case id_prefix mints an id the prefix check
// accepts. Before the fix the check compared case-sensitively, so the
// minted NT-001 was refused against "nt-" and no entity could be created.
func TestCreate_MintsForNonUpperCasePrefix(t *testing.T) {
	tests := []struct {
		typ  string
		want []string
	}{
		{"note", []string{"NT-001", "NT-002"}},
		{"memo", []string{"MEMO-001", "MEMO-002"}},
	}
	for _, tc := range tests {
		t.Run(tc.typ, func(t *testing.T) {
			mgr := newPrefixCaseManager(t)
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

// TestCreate_ManualIDPrefixFoldsCase pins the other side of BUG-4OPP525: a
// manual id passes the prefix check in any case, and an id with another
// prefix is still refused.
func TestCreate_ManualIDPrefixFoldsCase(t *testing.T) {
	tests := []struct {
		id      string
		wantErr bool
	}{
		{"doc-intro", false},
		{"DOC-setup", false},
		{"note-x", true},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			mgr := newPrefixCaseManager(t)
			_, err := mgr.CreateEntity(context.Background(), entity.New("", "doc"), entity.CreateOptions{ID: tc.id})
			if gotErr := err != nil; gotErr != tc.wantErr {
				t.Fatalf("create %s: err = %v, want error %v", tc.id, err, tc.wantErr)
			}
		})
	}
}
