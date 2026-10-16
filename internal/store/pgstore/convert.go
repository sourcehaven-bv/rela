package pgstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
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
	if id == "" || len(replacements) == 0 {
		return errors.New("pgstore: replacement family requires an id and rows")
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
			return fmt.Errorf("pgstore: invalid replacement for source face %q", replacement.SourceFace)
		}
		if typeName == "" {
			typeName = e.Type
		} else if e.Type != typeName {
			return errors.New("pgstore: replacement family has mixed entity types")
		}
		if seenTarget[e.Face] {
			return fmt.Errorf("pgstore: replacement family maps multiple rows to face %q", e.Face)
		}
		seenSource[replacement.SourceFace] = true
		seenTarget[e.Face] = true
	}
	if len(seenSource) != len(current) {
		return errors.New("pgstore: replacement must account for every source face")
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
	events := make([]store.Event, 0, len(replacements))
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
		stored := e.Clone()
		stored.UpdatedAt = updatedAt
		notifyFaceDelete(s, id, replacement.SourceFace)
		s.notifyPut(stored)
		ev := store.Event{Op: store.EventEntityUpdated, EntityType: e.Type, EntityID: e.ID, Face: e.Face}
		s.notify(ctx, s.db, ev)
		events = append(events, ev)
	}
	s.emitAll(events)
	return nil
}
