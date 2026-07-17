package sync

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// LocalSnapshot is the result of enumerating + hashing the working copy: every
// live record keyed by its wire key, ready to diff against the index.
type LocalSnapshot struct {
	Records map[string]LocalRecord
}

// SnapshotLocal enumerates every entity and relation in the store and computes
// its canonical hash. Locked (e.g. git-crypt) records are skipped: their
// properties are unreadable, so hashing or pushing them would corrupt remote
// data with cleartext form — a documented limitation, not a silent drop (the
// caller reports the count).
func SnapshotLocal(ctx context.Context, st store.Store) (*LocalSnapshot, int, error) {
	snap := &LocalSnapshot{Records: map[string]LocalRecord{}}
	locked := 0

	for e, err := range st.ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			return nil, locked, fmt.Errorf("list entities: %w", err)
		}
		if e.IsLocked() {
			locked++
			continue
		}
		key := EntityKey(e.ID)
		snap.Records[key] = LocalRecord{
			Key: key, Kind: KindEntity, Hash: canonical.HashEntity(*e), Entity: e,
		}
	}

	for r, err := range st.ListRelations(ctx, store.RelationQuery{}) {
		if err != nil {
			return nil, locked, fmt.Errorf("list relations: %w", err)
		}
		if r.IsLocked() {
			locked++
			continue
		}
		key := RelationKey(r.From, r.Type, r.To)
		snap.Records[key] = LocalRecord{
			Key: key, Kind: KindRelation, Hash: canonical.HashRelation(*r), Relation: r,
		}
	}
	return snap, locked, nil
}

// LocalChange classifies how a local record differs from the index.
type LocalChange struct {
	Record  LocalRecord // the live record (zero Entity/Relation when Deleted)
	Key     string
	Kind    Kind
	Base    string // index hash (the agreed base); "" for a new record
	Deleted bool   // present in index, absent in working copy
}

// DiffLocal compares the working-copy snapshot against the index and returns the
// records that have diverged from the agreed baseline: created (in working copy,
// not in index), updated (hash differs from index), and deleted (in index, gone
// from working copy). Records whose hash equals the index are omitted — they are
// already in sync. The result is unordered; the push command applies topological
// ordering.
func DiffLocal(snap *LocalSnapshot, idx *State) []LocalChange {
	var changes []LocalChange

	for key, rec := range snap.Records {
		base, indexed := idx.Hash(key)
		switch {
		case !indexed:
			changes = append(changes, LocalChange{Record: rec, Key: key, Kind: rec.Kind, Base: ""})
		case base != rec.Hash:
			changes = append(changes, LocalChange{Record: rec, Key: key, Kind: rec.Kind, Base: base})
		}
	}

	// Deletions: in the index but no longer present locally.
	for key, base := range idx.Records {
		if _, stillPresent := snap.Records[key]; stillPresent {
			continue
		}
		changes = append(changes, LocalChange{
			Key:     key,
			Kind:    kindFromKey(key),
			Base:    base,
			Deleted: true,
		})
	}
	return changes
}

// kindFromKey infers a record's kind from its index key shape. An entity key is
// a bare id (no slash); a relation key is "from/type/to". The index does not
// store kind separately, so we recover it structurally — safe because
// validIDSegment forbids slashes inside any segment.
func kindFromKey(key string) Kind {
	for _, c := range key {
		if c == '/' {
			return KindRelation
		}
	}
	return KindEntity
}
