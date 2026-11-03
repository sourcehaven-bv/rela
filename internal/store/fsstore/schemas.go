package fsstore

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"path"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// SetSchemas replaces the entity types the store declares, for a schema
// reload against an open store. It validates and normalizes schemas the
// way [New] does, then publishes them, so the index matches what a reopen
// with the new schemas would find:
//
//   - The files in the directory of a newly declared type join the index.
//     Each one is announced the way the watcher announces a new file:
//     observers get it, subscribers get an [store.EventEntityCreated], and
//     the mention index records its links. A file whose id belongs to a
//     soft-deleted family, or that is already indexed under another type,
//     is skipped with a warning.
//   - The entries of a type the schemas no longer declare leave the index,
//     announced as deletes. Its files stay on disk. Its soft-deleted
//     families stay marked, so declaring the type again does not bring
//     them back; until then the soft-delete operations skip them.
//
// A type whose plural changes while the store still holds entities of it
// keeps its old plural, because those entries point into the old
// directory. Everything else is applied, and the returned error names
// each such type. A type with no entities moves at once. If keeping an
// old plural would make two types share a directory, or a directory
// cannot be read, nothing is applied.
//
// It takes the same locks as a write, so it never runs between the type
// check and the file write of a concurrent write.
func (s *FSStore) SetSchemas(schemas map[string]store.EntityTypeSchema) error {
	declared, err := normalizeSchemas(schemas)
	if err != nil {
		return err
	}
	defer s.lockTx()()
	s.mu.Lock()
	defer s.mu.Unlock()

	added, refused := s.planSchemas(declared)
	if len(refused) > 0 {
		if _, err := normalizeSchemas(declared); err != nil {
			return errors.Join(append(refused, err)...)
		}
	}

	// Read every new directory before publishing, so a read error leaves
	// the store on its previous schemas.
	var found []entityMeta
	for _, typ := range added {
		entries, err := scanTypeDir(s.rooted, s.layout.entitiesKey, declared[typ].Plural, typ)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("fsstore: scan entity type %q: %w", typ, err)
		}
		found = append(found, entries...)
	}

	s.layout.schemas.Store(&schemaSet{declared: declared})
	dropUndeclared(s, declared)
	for _, meta := range found {
		indexAnnounced(s, meta)
	}
	return errors.Join(refused...)
}

// planSchemas returns the types of declared whose directory must be
// scanned, sorted. A type whose plural changed while the store holds
// entities of it, live or soft-deleted, gets its old plural back in
// declared and an error in refused. Caller holds s.mu.
func (s *FSStore) planSchemas(declared map[string]store.EntityTypeSchema) (added []string, refused []error) {
	prev := s.layout.schemas.Load().declared
	held := map[string]bool{}
	for _, meta := range s.entities {
		held[meta.Type] = true
	}
	for _, meta := range markedEntityMetas(s) {
		held[meta.Type] = true
	}
	for _, typ := range slices.Sorted(maps.Keys(declared)) {
		old, ok := prev[typ]
		next := declared[typ]
		switch {
		case !ok:
			added = append(added, typ)
		case old.Plural != next.Plural && held[typ]:
			refused = append(refused, fmt.Errorf(
				"fsstore: type %q: plural changed from %q to %q while entities exist; rename %s to %s and restart",
				typ, old.Plural, next.Plural,
				path.Join(s.layout.entitiesKey, old.Plural), path.Join(s.layout.entitiesKey, next.Plural)))
			next.Plural = old.Plural
			declared[typ] = next
		case old.Plural != next.Plural:
			added = append(added, typ)
		}
	}
	return added, refused
}

// dropUndeclared removes the entries of types that declared does not hold
// from the index. Soft-deleted families keep them (see
// [FSStore.SetSchemas]). Caller holds s.mu.
func dropUndeclared(s *FSStore, declared map[string]store.EntityTypeSchema) {
	for _, key := range slices.Clone(s.entityOrder) {
		meta := s.entities[key]
		if _, ok := declared[meta.Type]; ok {
			continue
		}
		delete(s.entities, key)
		s.entityOrder = storeutil.SortedRemoveFunc(s.entityOrder, key, storeutil.CompareStateKeys)
		s.notifyFaceDelete(meta.ID, meta.Face)
		// Per file, as the watcher does: a bare-id observer hears only the
		// implicit face. Both production observers are face-aware.
		if meta.Face.IsImplicit() {
			s.notifyLastFaceDelete(meta.ID)
		}
		s.emit(store.Event{Op: store.EventEntityDeleted, EntityType: meta.Type, EntityID: meta.ID, Face: meta.Face})
	}
}

// indexAnnounced adds one entry found by SetSchemas to the index and
// announces it as the watcher announces a new file. Caller holds s.mu.
func indexAnnounced(s *FSStore, meta entityMeta) {
	key := stateKey(meta.ID, meta.Face)
	if existing, ok := s.entities[key]; ok {
		slog.Warn("fsstore: skipping entity file whose id is already indexed under another type",
			"id", key, "type", meta.Type, "indexed_type", existing.Type)
		return
	}
	if markedTaken(s, meta.ID, "") {
		slog.Warn("fsstore: skipping entity file of a soft-deleted entity", "id", key, "type", meta.Type)
		return
	}
	s.entities[key] = meta
	s.entityOrder = storeutil.SortedInsertFunc(s.entityOrder, key, storeutil.CompareStateKeys)
	if e, err := s.loadEntityMeta(meta); err != nil {
		// Indexed anyway, as the open scan would: reads report the error.
		slog.Warn("fsstore: cannot read newly declared entity file", "id", key, "error", err)
	} else {
		noteMentions(s, e)
		s.notifyPut(e)
	}
	s.emit(store.Event{Op: store.EventEntityCreated, EntityType: meta.Type, EntityID: meta.ID, Face: meta.Face})
}
