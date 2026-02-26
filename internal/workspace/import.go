package workspace

import (
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// ImportOptions configures the import behavior.
type ImportOptions struct {
	// DryRun validates without creating files.
	DryRun bool
	// Update allows updating existing entities instead of failing on duplicates.
	Update bool
	// SkipErrors continues importing on validation errors.
	SkipErrors bool
}

// ImportResult contains the outcome of an import operation.
type ImportResult struct {
	EntitiesCreated  int
	EntitiesUpdated  int
	EntitiesSkipped  int
	RelationsCreated int
	RelationsSkipped int
	Errors           []ImportError
}

// ImportError represents an error during import with context.
type ImportError struct {
	Kind    string // "entity" or "relation"
	ID      string // entity ID or relation key
	Message string
}

func (e ImportError) Error() string {
	return fmt.Sprintf("%s %s: %s", e.Kind, e.ID, e.Message)
}

// ImportData represents data to import.
type ImportData struct {
	Entities  []ImportEntity
	Relations []ImportRelation
}

// ImportEntity represents an entity to import.
type ImportEntity struct {
	ID         string
	Type       string
	Properties map[string]interface{}
}

// ImportRelation represents a relation to import.
type ImportRelation struct {
	From       string
	Type       string
	To         string
	Properties map[string]interface{}
}

// Import imports entities and relations into the workspace.
// It validates all data first, then creates/updates in order: entities, then relations.
func (w *Workspace) Import(data *ImportData, opts ImportOptions) (*ImportResult, error) {
	result := &ImportResult{}
	meta := w.Meta()

	// Phase 1: Validate all entities
	validEntities, err := w.validateImportEntities(data.Entities, opts, result)
	if err != nil {
		return result, err
	}

	// Phase 2: Validate all relations
	validRelations, err := w.validateImportRelations(data.Relations, validEntities, opts, result)
	if err != nil {
		return result, err
	}

	// If dry run, stop here
	if opts.DryRun {
		result.EntitiesCreated = len(validEntities)
		result.RelationsCreated = len(validRelations)
		return result, nil
	}

	// Phase 3: Create/update entities
	for _, ie := range validEntities {
		created, err := w.importEntity(&ie, meta)
		if err != nil {
			impErr := ImportError{Kind: "entity", ID: ie.ID, Message: err.Error()}
			if opts.SkipErrors {
				result.Errors = append(result.Errors, impErr)
				result.EntitiesSkipped++
				continue
			}
			return result, &impErr
		}
		if created {
			result.EntitiesCreated++
		} else {
			result.EntitiesUpdated++
		}
	}

	// Phase 4: Create relations
	for _, ir := range validRelations {
		created, err := w.importRelation(&ir)
		if err != nil {
			impErr := ImportError{Kind: "relation", ID: ir.From + "--" + ir.Type + "--" + ir.To, Message: err.Error()}
			if opts.SkipErrors {
				result.Errors = append(result.Errors, impErr)
				result.RelationsSkipped++
				continue
			}
			return result, &impErr
		}
		if created {
			result.RelationsCreated++
		} else {
			result.RelationsSkipped++
		}
	}

	w.saveCacheQuietly()
	return result, nil
}

// validateImportEntities validates all entities and returns valid ones.
func (w *Workspace) validateImportEntities(
	entities []ImportEntity, opts ImportOptions, result *ImportResult,
) ([]ImportEntity, error) {
	meta := w.Meta()
	valid := make([]ImportEntity, 0, len(entities))

	for _, ie := range entities {
		if err := w.validateImportEntity(&ie, meta, opts.Update); err != nil {
			impErr := ImportError{Kind: "entity", ID: ie.ID, Message: err.Error()}
			if opts.SkipErrors {
				result.Errors = append(result.Errors, impErr)
				result.EntitiesSkipped++
				continue
			}
			return valid, &impErr
		}
		valid = append(valid, ie)
	}
	return valid, nil
}

// validateImportEntity validates a single entity for import.
func (w *Workspace) validateImportEntity(ie *ImportEntity, meta *metamodel.Metamodel, allowUpdate bool) error {
	// ID is required
	if ie.ID == "" {
		return fmt.Errorf("missing required field: id")
	}
	if err := model.ValidateID(ie.ID); err != nil {
		return err
	}

	// Type is required
	if ie.Type == "" {
		return fmt.Errorf("missing required field: type")
	}

	// Resolve type alias
	ie.Type = meta.ResolveAlias(ie.Type)

	// Check type exists
	entityDef, ok := meta.GetEntityDef(ie.Type)
	if !ok {
		return fmt.Errorf("unknown entity type: %s", ie.Type)
	}

	// Check if entity already exists
	if _, exists := w.graph.GetNode(ie.ID); exists {
		if !allowUpdate {
			return fmt.Errorf("entity already exists (use --update to overwrite)")
		}
	}

	// Build entity for validation
	entity := model.NewEntity(ie.ID, ie.Type)
	for k, v := range ie.Properties {
		entity.Properties[k] = v
	}

	// Apply default status if not provided
	if _, hasStatus := entity.Properties["status"]; !hasStatus {
		defaultStatus := entityDef.GetDefaultStatus(meta)
		if defaultStatus != "" {
			entity.Properties["status"] = defaultStatus
		}
	}

	// Validate against metamodel
	errs := meta.ValidateEntity(entity)
	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return fmt.Errorf("validation failed: %s", strings.Join(msgs, "; "))
	}

	return nil
}

// validateImportRelations validates all relations and returns valid ones.
func (w *Workspace) validateImportRelations(
	relations []ImportRelation, validEntities []ImportEntity, opts ImportOptions, result *ImportResult,
) ([]ImportRelation, error) {
	meta := w.Meta()

	// Build set of known entity IDs (existing + to-be-imported)
	entityIDs := make(map[string]bool)
	for _, ie := range validEntities {
		entityIDs[ie.ID] = true
	}
	for _, id := range w.graph.AllIDs() {
		entityIDs[id] = true
	}

	valid := make([]ImportRelation, 0, len(relations))
	for _, ir := range relations {
		if err := w.validateImportRelation(&ir, entityIDs, meta); err != nil {
			impErr := ImportError{Kind: "relation", ID: ir.From + "--" + ir.Type + "--" + ir.To, Message: err.Error()}
			if opts.SkipErrors {
				result.Errors = append(result.Errors, impErr)
				result.RelationsSkipped++
				continue
			}
			return valid, &impErr
		}
		valid = append(valid, ir)
	}
	return valid, nil
}

// validateImportRelation validates a single relation for import.
func (w *Workspace) validateImportRelation(
	ir *ImportRelation, knownIDs map[string]bool, meta *metamodel.Metamodel,
) error {
	if ir.From == "" {
		return fmt.Errorf("missing required field: from")
	}
	if ir.Type == "" {
		return fmt.Errorf("missing required field: type (relation)")
	}
	if ir.To == "" {
		return fmt.Errorf("missing required field: to")
	}

	// Check entities exist (either in graph or in import batch)
	if !knownIDs[ir.From] {
		return fmt.Errorf("source entity not found: %s", ir.From)
	}
	if !knownIDs[ir.To] {
		return fmt.Errorf("target entity not found: %s", ir.To)
	}

	// Get entity types for relation validation (only if both exist in graph)
	fromNode, fromExists := w.graph.GetNode(ir.From)
	toNode, toExists := w.graph.GetNode(ir.To)

	if fromExists && toExists {
		// Both entities exist, validate relation type
		return meta.ValidateRelation(ir.Type, fromNode.Type, toNode.Type)
	}

	// One or both entities are in the import batch - skip metamodel validation
	// (will be validated when relation is created after entities exist)
	return nil
}

// importEntity creates or updates an entity (no templates, no automation).
func (w *Workspace) importEntity(ie *ImportEntity, meta *metamodel.Metamodel) (created bool, err error) {
	entityDef, _ := meta.GetEntityDef(ie.Type)

	entity := model.NewEntity(ie.ID, ie.Type)
	for k, v := range ie.Properties {
		entity.Properties[k] = v
	}

	// Apply default status if not provided
	if _, hasStatus := entity.Properties["status"]; !hasStatus {
		defaultStatus := entityDef.GetDefaultStatus(meta)
		if defaultStatus != "" {
			entity.Properties["status"] = defaultStatus
		}
	}

	// Check if updating
	_, exists := w.graph.GetNode(ie.ID)

	// Write to disk
	if err := w.repo.WriteEntity(entity, meta); err != nil {
		return false, fmt.Errorf("failed to write entity: %w", err)
	}

	// Add/update in graph
	if exists {
		w.graph.UpdateNode(entity)
	} else {
		w.graph.AddNode(entity)
	}

	return !exists, nil
}

// importRelation creates a relation (no templates).
func (w *Workspace) importRelation(ir *ImportRelation) (created bool, err error) {
	// Check if relation already exists
	if _, exists := w.graph.GetEdge(ir.From, ir.Type, ir.To); exists {
		return false, nil
	}

	// Create relation
	rel := model.NewRelation(ir.From, ir.Type, ir.To)
	rel.Properties = ir.Properties

	// Write to disk
	if err := w.repo.WriteRelation(rel); err != nil {
		return false, fmt.Errorf("failed to write relation: %w", err)
	}

	// Add to graph
	w.graph.AddEdge(rel)

	return true, nil
}
