package workspace

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/rename"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Rename performs an entity ID rename. It updates the entity file, all
// relation files referencing the old ID (in either direction), and the
// in-memory store mirror — best-effort: if a relation rename fails
// mid-way, earlier writes stay in place. Callers can re-run the command.
//
// If opts.DryRun is true, no changes are persisted; the returned
// Result describes what *would* change against the current snapshot.
func (w *Workspace) rename(entityType, oldID, newID string, opts rename.Options) (*rename.Result, error) {
	w.reloadMu.Lock()
	defer w.reloadMu.Unlock()

	if w.closed.Load() {
		return nil, fmt.Errorf("workspace is closed")
	}
	s := w.state.Load()
	if s == nil {
		return nil, fmt.Errorf("workspace not initialized")
	}

	st := w.Store()
	ent, incoming, outgoing, err := loadAndValidateRename(st, entityType, oldID, newID)
	if err != nil {
		return nil, err
	}

	result := buildRenameResult(entityType, oldID, newID, incoming, outgoing)

	if opts.DryRun {
		return result, nil
	}

	// Write the new entity under the new ID.
	newEntity := &entity.Entity{
		ID:         newID,
		Type:       ent.Type,
		Properties: ent.Properties,
		Content:    ent.Content,
	}
	if err := w.writeEntity(newEntity); err != nil {
		return nil, fmt.Errorf("write new entity %s: %w", newID, err)
	}

	// Write the updated relations (with the new ID substituted).
	if err := writeRenamedRelations(w, oldID, newID, incoming, outgoing); err != nil {
		return nil, err
	}

	// Delete the old relation files first, then the entity. Self-
	// referential relations are deleted exactly once via the outgoing
	// loop; the incoming loop skips them.
	for _, rel := range outgoing {
		if err := w.deleteRelationStore(oldID, rel.Type, rel.To); err != nil {
			return nil, fmt.Errorf("delete old outgoing relation %s--%s-->%s: %w",
				oldID, rel.Type, rel.To, err)
		}
	}
	for _, rel := range incoming {
		if rel.From == oldID { // self-referential, already deleted above
			continue
		}
		if err := w.deleteRelationStore(rel.From, rel.Type, oldID); err != nil {
			return nil, fmt.Errorf("delete old incoming relation %s--%s-->%s: %w",
				rel.From, rel.Type, oldID, err)
		}
	}
	if err := w.deleteEntityStore(oldID); err != nil {
		return nil, fmt.Errorf("delete old entity %s: %w", oldID, err)
	}

	return result, nil
}

// loadAndValidateRename combines existence + type checks and incident-edge
// collection needed by the rename flow, all against a single store.
func loadAndValidateRename(
	st store.Store, entityType, oldID, newID string,
) (ent *entity.Entity, incoming, outgoing []*entity.Relation, err error) {
	if vErr := validateRename(st, oldID, newID); vErr != nil {
		return nil, nil, nil, vErr
	}
	ctx := context.Background()
	ent, err = st.GetEntity(ctx, oldID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("entity not found: %s", oldID)
	}
	if ent.Type != entityType {
		return nil, nil, nil, fmt.Errorf("entity %s has type %s, not %s", oldID, ent.Type, entityType)
	}

	incoming = relationsForRename(st, oldID, store.DirectionIncoming)
	outgoing = relationsForRename(st, oldID, store.DirectionOutgoing)
	return ent, incoming, outgoing, nil
}

// relationsForRename collects incident relations in the given direction.
// Errors from the store iterator are swallowed: a partial result is
// preferable to failing the rename outright.
func relationsForRename(st store.Store, id string, dir store.Direction) []*entity.Relation {
	out := make([]*entity.Relation, 0)
	for r, err := range st.ListRelations(context.Background(), store.RelationQuery{
		EntityID:  id,
		Direction: dir,
	}) {
		if err != nil {
			continue
		}
		out = append(out, r)
	}
	return out
}

// validateRename checks rename preconditions against the given store.
func validateRename(st store.Store, oldID, newID string) error {
	ctx := context.Background()
	if _, err := st.GetEntity(ctx, oldID); err != nil {
		return fmt.Errorf("entity not found: %s", oldID)
	}
	if _, err := st.GetEntity(ctx, newID); err == nil {
		return fmt.Errorf("entity with ID %s already exists", newID)
	}
	if err := entity.ValidateID(newID); err != nil {
		return fmt.Errorf("invalid new ID: %w", err)
	}
	return nil
}

// buildRenameResult assembles the descriptive Result returned by Rename.
func buildRenameResult(
	entityType, oldID, newID string,
	incoming, outgoing []*entity.Relation,
) *rename.Result {
	result := &rename.Result{
		OldID:            oldID,
		NewID:            newID,
		EntityType:       entityType,
		RelationsUpdated: make([]rename.RelationRef, 0, len(incoming)+len(outgoing)),
	}

	for _, rel := range outgoing {
		result.RelationsUpdated = append(result.RelationsUpdated, rename.RelationRef{
			From: newID,
			Type: rel.Type,
			To:   rel.To,
		})
	}
	for _, rel := range incoming {
		result.RelationsUpdated = append(result.RelationsUpdated, rename.RelationRef{
			From: rel.From,
			Type: rel.Type,
			To:   newID,
		})
	}

	return result
}

// writeRenamedRelations writes the relation files with the new ID
// substituted for the old one (in either direction). Self-referential
// relations are written exactly once.
func writeRenamedRelations(w *Workspace, oldID, newID string, incoming, outgoing []*entity.Relation) error {
	// Outgoing: from = oldID becomes from = newID. If the relation is
	// self-referential (to == oldID), the new to is also newID.
	for _, rel := range outgoing {
		newTo := rel.To
		if rel.To == oldID {
			newTo = newID
		}
		newRel := &entity.Relation{
			From:       newID,
			Type:       rel.Type,
			To:         newTo,
			Properties: rel.Properties,
			Content:    rel.Content,
		}
		if err := w.writeRelation(newRel); err != nil {
			return fmt.Errorf("write relation %s--%s-->%s: %w", newID, rel.Type, newTo, err)
		}
	}

	// Incoming: to = oldID becomes to = newID. Skip self-referential
	// relations because they were already handled in the outgoing loop.
	for _, rel := range incoming {
		if rel.From == oldID {
			continue
		}
		newRel := &entity.Relation{
			From:       rel.From,
			Type:       rel.Type,
			To:         newID,
			Properties: rel.Properties,
			Content:    rel.Content,
		}
		if err := w.writeRelation(newRel); err != nil {
			return fmt.Errorf("write relation %s--%s-->%s: %w", rel.From, rel.Type, newID, err)
		}
	}

	return nil
}
