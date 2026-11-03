package sqlitestore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

var _ store.FamilyReplacer = (*Store)(nil)

// ReplaceEntityFamily implements store.FamilyReplacer. Like pgstore it parks
// every source row on a temporary face first, so a face swap never collides
// with the family primary key mid-transaction.
func (s *Store) ReplaceEntityFamily(ctx context.Context, id string, replacements []store.FamilyReplacement) error {
	if s.conn == nil {
		return s.Tx(ctx, func(tx store.Store) error {
			txCtx := store.ContextInTx(ctx)
			replacer, ok := tx.(store.FamilyReplacer)
			if !ok {
				return errors.New("sqlitestore: transaction cannot replace entity families")
			}
			return replacer.ReplaceEntityFamily(
				txCtx, id, replacements,
			)
		})
	}
	var current []entity.Face
	oldType := ""
	for row, err := range s.ListEntities(ctx, store.EntityQuery{IDs: []string{id}, Faces: store.AllFaces()}) {
		if err != nil {
			return err
		}
		if row.ID == id {
			current = append(current, row.Face)
			oldType = row.Type
		}
	}
	emptied, err := storeutil.ValidateFamilyReplacement(id, current, replacements)
	if err != nil {
		return err
	}

	for i, replacement := range replacements {
		temporary := entity.Face(fmt.Sprintf("\x01convert-%d", i))
		if _, err := s.write(ctx, `UPDATE entities SET face = ? WHERE id = ? AND face = ?`,
			string(temporary), id, string(replacement.SourceFace)); err != nil {
			return err
		}
	}

	editorUser, editorTool := store.AttributionColumns(ctx)
	origin := originColumns(store.OriginFrom(ctx))
	stored := make([]*entity.Entity, 0, len(replacements))
	events := make([]store.Event, 0, len(replacements)+len(emptied))
	for i, replacement := range replacements {
		e := replacement.Entity
		props, err := marshalProps(e.Properties)
		if err != nil {
			return err
		}
		temporary := entity.Face(fmt.Sprintf("\x01convert-%d", i))
		updated := time.Now()
		res, err := s.write(ctx, `UPDATE entities SET face = ?, type = ?, properties = ?, content = ?, updated_at = ?,
				last_edited_by_user = ?, last_edited_by_tool = ?, origin_kind = ?, origin_source = ?,
				origin_source_face = ?, origin_source_type = ?, origin_definition = ?, mention_targets = ?
				WHERE id = ? AND face = ?`,
			string(e.Face), e.Type, props, e.Content, sqlitedb.FormatTime(updated),
			editorUser, editorTool, origin.kind, origin.source, origin.sourceFace, origin.sourceType,
			origin.definition, mentionTargetsJSON(e.Content), id, string(temporary))
		if err != nil {
			if isUniqueViolation(err) {
				return store.ErrConflict
			}
			return fmt.Errorf("sqlitestore: replace family row %s: %w", entity.FormatStateRef(id, e.Face), err)
		}
		if affected, err := res.RowsAffected(); err != nil {
			return err
		} else if affected == 0 {
			return store.ErrNotFound
		}
		row := e.Clone()
		row.UpdatedAt = updated
		stored = append(stored, row)
		events = append(events, store.Event{
			Op: store.EventEntityUpdated, EntityID: id, EntityType: e.Type, Face: e.Face,
		})
	}
	for _, face := range emptied {
		events = append(events, store.Event{
			Op: store.EventEntityDeleted, EntityID: id, EntityType: oldType, Face: face,
		})
	}
	for _, face := range current {
		s.notifyFaceDelete(id, face)
	}
	// Bare-id observers index the family under one id; drop it before the
	// rows come back under the new type.
	s.notifyLastFaceDelete(id)
	for _, row := range stored {
		s.notifyPut(row)
	}
	for _, ev := range events {
		s.emit(ev)
	}
	return nil
}
