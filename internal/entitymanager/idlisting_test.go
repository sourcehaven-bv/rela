package entitymanager_test

import (
	"context"
	"iter"
	"sync/atomic"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// idListingStore is a store that can list its entity ids without loading
// entities, like fsstore. It counts full scans so a test can pin that id
// generation uses the cheap listing.
type idListingStore struct {
	store.Store
	fullScans atomic.Int32
	idLists   atomic.Int32
}

func (s *idListingStore) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	if q.Faces.IsAll() && len(q.IDs) == 0 && q.Type == "" {
		s.fullScans.Add(1)
	}
	return s.Store.ListEntities(ctx, q)
}

func (s *idListingStore) EntityIDs(ctx context.Context) ([]string, error) {
	s.idLists.Add(1)
	var ids []string
	for e, err := range s.Store.ListEntities(ctx, store.EntityQuery{Faces: store.AllFaces()}) {
		if err != nil {
			return nil, err
		}
		ids = append(ids, e.ID)
	}
	return ids, nil
}

// TestCreate_GeneratesIDFromIDListing pins BUG-TT73D0P: minting an id must not
// load every entity in the store. On fsstore that scan read every entity
// file on each create, so a seed script slowed down quadratically and hit
// the script timeout.
func TestCreate_GeneratesIDFromIDListing(t *testing.T) {
	st := &idListingStore{Store: memstore.New()}
	meta := parseMeta(t)
	machines, err := statemachine.Compile(meta)
	if err != nil {
		t.Fatalf("statemachine.Compile: %v", err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:       st,
		Meta:        meta,
		Templater:   nopTemplater{},
		Audit:       audit.Nop{},
		ACL:         acl.NopACL{},
		Transitions: machines,
		FieldGate:   entitymanager.AllowAllFieldGate{},
		AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}

	first := createReq(t, mgr, "first")
	second := createReq(t, mgr, "second")

	if first.ID == second.ID {
		t.Fatalf("both creates got id %s", first.ID)
	}
	if got := st.idLists.Load(); got != 2 {
		t.Errorf("EntityIDs calls = %d, want 2 (one per create)", got)
	}
	if got := st.fullScans.Load(); got != 0 {
		t.Errorf("full entity scans = %d, want 0", got)
	}
}
