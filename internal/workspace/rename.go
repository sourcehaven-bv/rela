package workspace

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/rename"
	"github.com/Sourcehaven-BV/rela/internal/repository"
)

// Rename performs an entity ID rename. It updates the entity file, all
// relation files referencing the old ID (in either direction), and the
// in-memory graph + search index — all atomically via WithTx. On any
// failure, no on-disk or in-memory state is modified.
//
// If opts.DryRun is true, no changes are persisted; the returned Result
// describes what *would* change against the current snapshot.
//
// Inherits the atomicity caveats of Workspace.WithTx — see WithTx's
// docs for the rollback hazards in repository.Transaction's phase-2
// deletes.
//
// This is the canonical replacement for the legacy `rename.Rename` free
// function. The orchestration was moved into the workspace package so it
// could use ws.WithTx; the rename package now contains only the public
// types (Options, Result, RelationRef).
func (w *Workspace) rename(entityType, oldID, newID string, opts rename.Options) (*rename.Result, error) {
	if opts.DryRun {
		return w.renameDryRun(entityType, oldID, newID)
	}

	var result *rename.Result
	err := w.WithTx(func(tx *Tx) error {
		// Validate against the in-tx snapshot. Doing this inside the
		// closure (rather than against w.state.Load() before WithTx)
		// guarantees that validation, the entity payload we write,
		// and the relation lists we walk all come from the same
		// epoch — even if a concurrent reload landed between the
		// caller's invocation and the moment WithTx took reloadMu.
		entity, incoming, outgoing, err := loadAndValidateRename(tx, entityType, oldID, newID)
		if err != nil {
			return err
		}

		// Write the new entity under the new ID.
		newEntity := &model.Entity{
			ID:         newID,
			Type:       entity.Type,
			Properties: entity.Properties,
			Content:    entity.Content,
		}
		if err := tx.WriteEntity(newEntity); err != nil {
			return fmt.Errorf("write new entity %s: %w", newID, err)
		}

		// Write the updated relations (with the new ID substituted).
		if err := writeRenamedRelations(tx, oldID, newID, incoming, outgoing); err != nil {
			return err
		}

		// Delete the old entity and its old relation files. Self-
		// referential relations are deleted exactly once via the
		// outgoing loop; the incoming loop skips them.
		if err := tx.DeleteEntity(entity.Type, oldID); err != nil {
			return fmt.Errorf("delete old entity %s: %w", oldID, err)
		}
		for _, rel := range outgoing {
			if err := tx.DeleteRelation(oldID, rel.Type, rel.To); err != nil {
				return fmt.Errorf("delete old outgoing relation %s--%s-->%s: %w",
					oldID, rel.Type, rel.To, err)
			}
		}
		for _, rel := range incoming {
			if rel.From == oldID { // self-referential, already deleted above
				continue
			}
			if err := tx.DeleteRelation(rel.From, rel.Type, oldID); err != nil {
				return fmt.Errorf("delete old incoming relation %s--%s-->%s: %w",
					rel.From, rel.Type, oldID, err)
			}
		}

		// Build the result from the same in-tx snapshot we just
		// operated on, so it accurately reflects what was committed.
		result = buildRenameResult(w.repo, tx.base.meta, entityType, oldID, newID, incoming, outgoing)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// renameDryRun runs the validation and result-building phase against
// the workspace's current snapshot without acquiring reloadMu. The
// returned result is a "what would change right now" view; a concurrent
// commit may invalidate it before the caller can act on it. Acceptable
// because dry-run callers do not promise consistency.
func (w *Workspace) renameDryRun(entityType, oldID, newID string) (*rename.Result, error) {
	s := w.state.Load()
	if s == nil {
		return nil, fmt.Errorf("workspace not initialized")
	}
	g := s.graph

	if err := validateRename(g, oldID, newID); err != nil {
		return nil, err
	}
	entity, ok := g.GetNode(oldID)
	if !ok {
		return nil, fmt.Errorf("entity not found: %s", oldID)
	}
	if entity.Type != entityType {
		return nil, fmt.Errorf("entity %s has type %s, not %s", oldID, entity.Type, entityType)
	}

	incoming := g.IncomingEdges(oldID)
	outgoing := g.OutgoingEdges(oldID)
	return buildRenameResult(w.repo, s.meta, entityType, oldID, newID, incoming, outgoing), nil
}

// loadAndValidateRename combines the existence check, type check, and
// edge collection that the rename closure needs. It runs against the
// transaction's base snapshot so all callers see a consistent epoch.
func loadAndValidateRename(
	tx *Tx, entityType, oldID, newID string,
) (entity *model.Entity, incoming, outgoing []*model.Relation, err error) {
	g := tx.base.graph
	if err := validateRename(g, oldID, newID); err != nil {
		return nil, nil, nil, err
	}
	entity, ok := g.GetNode(oldID)
	if !ok {
		return nil, nil, nil, fmt.Errorf("entity not found: %s", oldID)
	}
	if entity.Type != entityType {
		return nil, nil, nil, fmt.Errorf("entity %s has type %s, not %s", oldID, entity.Type, entityType)
	}
	return entity, g.IncomingEdges(oldID), g.OutgoingEdges(oldID), nil
}

// validateRename checks rename preconditions against the given graph.
func validateRename(g *graph.Graph, oldID, newID string) error {
	if _, ok := g.GetNode(oldID); !ok {
		return fmt.Errorf("entity not found: %s", oldID)
	}
	if _, ok := g.GetNode(newID); ok {
		return fmt.Errorf("entity with ID %s already exists", newID)
	}
	if err := model.ValidateID(newID); err != nil {
		return fmt.Errorf("invalid new ID: %w", err)
	}
	return nil
}

// buildRenameResult assembles the descriptive Result returned by Rename.
// It runs before the transaction so it can also serve dry-run callers.
func buildRenameResult(
	repo repository.Store,
	meta *metamodel.Metamodel,
	entityType, oldID, newID string,
	incoming, outgoing []*model.Relation,
) *rename.Result {
	result := &rename.Result{
		OldID:            oldID,
		NewID:            newID,
		EntityType:       entityType,
		EntityFile:       repo.EntityFilePath(entityType, newID, meta),
		RelationsUpdated: make([]rename.RelationRef, 0, len(incoming)+len(outgoing)),
		OldFilesDeleted:  make([]string, 0, 1+len(incoming)+len(outgoing)),
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

	result.OldFilesDeleted = append(result.OldFilesDeleted,
		repo.EntityFilePath(entityType, oldID, meta))
	for _, rel := range outgoing {
		result.OldFilesDeleted = append(result.OldFilesDeleted,
			repo.Paths().RelationFilePath(oldID, rel.Type, rel.To))
	}
	for _, rel := range incoming {
		result.OldFilesDeleted = append(result.OldFilesDeleted,
			repo.Paths().RelationFilePath(rel.From, rel.Type, oldID))
	}

	return result
}

// writeRenamedRelations writes the relation files with the new ID
// substituted for the old one (in either direction). Self-referential
// relations are written exactly once.
func writeRenamedRelations(tx *Tx, oldID, newID string, incoming, outgoing []*model.Relation) error {
	// Outgoing: from = oldID becomes from = newID. If the relation is
	// self-referential (to == oldID), the new to is also newID.
	for _, rel := range outgoing {
		newTo := rel.To
		if rel.To == oldID {
			newTo = newID
		}
		newRel := &model.Relation{
			From:       newID,
			Type:       rel.Type,
			To:         newTo,
			Properties: rel.Properties,
			Content:    rel.Content,
		}
		if err := tx.WriteRelation(newRel); err != nil {
			return fmt.Errorf("write relation %s--%s-->%s: %w", newID, rel.Type, newTo, err)
		}
	}

	// Incoming: to = oldID becomes to = newID. Skip self-referential
	// relations because they were already handled in the outgoing loop.
	for _, rel := range incoming {
		if rel.From == oldID {
			continue
		}
		newRel := &model.Relation{
			From:       rel.From,
			Type:       rel.Type,
			To:         newID,
			Properties: rel.Properties,
			Content:    rel.Content,
		}
		if err := tx.WriteRelation(newRel); err != nil {
			return fmt.Errorf("write relation %s--%s-->%s: %w", rel.From, rel.Type, newID, err)
		}
	}

	return nil
}
