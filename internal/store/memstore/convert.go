package memstore

import (
	"context"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

var _ store.FamilyReplacer = (*MemStore)(nil)

func replaceEntityFamily(_ context.Context, m *MemStore, id string, replacements []store.FamilyReplacement) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var oldKeys []string
	var current []entity.Face
	for key, row := range m.entities {
		if row.ID == id {
			oldKeys = append(oldKeys, key)
			current = append(current, row.Face)
		}
	}
	emptied, err := storeutil.ValidateFamilyReplacement(id, current, replacements)
	if err != nil {
		return err
	}
	oldType := ""
	for _, key := range oldKeys {
		oldType = m.entities[key].Type
		m.notifyFaceDelete(id, m.entities[key].Face)
		delete(m.entities, key)
		m.entityOrder = entityRemove(m.entityOrder, key)
	}
	// Bare-id observers index the family under one id; drop it before the
	// rows come back under the new type.
	m.notifyLastFaceDelete(id)
	now := time.Now()
	for _, r := range replacements {
		row := r.Entity.Clone()
		row.UpdatedAt = now
		key := entity.FormatStateRef(row.ID, row.Face)
		m.entities[key] = row
		m.entityOrder = entityInsert(m.entityOrder, key)
		m.notifyPut(row)
		m.emit(store.Event{Op: store.EventEntityUpdated, EntityType: row.Type, EntityID: row.ID, Face: row.Face})
	}
	for _, face := range emptied {
		m.emit(store.Event{Op: store.EventEntityDeleted, EntityType: oldType, EntityID: id, Face: face})
	}
	return nil
}
