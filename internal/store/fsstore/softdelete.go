package fsstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path"
	"sort"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// Soft delete (store.SoftDeleter) on the filesystem.
//
// A mark does not touch the entity or relation files. It records the id in
// .rela/pending-deletes.json and moves the family's index entries into
// FSStore.marked, so every read (all of which go through the index) stops
// seeing them. Files stay where they are until PurgeMarked removes them.
//
// Why a list file rather than renaming or rewriting the files: it survives a
// restart without the watcher firing and without a git-visible change for a
// delete that may yet be undone. The cost is that another process on the same
// project keeps showing the entity until the purge removes its files; the CLI
// and every other process read the list when they open the store, so only a
// process that was already running sees it late.

const pendingDeletesFile = "pending-deletes.json"

// fsMarked is one marked family, held out of the live index.
type fsMarked struct {
	at        time.Time
	by        string
	entities  []entityMeta
	relations map[string]relationMeta
}

// pendingDelete is one entry of pending-deletes.json. Only the id and the
// mark are stored: the family and its relations are whatever the files on
// disk say when the store opens.
type pendingDelete struct {
	ID        string    `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
	DeletedBy string    `json:"deleted_by,omitempty"`
}

// SoftDelete implements [store.SoftDeleteProvider].
func (s *FSStore) SoftDelete() store.SoftDeleter { return softDeleter{s: s} }

// SoftDelete on the transaction view skips txMu, which the open Tx holds.
func (t txStore) SoftDelete() store.SoftDeleter { return softDeleter{s: t.FSStore, inTx: true} }

type softDeleter struct {
	s    *FSStore
	inTx bool
}

func (d softDeleter) lock() func() {
	if d.inTx {
		return func() {}
	}
	return d.s.lockTx()
}

func (d softDeleter) MarkDeleted(_ context.Context, id, by string) (*store.DeleteResult, error) {
	defer d.lock()()
	s := d.s
	s.mu.Lock()
	defer s.mu.Unlock()

	family, related := s.stateFamily(id)
	if len(family) == 0 {
		return nil, store.ErrNotFound
	}
	states := make([]*entity.Entity, 0, len(family))
	for _, meta := range family {
		e, err := s.loadEntityMeta(meta)
		if err != nil {
			return nil, err
		}
		states = append(states, e)
	}
	rels := make([]*entity.Relation, 0, len(related))
	for _, rm := range related {
		rels = append(rels, loadRelationOrKey(s, rm))
	}

	fam := &fsMarked{at: time.Now().UTC(), by: by, entities: family, relations: map[string]relationMeta{}}
	for _, rm := range related {
		fam.relations[rm.key()] = rm
	}
	// The list goes to disk before the index changes: if it cannot be
	// written, the delete fails and nothing is hidden.
	next := copyMarked(s.marked)
	next[id] = fam
	if err := writePendingDeletes(s, next); err != nil {
		return nil, err
	}
	s.marked = next

	for i, meta := range family {
		key := stateKey(meta.ID, meta.Face)
		delete(s.entities, key)
		s.entityOrder = storeutil.SortedRemoveFunc(s.entityOrder, key, storeutil.CompareStateKeys)
		if meta.Face.IsDefault() {
			removeEntityFromCache(s.propCache, states[i])
		}
		s.notifyFaceDelete(meta.ID, meta.Face)
	}
	s.notifyLastFaceDelete(id)
	forgetRelations(s, related)
	s.emitFamilyDeleted(family, related)
	return &store.DeleteResult{DeletedEntities: states, DeletedRelations: rels}, nil
}

func (d softDeleter) Unmark(_ context.Context, id string) (*store.DeleteResult, error) {
	defer d.lock()()
	s := d.s
	s.mu.Lock()
	defer s.mu.Unlock()

	fam, ok := s.marked[id]
	// A family whose files were all removed outside the store has nothing
	// to bring back; the purge clears what is left of it.
	if !ok || len(fam.entities) == 0 {
		return nil, store.ErrNotFound
	}
	next := copyMarked(s.marked)
	delete(next, id)
	// An edge whose other end is still marked stays hidden and moves to that
	// family, so it comes back when that one does.
	var back []relationMeta
	for _, rm := range sortedRelMetas(fam.relations) {
		if other, stillMarked := next[relOtherEnd(rm, id)]; stillMarked {
			other.relations[rm.key()] = rm
			continue
		}
		back = append(back, rm)
	}
	if err := writePendingDeletes(s, next); err != nil {
		return nil, err
	}
	s.marked = next

	result := &store.DeleteResult{}
	for _, meta := range fam.entities {
		key := stateKey(meta.ID, meta.Face)
		s.entities[key] = meta
		s.entityOrder = storeutil.SortedInsertFunc(s.entityOrder, key, storeutil.CompareStateKeys)
		e, err := s.loadEntityMeta(meta)
		if err != nil {
			// The index is restored either way; a file that cannot be read
			// now reports its error on the next read, as any other would.
			slog.Warn("fsstore: restored entity could not be read", "entity", meta.ID, "error", err)
			continue
		}
		if meta.Face.IsDefault() {
			addEntityToCache(s.propCache, e)
		}
		s.notifyPut(e)
		result.DeletedEntities = append(result.DeletedEntities, e)
	}
	for _, rm := range back {
		key := rm.key()
		s.relations[key] = rm
		s.relationOrder = storeutil.SortedInsert(s.relationOrder, key)
		result.DeletedRelations = append(result.DeletedRelations, loadRelationOrKey(s, rm))
	}

	for _, meta := range fam.entities {
		s.emit(store.Event{Op: store.EventEntityCreated, EntityType: meta.Type, EntityID: meta.ID, Face: meta.Face})
	}
	for _, rm := range back {
		s.emit(store.Event{
			Op: store.EventRelationCreated, RelationType: rm.Type, From: rm.From, To: rm.To, Face: rm.FromFace,
		})
	}
	return result, nil
}

func (d softDeleter) ListMarked(_ context.Context) ([]store.MarkedEntity, error) {
	s := d.s
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]store.MarkedEntity, 0, len(s.marked))
	for id, fam := range s.marked {
		me := store.MarkedEntity{ID: id, DeletedAt: fam.at, DeletedBy: fam.by}
		for _, meta := range fam.entities {
			e, err := s.loadEntityMeta(meta)
			if err != nil {
				return nil, err
			}
			me.Entities = append(me.Entities, e)
		}
		out = append(out, me)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].DeletedAt.Equal(out[j].DeletedAt) {
			return out[i].DeletedAt.Before(out[j].DeletedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (d softDeleter) PurgeMarked(_ context.Context, id string) (*store.DeleteResult, error) {
	defer d.lock()()
	s := d.s
	s.mu.Lock()
	defer s.mu.Unlock()

	fam, ok := s.marked[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	next := copyMarked(s.marked)
	delete(next, id)

	// Hidden edges to id held by another marked family go too; they must not
	// come back pointing at an entity that no longer exists.
	doomed := sortedRelMetas(fam.relations)
	for _, other := range next {
		for key, rm := range other.relations {
			if rm.From == id || rm.To == id {
				doomed = append(doomed, rm)
				delete(other.relations, key)
			}
		}
	}

	result := &store.DeleteResult{}
	for _, meta := range fam.entities {
		if e, err := s.loadEntityMeta(meta); err == nil {
			result.DeletedEntities = append(result.DeletedEntities, e)
		}
	}
	for _, rm := range doomed {
		result.DeletedRelations = append(result.DeletedRelations, loadRelationOrKey(s, rm))
	}

	// Relation files first, then entity files, the same order a hard delete
	// uses. A failure leaves the mark in place so the next purge retries.
	for _, rm := range doomed {
		key := s.layout.relationFileKeyMeta(rm)
		if err := s.rooted.Remove(key); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("purge relation file %s: %w", rm.key(), err)
		}
		s.echoes.Forget(s.layout.absPath(key))
	}
	for _, meta := range fam.entities {
		key := s.layout.entityFileKey(meta.Type, stateKey(meta.ID, meta.Face))
		if err := s.rooted.Remove(key); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("purge entity file %s: %w", key, err)
		}
		s.echoes.Forget(s.layout.absPath(key))
	}
	if err := s.removeAttachmentDir(id); err != nil {
		slog.Warn("fsstore: purged entity's attachment directory could not be removed",
			"entity", id, "error", err)
	}
	if err := writePendingDeletes(s, next); err != nil {
		return nil, err
	}
	s.marked = next
	return result, nil
}

// applyPendingDeletes re-hides the families listed in pending-deletes.json
// when the store opens. An entry whose files are gone is dropped. Called from
// New, before the store is shared.
func applyPendingDeletes(s *FSStore) error {
	entries, err := readPendingDeletes(s)
	if err != nil || len(entries) == 0 {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].DeletedAt.Before(entries[j].DeletedAt) })
	s.marked = map[string]*fsMarked{}
	for _, pd := range entries {
		family, related := s.stateFamily(pd.ID)
		if len(family) == 0 {
			continue
		}
		fam := &fsMarked{at: pd.DeletedAt, by: pd.DeletedBy, entities: family, relations: map[string]relationMeta{}}
		for _, meta := range family {
			key := stateKey(meta.ID, meta.Face)
			delete(s.entities, key)
			s.entityOrder = storeutil.SortedRemoveFunc(s.entityOrder, key, storeutil.CompareStateKeys)
		}
		for _, rm := range related {
			fam.relations[rm.key()] = rm
		}
		forgetRelations(s, related)
		s.marked[pd.ID] = fam
	}
	// The cached property counts may include the hidden entities.
	return s.rebuildPropCache()
}

func readPendingDeletes(s *FSStore) ([]pendingDelete, error) {
	if s.cacheKey == "" {
		return nil, nil
	}
	data, err := s.rooted.ReadFile(path.Join(s.cacheKey, pendingDeletesFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("fsstore: read %s: %w", pendingDeletesFile, err)
	}
	var entries []pendingDelete
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("fsstore: parse %s: %w", pendingDeletesFile, err)
	}
	return entries, nil
}

// writePendingDeletes replaces the list with marked. With no cache
// directory configured, marks live in memory only.
func writePendingDeletes(s *FSStore, marked map[string]*fsMarked) error {
	if s.cacheKey == "" {
		return nil
	}
	entries := make([]pendingDelete, 0, len(marked))
	for id, fam := range marked {
		entries = append(entries, pendingDelete{ID: id, DeletedAt: fam.at, DeletedBy: fam.by})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	if err := s.rooted.MkdirAll(s.cacheKey, 0o755); err != nil {
		return err
	}
	return s.rooted.WriteFile(path.Join(s.cacheKey, pendingDeletesFile), data, 0o644)
}

// dropMarkedEdges removes the hidden edges that touch id, files included, for
// a hard delete or a rename of id. Otherwise a restore of the marked end would
// bring back an edge to an entity that is gone, or to a new entity that took
// its id. The file goes too, because a reopen rebuilds a family's hidden
// edges from the relation files on disk. Called under s.mu.
func dropMarkedEdges(s *FSStore, id string) error {
	for _, fam := range s.marked {
		for key, rm := range fam.relations {
			if rm.From != id && rm.To != id {
				continue
			}
			fileKey := s.layout.relationFileKeyMeta(rm)
			if err := s.rooted.Remove(fileKey); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove hidden relation file %s: %w", rm.key(), err)
			}
			s.echoes.Forget(s.layout.absPath(fileKey))
			delete(fam.relations, key)
		}
	}
	return nil
}

// markedFamilyOf returns the marked family holding id, if any.
func markedFamilyOf(s *FSStore, id string) (*fsMarked, bool) {
	fam, ok := s.marked[id]
	return fam, ok
}

// markedTaken reports whether a marked family holds id, case-folded like
// idTaken. except skips one family, for a rename that only changes casing.
func markedTaken(s *FSStore, id, except string) bool {
	folded := storeutil.FoldID(id)
	for markedID := range s.marked {
		if except != "" && storeutil.FoldID(markedID) == storeutil.FoldID(except) {
			continue
		}
		if storeutil.FoldID(markedID) == folded {
			return true
		}
	}
	return false
}

// revealedRelations returns the hidden relations of the entity ctx reveals
// (see [store.WithRevealed]) that satisfy match. Caller holds mu.
func revealedRelations(ctx context.Context, s *FSStore, match func(*entity.Relation) bool) []relationMeta {
	fam, ok := markedFamilyOf(s, store.RevealedID(ctx))
	if !ok {
		return nil
	}
	var out []relationMeta
	for _, rm := range sortedRelMetas(fam.relations) {
		r := entity.NewRelation(rm.From, rm.Type, rm.To)
		r.FromFace = rm.FromFace
		if match(r) {
			out = append(out, rm)
		}
	}
	return out
}

// markedEntityMetas and markedRelationMetas list what the marked families
// hold, so the persisted index keeps describing every file on disk.
func markedEntityMetas(s *FSStore) []entityMeta {
	var out []entityMeta
	for _, fam := range s.marked {
		out = append(out, fam.entities...)
	}
	return out
}

func markedRelationMetas(s *FSStore) []relationMeta {
	var out []relationMeta
	for _, fam := range s.marked {
		for _, rm := range fam.relations {
			out = append(out, rm)
		}
	}
	return out
}

// watchMarkedEntity keeps an external edit to a marked entity's file inside
// its family instead of letting it reappear. Reports whether it did.
func watchMarkedEntity(s *FSStore, meta entityMeta, removed bool) bool {
	fam, ok := markedFamilyOf(s, meta.ID)
	if !ok {
		return false
	}
	kept := fam.entities[:0]
	for _, m := range fam.entities {
		if m.Face != meta.Face {
			kept = append(kept, m)
		}
	}
	if !removed {
		kept = append(kept, meta)
	}
	fam.entities = kept
	return true
}

// watchMarkedRelation does the same for a relation file touching a marked
// entity.
func watchMarkedRelation(s *FSStore, rm relationMeta, removed bool) bool {
	fam, ok := markedFamilyOf(s, rm.From)
	if !ok {
		fam, ok = markedFamilyOf(s, rm.To)
	}
	if !ok {
		for _, other := range s.marked {
			if _, held := other.relations[rm.key()]; held {
				fam, ok = other, true
				break
			}
		}
	}
	if !ok {
		return false
	}
	if removed {
		delete(fam.relations, rm.key())
	} else {
		fam.relations[rm.key()] = rm
	}
	return true
}

func loadRelationOrKey(s *FSStore, rm relationMeta) *entity.Relation {
	r, err := s.loadRelationMeta(rm)
	if err != nil {
		r = entity.NewRelation(rm.From, rm.Type, rm.To)
		r.FromFace = rm.FromFace
	}
	return r
}

func relOtherEnd(rm relationMeta, id string) string {
	if rm.From == id {
		return rm.To
	}
	return rm.From
}

func sortedRelMetas(in map[string]relationMeta) []relationMeta {
	out := make([]relationMeta, 0, len(in))
	for _, rm := range in {
		out = append(out, rm)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key() < out[j].key() })
	return out
}

// copyMarked copies the map and each family's relation set, so a write can
// be prepared and only published once the list file is on disk.
func copyMarked(in map[string]*fsMarked) map[string]*fsMarked {
	out := make(map[string]*fsMarked, len(in)+1)
	for id, fam := range in {
		rels := make(map[string]relationMeta, len(fam.relations))
		maps.Copy(rels, fam.relations)
		out[id] = &fsMarked{at: fam.at, by: fam.by, entities: fam.entities, relations: rels}
	}
	return out
}
