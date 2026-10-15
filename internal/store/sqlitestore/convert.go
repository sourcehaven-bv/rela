package sqlitestore

import (
	"context"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

var _ store.FamilyReplacer = (*Store)(nil)

func (s *Store) ReplaceEntityFamily(ctx context.Context, id string, replacements []store.FamilyReplacement) error {
	if s.conn == nil {
		return s.Tx(ctx, func(tx store.Store) error {
			txCtx := store.ContextInTx(ctx)
			return tx.(store.FamilyReplacer).ReplaceEntityFamily(txCtx, id, replacements)
		})
	}
	if id == "" || len(replacements) == 0 {
		return fmt.Errorf("sqlitestore: replacement family requires an id and rows")
	}
	current := map[entity.Face]bool{}
	for row, err := range s.ListEntities(ctx, store.EntityQuery{IDs: []string{id}, Faces: store.AllFaces()}) {
		if err != nil {
			return err
		}
		current[row.Face] = true
	}
	if len(current) == 0 {
		return store.ErrNotFound
	}
	seenSource := make(map[entity.Face]bool, len(replacements))
	seenTarget := make(map[entity.Face]bool, len(replacements))
	typeName := ""
	for _, replacement := range replacements {
		e := replacement.Entity
		if e == nil || e.ID != id || e.Type == "" || !current[replacement.SourceFace] || seenSource[replacement.SourceFace] {
			return fmt.Errorf("sqlitestore: invalid replacement for source face %q", replacement.SourceFace)
		}
		if typeName == "" {
			typeName = e.Type
		} else if e.Type != typeName {
			return fmt.Errorf("sqlitestore: replacement family has mixed entity types")
		}
		if seenTarget[e.Face] {
			return fmt.Errorf("sqlitestore: replacement family maps multiple rows to face %q", e.Face)
		}
		seenSource[replacement.SourceFace] = true
		seenTarget[e.Face] = true
	}
	if len(seenSource) != len(current) {
		return fmt.Errorf("sqlitestore: replacement must account for every source face")
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
	events := make([]store.Event, 0, len(replacements))
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
		stored := e.Clone()
		stored.UpdatedAt = updated
		s.notifyFaceDelete(id, replacement.SourceFace)
		s.notifyPut(stored)
		ev := store.Event{Op: store.EventEntityUpdated, EntityID: id, EntityType: e.Type, Face: e.Face}
		events = append(events, ev)
	}
	for _, ev := range events {
		s.emit(ev)
	}
	return nil
}
