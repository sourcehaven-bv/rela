package views

import (
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// Engine executes view definitions against a graph
type Engine struct {
	graph *graph.Graph
	meta  *metamodel.Metamodel
}

// NewEngine creates a new view engine
func NewEngine(g *graph.Graph, meta *metamodel.Metamodel) *Engine {
	return &Engine{
		graph: g,
		meta:  meta,
	}
}

// Execute runs a view and returns the result
func (e *Engine) Execute(view ViewDef, entryID string) (*ViewResult, error) {
	// Get the entry entity
	entry, ok := e.graph.GetNode(entryID)
	if !ok {
		return nil, fmt.Errorf("entry entity not found: %s", entryID)
	}

	// Validate entry type
	if entry.Type != view.Entry.Type {
		return nil, fmt.Errorf("entry entity %s is type %s, expected %s", entryID, entry.Type, view.Entry.Type)
	}

	result := &ViewResult{
		Entry:       entry,
		Collections: make(map[string][]*model.Entity),
		Relations:   make(map[string][]ExportedRelation),
	}

	// Initialize collections with the entry entity
	result.Collections["entry"] = []*model.Entity{entry}

	// Apply traverse rules with multi-pass approach to handle dependencies
	// Run multiple passes until no new entities are found
	maxPasses := 10 // Prevent infinite loops
	for pass := 0; pass < maxPasses; pass++ {
		initialSize := e.countEntities(result.Collections)

		for _, rule := range view.Traverse {
			if err := e.applyTraverseRule(rule, result); err != nil {
				return nil, fmt.Errorf("traverse rule failed: %w", err)
			}
		}

		newSize := e.countEntities(result.Collections)
		if newSize == initialSize {
			// No new entities found, we're done
			break
		}
	}

	// Apply filters
	if err := e.applyFilters(view.Filters, result); err != nil {
		return nil, fmt.Errorf("filter failed: %w", err)
	}

	// Remove entry collection (it was just for traversal)
	delete(result.Collections, "entry")

	// Apply derived collections
	if err := e.applyDerived(view.Derived, result); err != nil {
		return nil, fmt.Errorf("derived collection failed: %w", err)
	}

	// Apply relation exports
	e.applyRelationExports(view.RelationExports, result)

	// Enrich result with content and relation titles if requested
	e.enrichResult(view.Output, result)

	return result, nil
}

func (e *Engine) applyTraverseRule(rule TraverseRule, result *ViewResult) error {
	fromCollections := rule.GetFromCollections()
	collectAsNames := rule.GetCollectAsNames()

	// Gather source entities
	var sourceEntities []*model.Entity
	for _, fromCol := range fromCollections {
		if fromCol == "*" {
			// Collect from all existing collections
			for _, entities := range result.Collections {
				sourceEntities = append(sourceEntities, entities...)
			}
		} else {
			// Collect from specific collection
			if entities, ok := result.Collections[fromCol]; ok {
				sourceEntities = append(sourceEntities, entities...)
			}
		}
	}

	// Traverse from each source entity
	var foundEntities []*model.Entity
	for _, source := range sourceEntities {
		if rule.Recursive {
			maxDepth := rule.MaxDepth
			if maxDepth <= 0 {
				maxDepth = 10 // default
			}
			entities := e.traverseRecursive(source.ID, rule, 0, maxDepth)
			foundEntities = append(foundEntities, entities...)
		} else {
			entities := e.traverseOnce(source.ID, rule)
			foundEntities = append(foundEntities, entities...)
		}
	}

	// Apply where filter if specified
	if rule.Where != "" {
		filtered, err := e.filterEntities(foundEntities, rule.Where)
		if err != nil {
			return fmt.Errorf("where filter failed: %w", err)
		}
		foundEntities = filtered
	}

	// Add to collections with type filtering and deduplication
	for _, collectAs := range collectAsNames {
		if result.Collections[collectAs] == nil {
			result.Collections[collectAs] = []*model.Entity{}
		}

		// Build a set of existing entity IDs in the collection
		existing := make(map[string]bool)
		for _, entity := range result.Collections[collectAs] {
			existing[entity.ID] = true
		}

		// Filter entities by type if collection name suggests a type
		// and deduplicate
		for _, entity := range foundEntities {
			if e.matchesCollectionType(collectAs, entity.Type) && !existing[entity.ID] {
				result.Collections[collectAs] = append(result.Collections[collectAs], entity)
				existing[entity.ID] = true
			}
		}
	}

	return nil
}

func (e *Engine) traverseOnce(sourceID string, rule TraverseRule) []*model.Entity {
	var entities []*model.Entity

	//nolint:nestif // Traversal logic requires nested conditions
	if rule.Follow != "" {
		// Follow outgoing relations
		edges := e.graph.OutgoingEdges(sourceID)
		for _, edge := range edges {
			if edge.Type == rule.Follow {
				if target, ok := e.graph.GetNode(edge.To); ok {
					entities = append(entities, target)
				}
			}
		}
	} else if rule.FollowIncoming != "" {
		// Follow incoming relations
		edges := e.graph.IncomingEdges(sourceID)
		for _, edge := range edges {
			if edge.Type == rule.FollowIncoming {
				if source, ok := e.graph.GetNode(edge.From); ok {
					entities = append(entities, source)
				}
			}
		}
	}

	return entities
}

func (e *Engine) traverseRecursive(sourceID string, rule TraverseRule, depth, maxDepth int) []*model.Entity {
	if depth >= maxDepth {
		return nil
	}

	var entities []*model.Entity

	// Get immediate neighbors
	immediate := e.traverseOnce(sourceID, rule)
	entities = append(entities, immediate...)

	// Recursively traverse each neighbor
	for _, entity := range immediate {
		recursive := e.traverseRecursive(entity.ID, rule, depth+1, maxDepth)
		entities = append(entities, recursive...)
	}

	return entities
}

func (e *Engine) applyFilters(filters map[string]Filter, result *ViewResult) error {
	for collectionName, filterDef := range filters {
		collection, ok := result.Collections[collectionName]
		if !ok {
			// Collection doesn't exist yet, create it if expand mode is enabled
			if filterDef.Expand {
				collection = []*model.Entity{}
			} else {
				continue
			}
		}

		// If expand mode, add entities from the graph that match the filter
		if filterDef.Expand {
			expanded, err := e.expandCollection(collectionName, filterDef)
			if err != nil {
				return fmt.Errorf("expand for collection %s failed: %w", collectionName, err)
			}

			// Deduplicate when adding expanded entities
			existing := make(map[string]bool)
			for _, entity := range collection {
				existing[entity.ID] = true
			}
			for _, entity := range expanded {
				if !existing[entity.ID] {
					collection = append(collection, entity)
					existing[entity.ID] = true
				}
			}
		}

		filtered, err := e.applyFilter(collection, filterDef)
		if err != nil {
			return fmt.Errorf("filter for collection %s failed: %w", collectionName, err)
		}

		result.Collections[collectionName] = filtered
	}
	return nil
}

//nolint:gocognit // Filter logic is inherently complex
func (e *Engine) applyFilter(entities []*model.Entity, filterDef Filter) ([]*model.Entity, error) {
	// Handle match_any
	if len(filterDef.MatchAny) > 0 {
		var result []*model.Entity
		seen := make(map[string]bool)

		for _, subFilter := range filterDef.MatchAny {
			filtered, err := e.applyFilter(entities, subFilter)
			if err != nil {
				return nil, err
			}
			for _, entity := range filtered {
				if !seen[entity.ID] {
					seen[entity.ID] = true
					result = append(result, entity)
				}
			}
		}
		return result, nil
	}

	var result []*model.Entity

	for _, entity := range entities {
		include := true

		// via_traversal is always true since entities come from traversal
		_ = filterDef.ViaTraversal

		// Check id_prefix
		if len(filterDef.IDPrefix) > 0 {
			prefixMatch := false
			for _, prefix := range filterDef.IDPrefix {
				if strings.HasPrefix(entity.ID, prefix) {
					prefixMatch = true
					break
				}
			}
			if !prefixMatch {
				include = false
			}
		}

		// Check where expression
		if filterDef.Where != "" && include {
			filtered, err := e.filterEntities([]*model.Entity{entity}, filterDef.Where)
			if err != nil {
				return nil, err
			}
			if len(filtered) == 0 {
				include = false
			}
		}

		if include {
			result = append(result, entity)
		}
	}

	return result, nil
}

func (e *Engine) filterEntities(entities []*model.Entity, whereExpr string) ([]*model.Entity, error) {
	// Parse the where expression
	f, err := filter.Parse(whereExpr)
	if err != nil {
		return nil, fmt.Errorf("invalid where expression: %w", err)
	}

	var result []*model.Entity
	for _, entity := range entities {
		// Get entity definition to determine property types
		entityDef, ok := e.meta.GetEntityDef(entity.Type)
		if !ok {
			// Skip entities with unknown type
			continue
		}

		propDef, ok := entityDef.Properties[f.Property]
		if !ok {
			// Property not defined in metamodel, skip
			continue
		}

		// Use the filter.Match function
		matches, err := filter.Match(entity, f, &propDef, e.meta)
		if err != nil {
			// Skip entities with match errors
			continue
		}

		if matches {
			result = append(result, entity)
		}
	}

	return result, nil
}

func (e *Engine) applyDerived(derived map[string]Derived, result *ViewResult) error {
	for derivedName, derivedDef := range derived {
		sourceCollection, ok := result.Collections[derivedDef.Source]
		if !ok {
			// Source collection doesn't exist, skip
			continue
		}

		derivedEntities, err := e.computeDerived(sourceCollection, derivedDef)
		if err != nil {
			return fmt.Errorf("derived collection %s failed: %w", derivedName, err)
		}

		// For group_by, derivedEntities is already grouped (handled separately)
		// For now, we'll handle group_by in the output formatter
		result.Collections[derivedName] = derivedEntities
		if derivedDef.GroupBy != "" {
			// Store grouping info for later
			if result.GroupedCollections == nil {
				result.GroupedCollections = make(map[string]GroupingInfo)
			}
			result.GroupedCollections[derivedName] = GroupingInfo{
				GroupBy: derivedDef.GroupBy,
			}
		}
	}
	return nil
}

func (e *Engine) computeDerived(source []*model.Entity, derived Derived) ([]*model.Entity, error) {
	result := source

	// Apply where filter
	if derived.Where != "" {
		filtered, err := e.filterEntities(result, derived.Where)
		if err != nil {
			return nil, err
		}
		result = filtered
	}

	// Note: group_by and embed are handled during output formatting
	// For now, just pass through the filtered entities

	return result, nil
}

//nolint:gocognit // Relation export logic is inherently complex
func (e *Engine) applyRelationExports(exports []RelationExport, result *ViewResult) {
	// Collect all entity IDs from all collections
	allEntityIDs := make(map[string]bool)
	for _, entities := range result.Collections {
		for _, entity := range entities {
			allEntityIDs[entity.ID] = true
		}
	}

	for _, export := range exports {
		var relations []ExportedRelation

		// Get all edges from the graph
		allEdges := e.graph.AllEdges()

		for _, edge := range allEdges {
			// Check if relation type matches
			typeMatch := false
			for _, relType := range export.Types {
				if edge.Type == relType {
					typeMatch = true
					break
				}
			}
			if !typeMatch {
				continue
			}

			// Check if between constraint is satisfied
			if len(export.Between) == 2 {
				fromType := export.Between[0]
				toType := export.Between[1]

				fromEntity, fromOk := e.graph.GetNode(edge.From)
				toEntity, toOk := e.graph.GetNode(edge.To)

				if !fromOk || !toOk {
					continue
				}

				if fromEntity.Type != fromType || toEntity.Type != toType {
					continue
				}
			}

			// Check if both endpoints are in our collected entities
			if !allEntityIDs[edge.From] || !allEntityIDs[edge.To] {
				continue
			}

			relations = append(relations, ExportedRelation{
				From:     edge.From,
				To:       edge.To,
				Type:     edge.Type,
				Content:  edge.Content,
				Relation: edge,
			})
		}

		result.Relations[export.CollectAs] = relations
	}
}

func (e *Engine) enrichResult(output OutputDef, result *ViewResult) {
	// Include entry entity in result if requested (default true)
	includeEntry := output.IncludeEntry
	if !includeEntry && !output.IncludeContent && !output.ResolveRelationTitles {
		// Default to true if not specified
		includeEntry = true
	}

	if !includeEntry {
		result.Entry = nil
	}
	// Entry is already set in result if includeEntry is true

	// Content and relation title resolution is handled during serialization
	// Store the output config in the result
	result.OutputConfig = output
}

// countEntities returns the total number of unique entities across all collections
func (e *Engine) countEntities(collections map[string][]*model.Entity) int {
	seen := make(map[string]bool)
	for _, entities := range collections {
		for _, entity := range entities {
			seen[entity.ID] = true
		}
	}
	return len(seen)
}

// expandCollection queries the graph for entities matching the filter criteria
func (e *Engine) expandCollection(collectionName string, filterDef Filter) ([]*model.Entity, error) {
	var expanded []*model.Entity
	seen := make(map[string]bool)

	// Get all nodes from the graph
	allNodes := e.graph.AllNodes()

	for _, entity := range allNodes {
		// Skip if already seen
		if seen[entity.ID] {
			continue
		}

		// Check if entity matches collection type
		if !e.matchesCollectionType(collectionName, entity.Type) {
			continue
		}

		// Check id_prefix if specified
		if len(filterDef.IDPrefix) > 0 {
			prefixMatch := false
			for _, prefix := range filterDef.IDPrefix {
				if strings.HasPrefix(entity.ID, prefix) {
					prefixMatch = true
					break
				}
			}
			if !prefixMatch {
				continue
			}
		}

		// Check where expression if specified
		if filterDef.Where != "" {
			filtered, err := e.filterEntities([]*model.Entity{entity}, filterDef.Where)
			if err != nil {
				return nil, err
			}
			if len(filtered) == 0 {
				continue
			}
		}

		// Entity matches all criteria
		expanded = append(expanded, entity)
		seen[entity.ID] = true
	}

	return expanded, nil
}

// matchesCollectionType checks if an entity type matches a collection name
// Supports both singular and plural forms, and exact matches
func (e *Engine) matchesCollectionType(collectionName, entityType string) bool {
	// Exact match (e.g., "function" matches "function")
	if collectionName == entityType {
		return true
	}

	// Check if collection name is plural of entity type
	if entityDef, ok := e.meta.GetEntityDef(entityType); ok {
		// Check label plural (e.g., "Functions" or "functions")
		plural := strings.ToLower(entityDef.GetPlural())
		if strings.ToLower(collectionName) == plural {
			return true
		}

		// Check directory plural (e.g., "functions")
		dirPlural := strings.ToLower(entityDef.GetDirPlural(entityType))
		if strings.ToLower(collectionName) == dirPlural {
			return true
		}
	}

	// Check simple pluralization (entityType + "s")
	if strings.ToLower(collectionName) == strings.ToLower(entityType+"s") {
		return true
	}

	// If collection name doesn't match any entity type pattern, accept all entities
	// This allows for generic collection names like "items", "results", etc.
	if !e.looksLikeEntityType(collectionName) {
		return true
	}

	return false
}

// looksLikeEntityType checks if a collection name appears to be an entity type
// by checking if it matches any known entity type (singular or plural)
func (e *Engine) looksLikeEntityType(collectionName string) bool {
	lower := strings.ToLower(collectionName)

	for entityType, entityDef := range e.meta.Entities {
		if strings.ToLower(entityType) == lower {
			return true
		}
		if strings.ToLower(entityDef.GetPlural()) == lower {
			return true
		}
		if strings.ToLower(entityDef.GetDirPlural(entityType)) == lower {
			return true
		}
		if strings.ToLower(entityType+"s") == lower {
			return true
		}
	}

	return false
}

// ViewResult represents the output of a view execution
type ViewResult struct {
	Entry              *model.Entity
	Collections        map[string][]*model.Entity
	GroupedCollections map[string]GroupingInfo
	Relations          map[string][]ExportedRelation
	OutputConfig       OutputDef
}

// GroupingInfo stores information about how a collection is grouped
type GroupingInfo struct {
	GroupBy string
}

// ExportedRelation represents a relation in the export
type ExportedRelation struct {
	From     string
	To       string
	Type     string
	Content  string
	Relation *model.Relation
}
