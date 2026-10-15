package memstore

import (
	"context"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

var _ store.FamilyReplacer = (*MemStore)(nil)

func (m *MemStore) replaceEntityFamily(_ context.Context, id string, replacements []store.FamilyReplacement) error {
	if err := validateID(id); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	var oldKeys []string
	for key, current := range m.entities {
		if current.ID == id {
			oldKeys = append(oldKeys, key)
		}
	}
	if len(oldKeys) == 0 {
		return store.ErrNotFound
	}
	if len(replacements) == 0 {
		return fmt.Errorf("memstore: replacement family must not be empty")
	}
	seen := make(map[entity.Face]bool, len(replacements))
	seenSource := make(map[entity.Face]bool, len(replacements))
	oldFaces := make(map[entity.Face]bool, len(oldKeys))
	for _, key := range oldKeys {
		oldFaces[m.entities[key].Face] = true
	}
	newRows := make([]*entity.Entity, 0, len(replacements))
	targetType := ""
	for _, replacement := range replacements {
		target := replacement.Entity
		if target == nil || target.ID != id || target.Type == "" {
			return fmt.Errorf("memstore: replacement rows must use id %q and a declared type", id)
		}
		if !oldFaces[replacement.SourceFace] || seenSource[replacement.SourceFace] {
			return fmt.Errorf("memstore: replacement source face %q is missing or repeated", replacement.SourceFace)
		}
		seenSource[replacement.SourceFace] = true
		if targetType == "" {
			targetType = target.Type
		} else if target.Type != targetType {
			return fmt.Errorf("memstore: replacement family has mixed types %q and %q", targetType, target.Type)
		}
		if seen[target.Face] {
			return fmt.Errorf("memstore: replacement family maps multiple rows to face %q", target.Face)
		}
		seen[target.Face] = true
		if err := storeutil.ValidateProperties(target.Properties); err != nil {
			return err
		}
		newRows = append(newRows, target.Clone())
	}
	if len(seenSource) != len(oldFaces) {
		return fmt.Errorf("memstore: replacement must account for every source face")
	}

	for _, key := range oldKeys {
		m.notifyFaceDelete(id, m.entities[key].Face)
		delete(m.entities, key)
		m.entityOrder = entityRemove(m.entityOrder, key)
	}
	now := time.Now()
	for _, row := range newRows {
		row.UpdatedAt = now
		key := entity.FormatStateRef(row.ID, row.Face)
		m.entities[key] = row
		m.entityOrder = entityInsert(m.entityOrder, key)
	}
	// Replace the indexed family as a whole: search and backlink observers
	// must not retain old face or type keys after a conversion.
	m.notifyLastFaceDelete(id)
	for _, row := range newRows {
		m.notifyPut(row)
		m.emit(store.Event{Op: store.EventEntityUpdated, EntityType: row.Type, EntityID: row.ID, Face: row.Face})
	}
	return nil
}
