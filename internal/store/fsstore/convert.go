package fsstore

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

var _ store.FamilyReplacer = (*FSStore)(nil)

func (s *FSStore) replaceEntityFamily(_ context.Context, id string, replacements []store.FamilyReplacement) error {
	if err := storeutil.ValidateID(id); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	family, _ := s.stateFamily(id)
	if len(family) == 0 {
		return store.ErrNotFound
	}
	if len(replacements) == 0 {
		return fmt.Errorf("fsstore: replacement family must not be empty")
	}
	seen := make(map[entity.Face]bool, len(replacements))
	seenSource := make(map[entity.Face]bool, len(replacements))
	oldFaces := make(map[entity.Face]bool, len(family))
	for _, meta := range family {
		oldFaces[meta.Face] = true
	}
	typeName := ""
	newRows := make([]*entity.Entity, 0, len(replacements))
	for _, replacement := range replacements {
		target := replacement.Entity
		if target == nil || target.ID != id || target.Type == "" {
			return fmt.Errorf("fsstore: replacement rows must use id %q and a declared type", id)
		}
		if !oldFaces[replacement.SourceFace] || seenSource[replacement.SourceFace] {
			return fmt.Errorf("fsstore: replacement source face %q is missing or repeated", replacement.SourceFace)
		}
		seenSource[replacement.SourceFace] = true
		if typeName == "" {
			typeName = target.Type
		} else if target.Type != typeName {
			return fmt.Errorf("fsstore: replacement family has mixed entity types")
		}
		if _, ok := s.layout.schemas[target.Type]; !ok {
			return fmt.Errorf("fsstore: replacement type %q is not declared", target.Type)
		}
		if seen[target.Face] {
			return fmt.Errorf("fsstore: replacement family maps multiple rows to face %q", target.Face)
		}
		seen[target.Face] = true
		if err := storeutil.ValidateProperties(target.Properties); err != nil {
			return err
		}
		newRows = append(newRows, target.Clone())
	}
	if len(seenSource) != len(oldFaces) {
		return fmt.Errorf("fsstore: replacement must account for every source face")
	}

	// Check every destination before writing any of them. Conversion targets
	// use a different type directory, so an existing file there is a real
	// collision rather than one of the source rows being replaced.
	for _, row := range newRows {
		key := s.layout.entityFileKey(row.Type, stateKey(row.ID, row.Face))
		if _, err := s.rooted.Stat(key); err == nil {
			return store.ErrConflict
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("fsstore: inspect conversion target %s: %w", key, err)
		}
	}

	now := time.Now()
	for _, row := range newRows {
		row.UpdatedAt = now
		if err := s.writeEntity(row); err != nil {
			return fmt.Errorf("fsstore: write converted row %s: %w", entity.FormatStateRef(id, row.Face), err)
		}
	}
	for _, meta := range family {
		oldKey := s.layout.entityFileKey(meta.Type, stateKey(meta.ID, meta.Face))
		if err := s.rooted.Remove(oldKey); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("fsstore: remove converted source row %s: %w", entity.FormatStateRef(id, meta.Face), err)
		}
		s.echoes.Forget(s.layout.absPath(oldKey))
		movedKey := stateKey(meta.ID, meta.Face)
		delete(s.entities, movedKey)
		s.entityOrder = storeutil.SortedRemoveFunc(s.entityOrder, movedKey, storeutil.CompareStateKeys)
		s.notifyFaceDelete(id, meta.Face)
	}
	for _, row := range newRows {
		key := stateKey(row.ID, row.Face)
		s.entities[key] = entityMeta{ID: row.ID, Type: row.Type, Face: row.Face}
		s.entityOrder = storeutil.SortedInsertFunc(s.entityOrder, key, storeutil.CompareStateKeys)
		s.notifyPut(row)
		s.emit(store.Event{Op: store.EventEntityUpdated, EntityType: row.Type, EntityID: row.ID, Face: row.Face})
	}
	return nil
}
