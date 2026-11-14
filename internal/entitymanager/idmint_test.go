package entitymanager_test

import (
	"context"
	"errors"
	"iter"
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

// idMintYAML declares one type per minting path: sequential and short.
const idMintYAML = `version: "1.0"
entities:
  requirement:
    label: Requirement
    plural: requirements
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
  idea:
    label: Idea
    plural: ideas
    id_prefix: "IDEA-"
    id_type: short
    properties:
      title:
        type: string
`

func newIDMintManager(t *testing.T, st store.Store) *entitymanager.Manager {
	t.Helper()
	meta, err := metamodel.Parse([]byte(idMintYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
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
	return mgr
}

func seedIDs(t *testing.T, st store.Store, typ string, ids ...string) {
	t.Helper()
	for _, id := range ids {
		if err := st.CreateEntity(context.Background(), entity.New(id, typ)); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
}

func mintOne(t *testing.T, mgr *entitymanager.Manager, typ string) (string, error) {
	t.Helper()
	res, err := mgr.CreateEntity(context.Background(), entity.New("", typ), entity.CreateOptions{})
	if err != nil {
		return "", err
	}
	return res.Entity.ID, nil
}

// TestCreate_MintsPastMixedCaseIDs pins BUG-RLBXTG5: hand-written ids in any
// case hold their number, so the next mint neither reuses nor collides with
// one. Before the fix only "REQ-" and "req-" were asked for, so "Req-001"
// was missed, REQ-001 was minted, and the create failed on the case-folded
// identity.
func TestCreate_MintsPastMixedCaseIDs(t *testing.T) {
	tests := []struct {
		name string
		typ  string
		seed []string
		want string
	}{
		{"mixed case below upper", "requirement", []string{"Req-001", "REQ-003", "rEq-007"}, "REQ-008"},
		{"mixed case only", "requirement", []string{"Req-001"}, "REQ-002"},
		{"lower case", "requirement", []string{"req-012"}, "REQ-013"},
		{"empty store", "requirement", nil, "REQ-001"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := memstore.New()
			seedIDs(t, st, tc.typ, tc.seed...)
			got, err := mintOne(t, newIDMintManager(t, st), tc.typ)
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if got != tc.want {
				t.Fatalf("minted %s, want %s", got, tc.want)
			}
		})
	}
}

// shortIDFaultStore fails or saturates the two reads mintShortID makes: the
// row count and the lookup of each round's candidates.
type shortIDFaultStore struct {
	store.Store
	countErr  error
	lookupErr error
	allTaken  bool
}

// Tx hands the callback this decorator so its overrides stay in effect
// inside a transaction.
func (s *shortIDFaultStore) Tx(_ context.Context, fn func(store.Store) error) error {
	return fn(s)
}

func (s *shortIDFaultStore) CountEntities(ctx context.Context, q store.EntityQuery) (int, error) {
	if s.countErr != nil {
		return 0, s.countErr
	}
	return s.Store.CountEntities(ctx, q)
}

func (s *shortIDFaultStore) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	if s.lookupErr != nil {
		return func(yield func(*entity.Entity, error) bool) { yield(nil, s.lookupErr) }
	}
	if s.allTaken && len(q.IDs) > 0 {
		return func(yield func(*entity.Entity, error) bool) {
			for _, id := range q.IDs {
				if !yield(entity.New(id, "idea"), nil) {
					return
				}
			}
		}
	}
	return s.Store.ListEntities(ctx, q)
}

// TestCreate_ShortIDFailurePaths pins that a short-id create fails rather
// than guesses when the store cannot say which ids are taken, and when no
// candidate is free (RR-WRYHU2J).
func TestCreate_ShortIDFailurePaths(t *testing.T) {
	countErr := errors.New("boom: count failed")
	lookupErr := errors.New("boom: lookup failed")
	tests := []struct {
		name    string
		st      *shortIDFaultStore
		wantErr error
		wantMsg string
	}{
		{"count fails", &shortIDFaultStore{countErr: countErr}, countErr, ""},
		{"candidate lookup fails", &shortIDFaultStore{lookupErr: lookupErr}, lookupErr, ""},
		{"every candidate taken", &shortIDFaultStore{allTaken: true}, nil, "no free short ID"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.st.Store = memstore.New()
			_, err := mintOne(t, newIDMintManager(t, tc.st), "idea")
			if err == nil {
				t.Fatal("create succeeded, want an error")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("got %v, want %v", err, tc.wantErr)
			}
			if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("got %v, want it to mention %q", err, tc.wantMsg)
			}
		})
	}
}

// TestCreate_ShortIDMints is the happy path beside the failures above: the
// id carries the prefix and is stored.
func TestCreate_ShortIDMints(t *testing.T) {
	st := memstore.New()
	got, err := mintOne(t, newIDMintManager(t, st), "idea")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !strings.HasPrefix(got, "IDEA-") {
		t.Fatalf("minted %s, want the IDEA- prefix", got)
	}
	if _, err := st.GetEntity(context.Background(), entity.Ref{ID: got}); err != nil {
		t.Fatalf("minted id not stored: %v", err)
	}
}
