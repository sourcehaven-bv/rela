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

// replaceEntityFamily writes the converted rows, then removes the source
// files. fsstore's Tx cannot roll back, so this function undoes its own
// partial work: on any failure it removes the target files it wrote and
// rewrites the source files it removed, and the index is only touched once
// every file operation has succeeded. Disk never keeps both types for one id.
func replaceEntityFamily(_ context.Context, s *FSStore, id string, replacements []store.FamilyReplacement) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	family, _ := s.stateFamily(id)
	current := make([]entity.Face, 0, len(family))
	for _, meta := range family {
		current = append(current, meta.Face)
	}
	emptied, err := storeutil.ValidateFamilyReplacement(id, current, replacements)
	if err != nil {
		return err
	}
	newRows := make([]*entity.Entity, 0, len(replacements))
	for _, r := range replacements {
		if !s.layout.declared(r.Entity.Type) {
			return fmt.Errorf("%w: type %q is not declared", store.ErrInvalidFamilyReplacement, r.Entity.Type)
		}
		newRows = append(newRows, r.Entity.Clone())
	}
	// A conversion target lives in another type directory, so an existing
	// file there is a real collision, not one of the rows being replaced.
	for _, row := range newRows {
		key := s.layout.entityFileKey(row.Type, stateKey(row.ID, row.Face))
		if _, err := s.rooted.Stat(key); err == nil {
			return store.ErrConflict
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("fsstore: inspect conversion target %s: %w", key, err)
		}
	}
	oldRows := make([]*entity.Entity, 0, len(family))
	for _, meta := range family {
		e, err := s.loadEntityMeta(meta)
		if err != nil {
			return err
		}
		oldRows = append(oldRows, e)
	}

	if err := swapFamilyFiles(s, oldRows, newRows); err != nil {
		return err
	}

	oldType := ""
	for _, meta := range family {
		oldType = meta.Type
		s.echoes.Forget(s.layout.absPath(s.layout.entityFileKey(meta.Type, stateKey(meta.ID, meta.Face))))
		movedKey := stateKey(meta.ID, meta.Face)
		delete(s.entities, movedKey)
		s.entityOrder = storeutil.SortedRemoveFunc(s.entityOrder, movedKey, storeutil.CompareStateKeys)
		s.notifyFaceDelete(id, meta.Face)
	}
	// Bare-id observers index the family under one id; drop it before the
	// rows come back under the new type.
	s.notifyLastFaceDelete(id)
	for _, row := range newRows {
		key := stateKey(row.ID, row.Face)
		s.entities[key] = entityMeta{ID: row.ID, Type: row.Type, Face: row.Face}
		s.entityOrder = storeutil.SortedInsertFunc(s.entityOrder, key, storeutil.CompareStateKeys)
		s.notifyPut(row)
		s.emit(store.Event{Op: store.EventEntityUpdated, EntityType: row.Type, EntityID: row.ID, Face: row.Face})
	}
	for _, face := range emptied {
		s.emit(store.Event{Op: store.EventEntityDeleted, EntityType: oldType, EntityID: id, Face: face})
	}
	return nil
}

// swapFamilyFiles writes newRows and removes oldRows' files. On failure it
// restores the starting disk state as far as the filesystem allows and
// returns the original error joined with any undo failure.
func swapFamilyFiles(s *FSStore, oldRows, newRows []*entity.Entity) error {
	var written, removed []*entity.Entity
	undo := func(cause error) error {
		errs := []error{cause}
		for _, row := range written {
			key := s.layout.entityFileKey(row.Type, stateKey(row.ID, row.Face))
			if err := s.rooted.Remove(key); err != nil && !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, fmt.Errorf("fsstore: undo converted row %s: %w", key, err))
			}
		}
		for _, row := range removed {
			if err := s.writeEntity(row); err != nil {
				errs = append(errs, fmt.Errorf("fsstore: restore source row %s: %w",
					entity.FormatStateRef(row.ID, row.Face), err))
			}
		}
		return errors.Join(errs...)
	}
	now := time.Now()
	for _, row := range newRows {
		row.UpdatedAt = now
		if err := s.writeEntity(row); err != nil {
			return undo(fmt.Errorf("fsstore: write converted row %s: %w", entity.FormatStateRef(row.ID, row.Face), err))
		}
		written = append(written, row)
	}
	for _, row := range oldRows {
		key := s.layout.entityFileKey(row.Type, stateKey(row.ID, row.Face))
		if err := s.rooted.Remove(key); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return undo(fmt.Errorf("fsstore: remove converted source row %s: %w",
				entity.FormatStateRef(row.ID, row.Face), err))
		}
		removed = append(removed, row)
	}
	return nil
}
