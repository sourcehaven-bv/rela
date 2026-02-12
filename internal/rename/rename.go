package rename

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
)

// Options configures the rename operation.
type Options struct {
	DryRun bool // If true, return what would change without making changes
}

// RelationRef identifies a relation by its three-part key.
type RelationRef struct {
	From string `json:"from"`
	Type string `json:"type"`
	To   string `json:"to"`
}

// Result contains the outcome of a rename operation.
type Result struct {
	OldID            string
	NewID            string
	EntityType       string
	EntityFile       string        // Path to new entity file
	RelationsUpdated []RelationRef // Relations that were updated
	OldFilesDeleted  []string      // Files that were deleted
}

// Store defines the repository operations needed for rename.
type Store interface {
	EntityFilePath(entityType, id string, meta *metamodel.Metamodel) string
	Paths() *project.Context
	Transaction(fn func(tx repository.Tx) error) error
}

// Rename performs a staged entity ID rename with transaction tracking.
// It updates the entity file and all relations referencing the old ID.
func Rename(
	repo Store,
	meta *metamodel.Metamodel,
	g *graph.Graph,
	entityType, oldID, newID string,
	opts Options,
) (*Result, error) {
	// 1. Validate inputs
	if err := validate(g, oldID, newID); err != nil {
		return nil, err
	}

	// 2. Get the entity
	entity, ok := g.GetNode(oldID)
	if !ok {
		return nil, fmt.Errorf("entity not found: %s", oldID)
	}
	if entity.Type != entityType {
		return nil, fmt.Errorf("entity %s has type %s, not %s", oldID, entity.Type, entityType)
	}

	// 3. Collect affected relations
	incoming := g.IncomingEdges(oldID)
	outgoing := g.OutgoingEdges(oldID)

	// 4. Build result for dry-run or actual execution
	result := &Result{
		OldID:            oldID,
		NewID:            newID,
		EntityType:       entityType,
		EntityFile:       repo.EntityFilePath(entityType, newID, meta),
		RelationsUpdated: make([]RelationRef, 0, len(incoming)+len(outgoing)),
		OldFilesDeleted:  make([]string, 0),
	}

	// Collect relation refs
	for _, rel := range outgoing {
		result.RelationsUpdated = append(result.RelationsUpdated, RelationRef{
			From: newID, // Will be renamed
			Type: rel.Type,
			To:   rel.To,
		})
	}
	for _, rel := range incoming {
		result.RelationsUpdated = append(result.RelationsUpdated, RelationRef{
			From: rel.From,
			Type: rel.Type,
			To:   newID, // Will be renamed
		})
	}

	// Collect files to delete
	result.OldFilesDeleted = append(result.OldFilesDeleted, repo.EntityFilePath(entityType, oldID, meta))
	for _, rel := range outgoing {
		result.OldFilesDeleted = append(result.OldFilesDeleted,
			repo.Paths().RelationFilePath(oldID, rel.Type, rel.To))
	}
	for _, rel := range incoming {
		result.OldFilesDeleted = append(result.OldFilesDeleted,
			repo.Paths().RelationFilePath(rel.From, rel.Type, oldID))
	}

	if opts.DryRun {
		return result, nil
	}

	// 5. Execute the rename with transaction tracking
	if err := executeRename(repo, meta, g, entity, oldID, newID, incoming, outgoing); err != nil {
		return nil, err
	}

	return result, nil
}

func validate(g *graph.Graph, oldID, newID string) error {
	// Check old ID exists
	if _, ok := g.GetNode(oldID); !ok {
		return fmt.Errorf("entity not found: %s", oldID)
	}

	// Check new ID doesn't exist
	if _, ok := g.GetNode(newID); ok {
		return fmt.Errorf("entity with ID %s already exists", newID)
	}

	// Validate new ID format
	if err := model.ValidateID(newID); err != nil {
		return fmt.Errorf("invalid new ID: %w", err)
	}

	return nil
}

func executeRename(
	repo Store,
	meta *metamodel.Metamodel,
	g *graph.Graph,
	entity *model.Entity,
	oldID, newID string,
	incoming, outgoing []*model.Relation,
) error {
	err := repo.Transaction(func(tx repository.Tx) error {
		// Write new entity file
		newEntity := &model.Entity{
			ID:         newID,
			Type:       entity.Type,
			Properties: entity.Properties,
			Content:    entity.Content,
		}
		if writeErr := tx.WriteEntity(newEntity, meta); writeErr != nil {
			return fmt.Errorf("write new entity: %w", writeErr)
		}

		// Write updated relations
		if relErr := writeUpdatedRelations(tx, oldID, newID, incoming, outgoing); relErr != nil {
			return relErr
		}

		// Delete old files
		if delErr := tx.DeleteEntity(entity.Type, oldID, meta); delErr != nil {
			return fmt.Errorf("delete old entity: %w", delErr)
		}
		for _, rel := range outgoing {
			if delErr := tx.DeleteRelation(oldID, rel.Type, rel.To); delErr != nil {
				return fmt.Errorf("delete old relation: %w", delErr)
			}
		}
		for _, rel := range incoming {
			if delErr := tx.DeleteRelation(rel.From, rel.Type, oldID); delErr != nil {
				return fmt.Errorf("delete old relation: %w", delErr)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Update graph in-memory (only after successful commit)
	updateGraph(g, entity, newID, incoming, outgoing)

	return nil
}

func writeUpdatedRelations(tx repository.Tx, oldID, newID string, incoming, outgoing []*model.Relation) error {
	// Update outgoing relations (from=oldID -> from=newID)
	for _, rel := range outgoing {
		newTo := rel.To
		if rel.To == oldID { // Self-referential
			newTo = newID
		}
		newRel := &model.Relation{
			From: newID, Type: rel.Type, To: newTo,
			Properties: rel.Properties, Content: rel.Content,
		}
		if err := tx.WriteRelation(newRel); err != nil {
			return fmt.Errorf("write relation %s--%s-->%s: %w", newID, rel.Type, newTo, err)
		}
	}

	// Update incoming relations (skip self-referential, already handled)
	for _, rel := range incoming {
		if rel.From == oldID {
			continue
		}
		newRel := &model.Relation{
			From: rel.From, Type: rel.Type, To: newID,
			Properties: rel.Properties, Content: rel.Content,
		}
		if err := tx.WriteRelation(newRel); err != nil {
			return fmt.Errorf("write relation %s--%s-->%s: %w", rel.From, rel.Type, newID, err)
		}
	}

	return nil
}

func updateGraph(
	g *graph.Graph,
	entity *model.Entity,
	newID string,
	incoming, outgoing []*model.Relation,
) {
	oldID := entity.ID

	// Remove old edges
	for _, rel := range outgoing {
		g.RemoveEdge(oldID, rel.Type, rel.To)
	}
	for _, rel := range incoming {
		g.RemoveEdge(rel.From, rel.Type, oldID)
	}

	// Remove old node
	g.RemoveNode(oldID)

	// Add new node
	newEntity := &model.Entity{
		ID:         newID,
		Type:       entity.Type,
		Properties: entity.Properties,
		Content:    entity.Content,
		FilePath:   entity.FilePath, // Will be updated by WriteEntity
	}
	g.AddNode(newEntity)

	// Add new edges
	for _, rel := range outgoing {
		// For self-referential relations, update both from and to
		newTo := rel.To
		if rel.To == oldID {
			newTo = newID
		}
		g.AddEdge(&model.Relation{
			From:       newID,
			Type:       rel.Type,
			To:         newTo,
			Properties: rel.Properties,
			Content:    rel.Content,
		})
	}
	for _, rel := range incoming {
		// Skip self-referential (already added in outgoing loop)
		if rel.From == oldID {
			continue
		}
		g.AddEdge(&model.Relation{
			From:       rel.From,
			Type:       rel.Type,
			To:         newID,
			Properties: rel.Properties,
			Content:    rel.Content,
		})
	}
}
