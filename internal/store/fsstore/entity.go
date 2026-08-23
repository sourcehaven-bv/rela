package fsstore

import (
	"context"
	"fmt"
	"iter"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// --- EntityReader ---

func (s *FSStore) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	// The bare id IS the default state's index key (stateKey(id, zero)).
	return s.GetEntityState(ctx, id, "")
}

func (s *FSStore) GetEntityState(_ context.Context, id string, p entity.Pointer) (*entity.Entity, error) {
	key := stateKey(id, p)
	s.mu.RLock()
	meta, ok := s.entities[key]
	s.mu.RUnlock()

	if !ok {
		return nil, store.ErrNotFound
	}
	return s.loadEntityMeta(meta)
}

func (s *FSStore) ListEntities(_ context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	if err := storeutil.ValidateEntityQuery(q); err != nil {
		return func(yield func(*entity.Entity, error) bool) { yield(nil, err) }
	}
	s.mu.RLock()
	idSet := entityIDSet(q.IDs)

	// Collect matching metas from index.
	matches := make([]entityMeta, 0)
	for _, key := range s.entityOrder {
		if !matchEntityQuery(s.entities[key], q, idSet) {
			continue
		}
		matches = append(matches, s.entities[key])
	}
	s.mu.RUnlock()

	matches = keepPrimes(q.World, matches)

	return func(yield func(*entity.Entity, error) bool) {
		for _, m := range matches {
			e, err := s.loadEntityMeta(m)
			if err != nil {
				if !yield(nil, err) {
					return
				}
				continue
			}
			if !yield(e, nil) {
				return
			}
		}
	}
}

func (s *FSStore) ListEntitiesPage(_ context.Context, q store.EntityQuery) (store.Page[*entity.Entity], error) {
	if err := storeutil.ValidateEntityQuery(q); err != nil {
		return store.Page[*entity.Entity]{}, err
	}
	cursorKey, err := storeutil.DecodeCursor(q.Cursor)
	if err != nil {
		return store.Page[*entity.Entity]{}, err
	}

	s.mu.RLock()
	idSet := entityIDSet(q.IDs)
	matches := func(id string) bool { return matchEntityQuery(s.entities[id], q, idSet) }

	var keys storeutil.PageKeys
	if q.World.IsDefaultWorld() {
		keys = storeutil.PaginateSortedKeysFunc(
			s.entityOrder, cursorKey, q.Limit, matches, storeutil.CompareStateKeys)
	} else {
		// The world path buffers ONE family at a time and counts PRIMES
		// against the limit — see storeutil.PaginateWorldPrimes.
		keys = storeutil.PaginateWorldPrimes(
			s.entityOrder, cursorKey, q.Limit, q.World, matches,
			func(key string) (storeutil.WorldCandidate, bool) {
				m, ok := s.entities[key]
				if !ok {
					return storeutil.WorldCandidate{}, false
				}
				return storeutil.WorldCandidate{ID: m.ID, Type: m.Type, Pointer: m.Pointer}, true
			})
	}

	// Capture metas while still holding the lock — loading needs the
	// type and pointer and we must not do I/O under the lock.
	pairs := make([]entityMeta, 0, len(keys.Keys))
	for _, key := range keys.Keys {
		if meta, ok := s.entities[key]; ok {
			pairs = append(pairs, meta)
		}
	}
	s.mu.RUnlock()

	items := make([]*entity.Entity, 0, len(pairs))
	for _, p := range pairs {
		e, err := s.loadEntityMeta(p)
		if err != nil {
			return store.Page[*entity.Entity]{}, err
		}
		items = append(items, e)
	}
	return store.Page[*entity.Entity]{Items: items, NextCursor: keys.NextCursor}, nil
}

// entityIDSet builds the ID-lookup set used by ListEntities and its page
// variant. Returns nil when ids is empty so callers can test with len().
func entityIDSet(ids []string) map[string]bool {
	if len(ids) == 0 {
		return nil
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// matchEntityQuery adapts this backend's index metadata to the shared
// rule in storeutil. The rule itself must not live here: memstore
// applies the identical filter, and the two byte-similar copies this
// replaced are exactly the drift storetest exists to catch.
func matchEntityQuery(m entityMeta, q store.EntityQuery, idSet map[string]bool) bool {
	return storeutil.MatchEntityQuery(m.Type, m.ID, m.Pointer, q, idSet)
}

func (s *FSStore) CountEntities(_ context.Context, q store.EntityQuery) (int, error) {
	if err := storeutil.ValidateEntityQuery(q); err != nil {
		return 0, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	idSet := entityIDSet(q.IDs)

	// The default world resolves every row to itself, so counting needs
	// no buffer — and this is the common path for a project that never
	// declares a pointer, which must stay allocation-free.
	if q.World.IsDefaultWorld() {
		n := 0
		for _, meta := range s.entities {
			if matchEntityQuery(meta, q, idSet) {
				n++
			}
		}
		return n, nil
	}

	matches := make([]entityMeta, 0)
	for _, meta := range s.entities {
		if matchEntityQuery(meta, q, idSet) {
			matches = append(matches, meta)
		}
	}
	// Counts must be world-scoped, not raw: an unscoped tally tells a
	// published-world surface how many unpublished drafts exist.
	return len(keepPrimes(q.World, matches)), nil
}

// keepPrimes resolves a world over matched index metadata, returning
// only each entity's prime. A no-op for the default world.
func keepPrimes(w store.WorldScope, metas []entityMeta) []entityMeta {
	if w.IsDefaultWorld() || len(metas) == 0 {
		return metas
	}
	cands := make([]storeutil.WorldCandidate, len(metas))
	for i, m := range metas {
		cands[i] = storeutil.WorldCandidate{ID: m.ID, Type: m.Type, Pointer: m.Pointer}
	}
	primes := storeutil.WorldPrimes(w, cands)
	kept := make([]entityMeta, 0, len(primes))
	for _, m := range metas {
		if p, ok := primes[m.ID]; ok && p == m.Pointer {
			kept = append(kept, m)
		}
	}
	return kept
}

func (s *FSStore) HighestID(_ context.Context, prefix string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	highest := 0
	pfx := prefix + "-"
	for _, meta := range s.entities {
		if !meta.Pointer.IsDefault() {
			continue // states share the base id's number
		}
		id := meta.ID
		if !strings.HasPrefix(id, pfx) {
			continue
		}
		suffix := id[len(pfx):]
		var n int
		if _, err := fmt.Sscanf(suffix, "%d", &n); err == nil && n > highest {
			highest = n
		}
	}
	return highest, nil
}

func (s *FSStore) PropertyValues(_ context.Context, property string, limit int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts, ok := s.propCache[property]
	if !ok {
		return []string{}, nil
	}

	type vc struct {
		value string
		count int
	}
	sorted := make([]vc, 0, len(counts))
	for v, c := range counts {
		sorted = append(sorted, vc{v, c})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].count != sorted[j].count {
			return sorted[i].count > sorted[j].count
		}
		return sorted[i].value < sorted[j].value
	})

	result := make([]string, 0, limit)
	for i := 0; i < len(sorted) && (limit == 0 || i < limit); i++ {
		result = append(result, sorted[i].value)
	}
	return result, nil
}

// --- EntityWriter ---

// idTaken reports whether any key of index case-folds to the same identity as
// id. except is skipped so a rename can ask "does any OTHER entity claim this
// identity?" without self-colliding.
//
// Scans the existing index on each call rather than maintaining a second
// case-folded map: the entity index is mutated at several sites, and a
// parallel index would silently drift at whichever one a future change
// forgets to update. Runs only on create and rename, never on a read path.
//
// A free function rather than a method — FSStore is at its plimsoll
// max-methods line, and this needs no receiver state beyond the index.
// Callers must hold s.mu.
// idTaken folds on the BARE entity id (meta.ID): any state of an
// entity claims the whole identity, so a new id collides with an
// existing family regardless of which states it has.
//
// except is a BARE id matched case-folded — it skips that entire
// family, not one exact key (a rename may change its own casing, and
// its states must not self-collide either). Wider than the historical
// exact-key skip on purpose.
func idTaken(index map[string]entityMeta, id, except string) bool {
	folded := storeutil.FoldID(id)
	exceptFolded := ""
	if except != "" {
		exceptFolded = storeutil.FoldID(except)
	}
	for _, meta := range index {
		if exceptFolded != "" && storeutil.FoldID(meta.ID) == exceptFolded {
			continue
		}
		if storeutil.FoldID(meta.ID) == folded {
			return true
		}
	}
	return false
}

func (s *FSStore) createEntity(_ context.Context, e *entity.Entity) error {
	if err := storeutil.ValidateID(e.ID); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := stateKey(e.ID, e.Pointer)
	if e.Pointer.IsDefault() {
		// Case-folded: on a case-insensitive filesystem (macOS, Windows)
		// "ABC" and "abc" are the same file, so a byte-exact check here
		// would let the write silently overwrite the existing entity
		// (BUG-3RCWNS).
		if idTaken(s.entities, e.ID, "") {
			return store.ErrConflict
		}
	} else {
		// Row-family invariants (TKT-DOFYR1, design doc §6): a
		// non-default state requires its default row (no headless
		// states) and must share the family's type. Enforced at the
		// store so every direct writer hits one choke point; the LOAD
		// path deliberately tolerates violations found on disk.
		def, ok := s.entities[e.ID]
		if !ok {
			return storeutil.HeadlessStateError(e.ID)
		}
		if def.Type != e.Type {
			return storeutil.StateTypeMismatchError(e.ID, e.Pointer, e.Type, def.Type)
		}
		if _, exists := s.entities[key]; exists {
			return store.ErrConflict
		}
	}

	// Write to disk
	stored := e.Clone()
	stored.UpdatedAt = time.Now()
	if err := s.writeEntity(stored); err != nil {
		return err
	}

	// Update index
	s.entities[key] = entityMeta{ID: e.ID, Type: e.Type, Pointer: e.Pointer}
	s.entityOrder = storeutil.SortedInsertFunc(s.entityOrder, key, storeutil.CompareStateKeys)
	if stored.Pointer.IsDefault() {
		addEntityToCache(s.propCache, stored)
	}
	s.notifyPut(stored)

	s.emit(store.Event{
		Op:         store.EventEntityCreated,
		EntityType: e.Type,
		EntityID:   e.ID,
		Pointer:    e.Pointer,
	})
	return nil
}

func (s *FSStore) updateEntity(_ context.Context, e *entity.Entity) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := stateKey(e.ID, e.Pointer)
	meta, exists := s.entities[key]
	if !exists {
		return store.ErrNotFound
	}
	// Row-family invariant: a non-default state cannot be re-typed away
	// from its family (TKT-DOFYR1, design doc §6).
	if !e.Pointer.IsDefault() && e.Type != meta.Type {
		return storeutil.StateTypeMismatchError(e.ID, e.Pointer, e.Type, meta.Type)
	}

	// Load old entity for prop cache diff.
	old, err := s.loadEntityMeta(meta)
	if err != nil {
		return err
	}

	stored := e.Clone()
	stored.UpdatedAt = time.Now()
	if err := s.writeEntity(stored); err != nil {
		return err
	}

	// The entity's type determines its file path, so a type change wrote the
	// record at the NEW type's path — the file at the old path must go too,
	// or it would resurrect the stale record on the next reload. Part of the
	// type-change-on-update store contract (storetest UpdateChangesType).
	if meta.Type != e.Type {
		oldKey := s.entityFileKey(meta.Type, e.ID)
		if err := s.rooted.Remove(oldKey); err != nil && !os.IsNotExist(err) {
			return err
		}
		s.echoes.Forget(s.absPath(oldKey))
	}

	// Update index
	s.entities[key] = entityMeta{ID: e.ID, Type: e.Type, Pointer: e.Pointer}
	if e.Pointer.IsDefault() {
		removeEntityFromCache(s.propCache, old)
		addEntityToCache(s.propCache, stored)
	}
	s.notifyPut(stored)

	s.emit(store.Event{
		Op:         store.EventEntityUpdated,
		EntityType: e.Type,
		EntityID:   e.ID,
		Pointer:    e.Pointer,
	})
	return nil
}

// stateFamily returns every indexed state of the bare id, sorted by
// pointer (default first), plus every relation touching the id on
// either endpoint. Defensive family scan (works for a headless family
// tolerated from disk, design doc §6); the bare From/To match sweeps
// every tail pointer. Callers must hold s.mu.
func (s *FSStore) stateFamily(id string) (family []entityMeta, related []relationMeta) {
	for _, meta := range s.entities {
		if meta.ID == id {
			family = append(family, meta)
		}
	}
	sort.Slice(family, func(i, j int) bool { return family[i].Pointer < family[j].Pointer })
	for _, rm := range s.relations {
		if rm.From == id || rm.To == id {
			related = append(related, rm)
		}
	}
	return family, related
}

func (s *FSStore) deleteEntity(_ context.Context, id string, cascade bool) (*store.DeleteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Delete addresses the whole state FAMILY of the bare id
	// (TKT-DOFYR1): every state row plus all their relations.
	family, related := s.stateFamily(id)
	if len(family) == 0 {
		return nil, store.ErrNotFound
	}

	if !cascade && len(related) > 0 {
		return nil, fmt.Errorf("%w: entity %s has %d relation(s)", store.ErrHasRelations, id, len(related))
	}

	// Load every state for the result and prop cache.
	states := make([]*entity.Entity, 0, len(family))
	for _, meta := range family {
		e, err := s.loadEntityMeta(meta)
		if err != nil {
			return nil, err
		}
		states = append(states, e)
	}

	// Load relations for result.
	deletedRelations := make([]*entity.Relation, 0)
	for _, rm := range related {
		r, loadErr := s.loadRelationMeta(rm)
		if loadErr != nil {
			r = entity.NewRelation(rm.From, rm.Type, rm.To)
			r.FromPointer = rm.FromPointer
		}
		deletedRelations = append(deletedRelations, r)
	}

	// Delete relation files first, then the entity files. A real removal
	// error (not "already gone") aborts before the entity files are touched
	// and before the in-memory index is mutated, so the entity is never
	// removed while one of its relations could not be — fail-secure, no
	// orphaned-from entity (BUG-C20T / issue #888). Not transactional: a
	// relation file removed before a later failure stays removed, but the
	// whole op runs under s.mu and the index is updated only on success.
	for _, rm := range related {
		key := s.relationFileKeyMeta(rm)
		if err := s.rooted.Remove(key); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("delete relation file %s: %w", rm.key(), err)
		}
		s.echoes.Forget(s.absPath(key))
	}
	for _, meta := range family {
		key := s.entityFileKey(meta.Type, stateKey(meta.ID, meta.Pointer))
		if err := s.rooted.Remove(key); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		s.echoes.Forget(s.absPath(key))
	}

	// Cascade attachments — under the per-entity layout the
	// attachment directory is owned 1:1 by the entity.
	if err := s.removeAttachmentDir(id); err != nil {
		return nil, err
	}

	// Update index
	for i, meta := range family {
		key := stateKey(meta.ID, meta.Pointer)
		delete(s.entities, key)
		s.entityOrder = storeutil.SortedRemoveFunc(s.entityOrder, key, storeutil.CompareStateKeys)
		if meta.Pointer.IsDefault() {
			removeEntityFromCache(s.propCache, states[i])
		}
	}
	s.notifyDelete(id)

	for _, rm := range related {
		key := rm.key()
		delete(s.relations, key)
		s.relationOrder = storeutil.SortedRemove(s.relationOrder, key)
	}

	result := &store.DeleteResult{
		DeletedEntities:  states,
		DeletedRelations: deletedRelations,
	}
	s.emitFamilyDeleted(family, related)
	return result, nil
}

// deleteEntityState removes ONE face and only the edges belonging to it
// (TKT-C1XUA8). Contrast deleteEntity above, which sweeps the whole family
// and every incident edge on both sides.
//
// Keeps that function's fail-secure file ordering: relation files first,
// then the entity file, with a real removal error aborting before the
// in-memory index is touched.
func (s *FSStore) deleteEntityState(
	_ context.Context, id string, p entity.Pointer,
) (*store.DeleteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := stateKey(id, p)
	meta, ok := s.entities[key]
	if !ok {
		return nil, store.ErrNotFound
	}

	// Refuse to orphan the family: a family with no default row has no
	// defined meaning and world fallback resolves against it. Deleting the
	// LAST face is fine — nothing is left to orphan.
	if p.IsDefault() {
		if n := s.familySize(id); n > 1 {
			return nil, fmt.Errorf(
				"%w: cannot delete the default face of %s while %d other state(s) remain",
				store.ErrInvalidQuery, id, n-1)
		}
	}

	e, err := s.loadEntityMeta(meta)
	if err != nil {
		return nil, err
	}

	// OUTGOING edges on this tail go with the face. INCOMING edges do NOT:
	// heads are entity-level (§2.3), so an inbound edge points at the entity
	// and survives its faces.
	var owned []relationMeta
	for _, rm := range s.relations {
		if rm.From == id && rm.FromPointer == p {
			owned = append(owned, rm)
		}
	}
	sort.Slice(owned, func(i, j int) bool { return owned[i].key() < owned[j].key() })

	deletedRelations := make([]*entity.Relation, 0, len(owned))
	for _, rm := range owned {
		r, loadErr := s.loadRelationMeta(rm)
		if loadErr != nil {
			r = entity.NewRelation(rm.From, rm.Type, rm.To)
			r.FromPointer = rm.FromPointer
		}
		deletedRelations = append(deletedRelations, r)
	}

	for _, rm := range owned {
		fileKey := s.relationFileKeyMeta(rm)
		if rerr := s.rooted.Remove(fileKey); rerr != nil && !os.IsNotExist(rerr) {
			return nil, fmt.Errorf("delete relation file %s: %w", rm.key(), rerr)
		}
		s.echoes.Forget(s.absPath(fileKey))
	}
	entKey := s.entityFileKey(meta.Type, key)
	if rerr := s.rooted.Remove(entKey); rerr != nil && !os.IsNotExist(rerr) {
		return nil, rerr
	}
	s.echoes.Forget(s.absPath(entKey))

	// The attachment directory is owned 1:1 by the ENTITY, not by a face, so
	// it only goes when the last face does — a discarded draft must not
	// destroy attachments the surviving faces serve.
	//
	// This runs BEFORE the index mutations below, matching deleteEntity's
	// fail-secure ordering: every fallible filesystem operation completes
	// first, so an error returns with the in-memory index still matching what
	// is on disk. Removing it afterwards would leave a caller who saw an
	// error with the entity already gutted from the index — a divergence
	// nothing reconciles until restart.
	lastFace := s.familySize(id) == 1
	if lastFace {
		if aerr := s.removeAttachmentDir(id); aerr != nil {
			return nil, aerr
		}
	}

	delete(s.entities, key)
	s.entityOrder = storeutil.SortedRemoveFunc(s.entityOrder, key, storeutil.CompareStateKeys)
	if p.IsDefault() {
		removeEntityFromCache(s.propCache, e)
	}
	for _, rm := range owned {
		rk := rm.key()
		delete(s.relations, rk)
		s.relationOrder = storeutil.SortedRemove(s.relationOrder, rk)
	}

	// Observers are bare-id keyed, so notifying on a per-face delete would
	// de-index an entity that still exists.
	if lastFace {
		s.notifyDelete(id)
	}

	s.emitFamilyDeleted([]entityMeta{meta}, owned)
	return &store.DeleteResult{
		DeletedEntities:  []*entity.Entity{e},
		DeletedRelations: deletedRelations,
	}, nil
}

// familySize counts the indexed state rows of a bare id. Callers must hold
// s.mu.
func (s *FSStore) familySize(id string) int {
	n := 0
	for _, meta := range s.entities {
		if meta.ID == id {
			n++
		}
	}
	return n
}

// emitFamilyDeleted emits per-state entity-deleted events and per-edge
// relation-deleted events for a cascaded family delete. Each event
// carries its own state's type — the load path tolerates a mistyped
// state on disk, so famType-for-all would misreport it.
func (s *FSStore) emitFamilyDeleted(family []entityMeta, related []relationMeta) {
	for _, meta := range family {
		s.emit(store.Event{
			Op:         store.EventEntityDeleted,
			EntityType: meta.Type,
			EntityID:   meta.ID,
			Pointer:    meta.Pointer,
		})
	}
	for _, rm := range related {
		s.emit(store.Event{
			Op:           store.EventRelationDeleted,
			RelationType: rm.Type,
			From:         rm.From,
			To:           rm.To,
			Pointer:      rm.FromPointer,
		})
	}
}

// rewriteRelationFiles rewrites every relation file in metas, moving
// endpoints from oldID to newID; the tail pointer rides along
// unchanged. Old files are removed after the new ones are written.
func (s *FSStore) rewriteRelationFiles(metas []relationMeta, oldID, newID string) error {
	for _, rm := range metas {
		r, loadErr := s.loadRelationMeta(rm)
		if loadErr != nil {
			r = entity.NewRelation(rm.From, rm.Type, rm.To)
			r.FromPointer = rm.FromPointer
		}
		if r.From == oldID {
			r.From = newID
		}
		if r.To == oldID {
			r.To = newID
		}
		if writeErr := s.writeRelation(r); writeErr != nil {
			return writeErr
		}
		oldKey := s.relationFileKeyMeta(rm)
		_ = s.rooted.Remove(oldKey)
		s.echoes.Forget(s.absPath(oldKey))
	}
	return nil
}

func (s *FSStore) renameEntity(_ context.Context, oldID, newID string) (*store.RenameResult, error) {
	if err := storeutil.ValidateID(newID); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Rename addresses the whole state FAMILY of the bare id
	// (TKT-DOFYR1): every state file is re-keyed onto the new id, and
	// every relation of every state follows.
	family, toUpdate := s.stateFamily(oldID)
	if len(family) == 0 {
		return nil, store.ErrNotFound
	}

	// except=oldID: an entity may change its own casing (abc -> ABC).
	// idTaken folds on the bare id, so ANY state of another entity under
	// the new id is a conflict.
	if idTaken(s.entities, newID, oldID) {
		return nil, store.ErrConflict
	}

	// Load, re-id, and write every state; remember the default state
	// for the observers.
	var renamedDefault *entity.Entity
	renamedStates := make([]*entity.Entity, 0, len(family))
	for _, meta := range family {
		e, err := s.loadEntityMeta(meta)
		if err != nil {
			return nil, err
		}
		renamed := e.Clone()
		renamed.ID = newID
		renamed.UpdatedAt = time.Now()
		if err := s.writeEntity(renamed); err != nil {
			return nil, err
		}
		renamedStates = append(renamedStates, renamed)
		if renamed.Pointer.IsDefault() {
			renamedDefault = renamed
		}
	}

	if err := s.rewriteRelationFiles(toUpdate, oldID, newID); err != nil {
		return nil, err
	}

	// Delete old entity files.
	for _, meta := range family {
		oldKey := s.entityFileKey(meta.Type, stateKey(meta.ID, meta.Pointer))
		_ = s.rooted.Remove(oldKey)
		s.echoes.Forget(s.absPath(oldKey))
	}

	// Move the attachment directory (if any) onto the new ID.
	if err := s.renameAttachmentDir(oldID, newID); err != nil {
		return nil, err
	}

	// Update entity index.
	for _, meta := range family {
		oldKey := stateKey(meta.ID, meta.Pointer)
		delete(s.entities, oldKey)
		s.entityOrder = storeutil.SortedRemoveFunc(s.entityOrder, oldKey, storeutil.CompareStateKeys)
		newKey := stateKey(newID, meta.Pointer)
		s.entities[newKey] = entityMeta{ID: newID, Type: meta.Type, Pointer: meta.Pointer}
		s.entityOrder = storeutil.SortedInsertFunc(s.entityOrder, newKey, storeutil.CompareStateKeys)
	}
	// Observers key documents by bare id: a headless family (tolerated
	// from disk) has NO default face to rename in the index, and handing
	// them a state would overwrite the bare-id document — the exact leak
	// the notifyPut skip prevents (TKT-DOFYR1 review).
	if renamedDefault != nil {
		s.notifyRenamed(oldID, renamedDefault)
	}

	// Update relation index.
	for _, rm := range toUpdate {
		oldKey := rm.key()
		delete(s.relations, oldKey)
		s.relationOrder = storeutil.SortedRemove(s.relationOrder, oldKey)

		newFrom, newTo := rm.From, rm.To
		if newFrom == oldID {
			newFrom = newID
		}
		if newTo == oldID {
			newTo = newID
		}
		newMeta := relationMeta{From: newFrom, Type: rm.Type, To: newTo, FromPointer: rm.FromPointer}
		newKey := newMeta.key()
		s.relations[newKey] = newMeta
		s.relationOrder = storeutil.SortedInsert(s.relationOrder, newKey)
	}

	for _, renamed := range renamedStates {
		s.emit(store.Event{
			Op:         store.EventEntityUpdated,
			EntityType: renamed.Type,
			EntityID:   newID,
			Pointer:    renamed.Pointer,
		})
	}

	return &store.RenameResult{RelationsUpdated: len(toUpdate)}, nil
}

// writeEntity writes an entity to disk using temp-file + rename.
func (s *FSStore) writeEntity(e *entity.Entity) error {
	return s.writeEntityFile(e)
}
