package search

import (
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// LinearSearch is a SearchIndex that performs brute-force substring matching.
// It is a simple in-memory fallback for backends that don't have a dedicated
// full-text index.
type LinearSearch struct {
	mu sync.RWMutex
	// entities is keyed per FACE, by [faceKey] — `PAGE-1` and `PAGE-1@draft`
	// are separate entries. Before worlds this was keyed on the bare id, so
	// indexing a non-default face OVERWROTE the default one; the stores
	// worked around that by not announcing non-default writes at all.
	entities     map[faceKey]*entity.Entity
	lastModified time.Time
}

// faceKey addresses one indexed face. A struct rather than a formatted
// string so the two components can never be ambiguous — an id containing the
// separator cannot forge another entity's face.
type faceKey struct {
	id   string
	face entity.Face
}

// NewLinearSearch creates a new linear substring search index.
func NewLinearSearch() *LinearSearch {
	return &LinearSearch{entities: make(map[faceKey]*entity.Entity)}
}

func (l *LinearSearch) EntityPut(e *entity.Entity) error {
	l.mu.Lock()
	l.entities[faceKey{e.ID, e.Face}] = e.Clone()
	l.advanceLastModified(e.UpdatedAt)
	l.mu.Unlock()
	return nil
}

// EntityDelete removes EVERY face of id.
//
// The bare-id callback cannot name a face, and a store only calls it when
// the whole entity is gone (see [store.FaceObserver]), so removing the whole
// family is the faithful reading. A per-face delete arrives through
// [LinearSearch.EntityFaceDelete] instead.
func (l *LinearSearch) EntityDelete(id string) error {
	l.mu.Lock()
	for k := range l.entities {
		if k.id == id {
			delete(l.entities, k)
		}
	}
	// A delete carries no mtime from the entity; use wall clock so the
	// timestamp still advances and consumers can observe the change.
	l.advanceLastModified(time.Now())
	l.mu.Unlock()
	return nil
}

// EntityFaceDelete removes exactly ONE face, leaving its siblings indexed.
// See [store.FaceObserver] for why this is a separate, optional capability
// rather than a widened EntityDelete.
func (l *LinearSearch) EntityFaceDelete(id string, p entity.Face) error {
	l.mu.Lock()
	delete(l.entities, faceKey{id, p})
	l.advanceLastModified(time.Now())
	l.mu.Unlock()
	return nil
}

// compile-time check: LinearSearch can evict a single face.
var _ store.FaceObserver = (*LinearSearch)(nil)

// EntityRenamed removes the old entity's faces and inserts a clone of the
// renamed one in one critical section so concurrent readers never observe a
// half-renamed state.
//
// Every face of oldID goes, not just the renamed one's coordinate: a rename
// re-keys the whole family, and leaving siblings behind would strand them
// under an id that no longer exists.
func (l *LinearSearch) EntityRenamed(oldID string, renamed *entity.Entity) error {
	l.mu.Lock()
	for k := range l.entities {
		if k.id == oldID {
			delete(l.entities, k)
		}
	}
	l.entities[faceKey{renamed.ID, renamed.Face}] = renamed.Clone()
	l.advanceLastModified(renamed.UpdatedAt)
	l.mu.Unlock()
	return nil
}

// LastModified returns the latest mtime observed by this index across all
// EntityPut and EntityDelete calls. Consumers compare this against the
// store's LastModified to decide whether the index needs repopulating.
func (l *LinearSearch) LastModified() time.Time {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lastModified
}

// advanceLastModified bumps the cursor forward; callers hold mu.
func (l *LinearSearch) advanceLastModified(t time.Time) {
	if t.After(l.lastModified) {
		l.lastModified = t
	}
}

// Search returns the matching FACES, resolved under world w.
//
// LinearSearch has no relevance scoring (it is brute-force substring
// matching), so the contract's "ordered by relevance" cannot apply; results
// are returned in natural-sort ID order instead — stable and deterministic
// across calls. All matches are collected before sorting and truncating, so
// a limit returns the first-N of that defined order rather than an arbitrary
// subset of the map's randomized iteration.
//
// # Resolve FIRST, match SECOND
//
// The world picks each entity's prime from the WHOLE family, and only that
// prime is then matched. Doing it the other way — matching first, then
// resolving the survivors — would let a non-prime face's text decide whether
// its entity appears at all: a `published`-world search would hit on a term
// that exists only in the draft while displaying published bytes that lack
// it. Resolution must see faces the query did not match, which is why the
// candidate set is built from the whole index rather than from the matches.
//
// # At most one hit per entity, structurally
//
// A world resolves at most one prime per entity, so the result holds at most
// one Face per id by construction — no grouping pass, and `limit` counts
// entities for free.
//
// The zero WorldScope is the default world: every entity resolves to its
// default face via rule 1, which is exactly the pre-worlds result set.
func (l *LinearSearch) Search(text string, limit int, w store.WorldScope) ([]Face, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	cands := make([]Candidate, 0, len(l.entities))
	for k, e := range l.entities {
		cands = append(cands, Candidate{ID: k.id, Type: e.Type, Face: k.face})
	}
	primes := ResolvePrimes(w, cands)

	var ids []string
	byID := make(map[string]Face, len(primes))
	for id, res := range primes {
		e, ok := l.entities[faceKey{id, res.Face}]
		if !ok {
			// The prime named a face this index does not hold. Resolution
			// only ever names a candidate it was given, so this is
			// unreachable; skipping keeps it from panicking if it ever is.
			continue
		}
		if !MatchText(e, text) {
			continue
		}
		ids = append(ids, id)
		byID[id] = Face{
			ID:            id,
			Face:          res.Face,
			Via:           res.Via,
			ChainPosition: res.ChainPosition,
		}
	}

	natsort.Strings(ids)
	if limit > 0 && len(ids) > limit {
		ids = ids[:limit]
	}
	out := make([]Face, 0, len(ids))
	for _, id := range ids {
		out = append(out, byID[id])
	}
	return out, nil
}

// MatchedFields reports which logical fields of e the query text matched.
// LinearSearch's matcher IS [MatchTextFields] (plain case-insensitive
// substring), so this is the ground-truth provenance the conformance suite
// checks the other backends against.
func (l *LinearSearch) MatchedFields(e *entity.Entity, text string) map[string]struct{} {
	return MatchTextFields(e, text)
}

// compile-time check: LinearSearch reports match provenance.
var _ FieldMatcher = (*LinearSearch)(nil)

func (l *LinearSearch) Close() error {
	return nil
}
