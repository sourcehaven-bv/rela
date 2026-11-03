package pgstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

var _ store.FamilyReplacer = (*Store)(nil)

// ReplaceEntityFamily implements store.FamilyReplacer. The temporary face
// coordinates allow swaps such as draft→approved and approved→draft without
// violating the family primary key halfway through the transaction.
func (s *Store) ReplaceEntityFamily(ctx context.Context, id string, replacements []store.FamilyReplacement) error {
	if s.txPending == nil {
		return s.Tx(ctx, func(tx store.Store) error {
			txCtx := store.ContextInTx(ctx)
			replacer, ok := tx.(store.FamilyReplacer)
			if !ok {
				return errors.New("pgstore: transaction cannot replace entity families")
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
		if _, err := s.db.Exec(ctx,
			`UPDATE entities SET face = $3, seq = nextval('rela_seq') WHERE id = $1 AND face = $2`,
			id, string(replacement.SourceFace), string(temporary)); err != nil {
			return err
		}
	}

	editorUser, editorTool := store.AttributionColumns(ctx)
	origin := originValues(ctx)
	stored := make([]*entity.Entity, 0, len(replacements))
	events := make([]store.Event, 0, len(replacements)+len(emptied))
	for i, replacement := range replacements {
		e := replacement.Entity
		props, err := marshalProps(e.Properties)
		if err != nil {
			return err
		}
		mentionTargets, mentionHash := mentionColumns(e.Content)
		temporary := entity.Face(fmt.Sprintf("\x01convert-%d", i))
		const update = `UPDATE entities
			SET face = $3, type = $4, properties = $5, content = $6,
			    search_text = $7, updated_at = now(), seq = nextval('rela_seq'),
			    last_edited_by_user = $8, last_edited_by_tool = $9,
			    origin_kind = $10, origin_source = $11, origin_source_face = $12,
			    origin_source_type = $13, origin_definition = $14,
			    mention_targets = $15, mentions_hash = $16
			WHERE id = $1 AND face = $2 RETURNING updated_at`
		var updatedAt time.Time
		if err := s.db.QueryRow(ctx, update, id, string(temporary), string(e.Face), e.Type, props,
			e.Content, entitySearchText(e), editorUser, editorTool,
			origin.kind, origin.source, origin.sourceFace, origin.sourceType, origin.definition,
			mentionTargets, mentionHash).Scan(&updatedAt); err != nil {
			return s.mapConflict(err)
		}
		row := e.Clone()
		row.UpdatedAt = updatedAt
		stored = append(stored, row)
		events = append(events, store.Event{
			Op: store.EventEntityUpdated, EntityType: e.Type, EntityID: e.ID, Face: e.Face,
		})
	}
	// A source face no target occupies is gone: tombstone it like any
	// face delete so other processes' catch-up drops the old coordinate.
	for _, face := range emptied {
		events = append(events, store.Event{
			Op: store.EventEntityDeleted, EntityType: oldType, EntityID: id, Face: face,
		})
	}
	if err := s.writeTombstonesForEvents(ctx, s.db, events); err != nil {
		return err
	}
	for _, ev := range events {
		s.notify(ctx, s.db, ev)
	}
	for _, face := range current {
		notifyFaceDelete(s, id, face)
	}
	// Bare-id observers index the family under one id; drop it before the
	// rows come back under the new type.
	notifyLastFaceDelete(s, id)
	for _, row := range stored {
		s.notifyPut(row)
	}
	s.emitAll(events)
	return nil
}
