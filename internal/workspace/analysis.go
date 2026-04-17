package workspace

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/validation"
)

// MetamodelAccessor provides read-only access to metamodel for validation.
// This interface is used by the validate command to check filter values.
type MetamodelAccessor interface {
	HasEntityType(entityType string) bool
	HasValidationRule(ruleName string) bool
}

// ValidationFilter specifies which validation rules to run.
type ValidationFilter struct {
	RuleName   string // Exact rule name match (empty = no filter)
	EntityType string // Entity type filter (empty = no filter)
}

// AnalyzeOptions configures analysis scope.
type AnalyzeOptions struct {
	// Scope limits analysis to specific entity IDs. If nil, all entities are analyzed.
	Scope map[string]bool
}

// --- Orphan Analysis ---

// FindOrphansWithScope returns entities with no relations, filtered by scope.
func (w *Workspace) FindOrphansWithScope(opts AnalyzeOptions) []*model.Entity {
	orphans := w.graph().FindOrphans()
	return filterByScope(orphans, opts.Scope)
}

// --- Duplicate Analysis ---

// DuplicateGroup represents entities with the same normalized title.
type DuplicateGroup struct {
	Title    string
	Entities []*model.Entity
}

// FindDuplicates returns groups of entities with similar titles, filtered by scope.
func (w *Workspace) FindDuplicates(opts AnalyzeOptions) []DuplicateGroup {
	entities := filterByScope(w.graph().AllNodes(), opts.Scope)

	// Group by normalized title
	titleGroups := make(map[string][]*model.Entity)
	for _, e := range entities {
		title := normalizeTitle(e.Title())
		if title != "" {
			titleGroups[title] = append(titleGroups[title], e)
		}
	}

	// Collect duplicates
	var duplicates []DuplicateGroup
	for _, group := range titleGroups {
		if len(group) > 1 {
			duplicates = append(duplicates, DuplicateGroup{
				Title:    group[0].Title(), // Use original (non-normalized) title
				Entities: group,
			})
		}
	}

	return duplicates
}

// --- Gap Analysis ---

// GapResult contains gaps in an ID sequence.
type GapResult struct {
	Prefix  string
	Missing []string
}

// FindGaps returns gaps in ID sequences, filtered by scope.
// Excludes entity types with manual (string) IDs.
func (w *Workspace) FindGaps(opts AnalyzeOptions) []GapResult {
	meta := w.meta()
	// Build a set of prefixes that belong to manual ID types (should be skipped)
	stringIDPrefixes := make(map[string]bool)
	for _, entityDef := range meta.Entities {
		if entityDef.IsManualID() {
			for _, idPrefix := range entityDef.GetIDPrefixes() {
				prefix := strings.TrimSuffix(idPrefix, "-")
				stringIDPrefixes[prefix] = true
			}
		}
	}

	// Group IDs by prefix (only for sequential ID types)
	prefixGroups := make(map[string][]int)
	for _, id := range w.graph().AllIDs() {
		if !inScope(id, opts.Scope) {
			continue
		}
		parsed, err := model.ParseEntityID(id)
		if err != nil || parsed.Prefix == "" {
			continue
		}
		if stringIDPrefixes[strings.TrimSuffix(parsed.Prefix, "-")] {
			continue
		}
		prefixGroups[parsed.Prefix] = append(prefixGroups[parsed.Prefix], parsed.Number)
	}

	// Find gaps in each sequence
	var allGaps []GapResult
	for prefix, numbers := range prefixGroups {
		sort.Ints(numbers)

		var gaps []int
		for i := 1; i < len(numbers); i++ {
			expected := numbers[i-1] + 1
			if numbers[i] != expected {
				for j := expected; j < numbers[i]; j++ {
					gaps = append(gaps, j)
				}
			}
		}

		if len(gaps) > 0 {
			gapStrs := make([]string, len(gaps))
			for i, n := range gaps {
				gapStrs[i] = fmt.Sprintf("%s%03d", prefix, n)
			}
			allGaps = append(allGaps, GapResult{
				Prefix:  prefix,
				Missing: gapStrs,
			})
		}
	}

	return allGaps
}

// --- Cardinality Analysis ---

// CardinalityViolation represents a cardinality constraint violation.
type CardinalityViolation struct {
	EntityID     string
	RelationType string
	Constraint   string // "min_outgoing", "max_outgoing", "min_incoming", "max_incoming"
	Required     int
	Actual       int
}

// CheckCardinality checks all cardinality constraints, filtered by scope.
func (w *Workspace) CheckCardinality(opts AnalyzeOptions) []CardinalityViolation {
	var violations []CardinalityViolation

	for relName, relDef := range w.meta().Relations {
		violations = append(violations, w.checkMinOutgoing(relName, relDef, opts.Scope)...)
		violations = append(violations, w.checkMaxOutgoing(relName, relDef, opts.Scope)...)
		violations = append(violations, w.checkMinIncoming(relName, relDef, opts.Scope)...)
		violations = append(violations, w.checkMaxIncoming(relName, relDef, opts.Scope)...)
	}

	return violations
}

func (w *Workspace) checkMinOutgoing(
	relName string, relDef metamodel.RelationDef, scope map[string]bool,
) []CardinalityViolation {
	if relDef.MinOutgoing == nil || *relDef.MinOutgoing == 0 {
		return nil
	}
	var violations []CardinalityViolation
	for _, sourceType := range relDef.From {
		for _, e := range filterByScope(w.graph().NodesByType(sourceType), scope) {
			count := w.countOutgoingByType(e.ID, relName)
			if count < *relDef.MinOutgoing {
				violations = append(violations, CardinalityViolation{
					EntityID:     e.ID,
					RelationType: relName,
					Constraint:   "min_outgoing",
					Required:     *relDef.MinOutgoing,
					Actual:       count,
				})
			}
		}
	}
	return violations
}

func (w *Workspace) checkMaxOutgoing(
	relName string, relDef metamodel.RelationDef, scope map[string]bool,
) []CardinalityViolation {
	if relDef.MaxOutgoing == nil {
		return nil
	}
	var violations []CardinalityViolation
	for _, sourceType := range relDef.From {
		for _, e := range filterByScope(w.graph().NodesByType(sourceType), scope) {
			count := w.countOutgoingByType(e.ID, relName)
			if count > *relDef.MaxOutgoing {
				violations = append(violations, CardinalityViolation{
					EntityID:     e.ID,
					RelationType: relName,
					Constraint:   "max_outgoing",
					Required:     *relDef.MaxOutgoing,
					Actual:       count,
				})
			}
		}
	}
	return violations
}

func (w *Workspace) checkMinIncoming(
	relName string, relDef metamodel.RelationDef, scope map[string]bool,
) []CardinalityViolation {
	if relDef.MinIncoming == nil || *relDef.MinIncoming == 0 {
		return nil
	}
	var violations []CardinalityViolation
	for _, targetType := range relDef.To {
		for _, e := range filterByScope(w.graph().NodesByType(targetType), scope) {
			count := w.countIncomingByType(e.ID, relName)
			if count < *relDef.MinIncoming {
				// Use inverse relation name for the message if available
				relLabel := relName
				if relDef.Inverse != nil && relDef.Inverse.GetID() != "" {
					relLabel = relDef.Inverse.GetID()
				}
				violations = append(violations, CardinalityViolation{
					EntityID:     e.ID,
					RelationType: relLabel,
					Constraint:   "min_incoming",
					Required:     *relDef.MinIncoming,
					Actual:       count,
				})
			}
		}
	}
	return violations
}

func (w *Workspace) checkMaxIncoming(
	relName string, relDef metamodel.RelationDef, scope map[string]bool,
) []CardinalityViolation {
	if relDef.MaxIncoming == nil {
		return nil
	}
	var violations []CardinalityViolation
	for _, targetType := range relDef.To {
		for _, e := range filterByScope(w.graph().NodesByType(targetType), scope) {
			count := w.countIncomingByType(e.ID, relName)
			if count > *relDef.MaxIncoming {
				// Use inverse relation name for the message if available
				relLabel := relName
				if relDef.Inverse != nil && relDef.Inverse.GetID() != "" {
					relLabel = relDef.Inverse.GetID()
				}
				violations = append(violations, CardinalityViolation{
					EntityID:     e.ID,
					RelationType: relLabel,
					Constraint:   "max_incoming",
					Required:     *relDef.MaxIncoming,
					Actual:       count,
				})
			}
		}
	}
	return violations
}

func (w *Workspace) countOutgoingByType(entityID, relName string) int {
	count := 0
	for _, edge := range w.graph().OutgoingEdges(entityID) {
		if edge.Type == relName {
			count++
		}
	}
	return count
}

func (w *Workspace) countIncomingByType(entityID, relName string) int {
	count := 0
	for _, edge := range w.graph().IncomingEdges(entityID) {
		if edge.Type == relName {
			count++
		}
	}
	return count
}

// --- Property Validation ---

// PropertyError represents a property validation error.
type PropertyError struct {
	EntityID   string
	EntityType string
	Errors     []*metamodel.ValidationError
}

// ValidateProperties validates entity properties against the metamodel, filtered by scope.
func (w *Workspace) ValidateProperties(opts AnalyzeOptions) []PropertyError {
	meta := w.meta()
	entities := filterByScope(w.graph().AllNodes(), opts.Scope)

	var allErrors []PropertyError
	for _, entity := range entities {
		errs := meta.ValidateEntity(entity.ID, entity.Type, entity.Properties)
		if len(errs) > 0 {
			allErrors = append(allErrors, PropertyError{
				EntityID:   entity.ID,
				EntityType: entity.Type,
				Errors:     errs,
			})
		}
	}

	return allErrors
}

// RelationPropertyError represents a relation property validation error.
type RelationPropertyError struct {
	RelationKey  string // "from--type--to"
	RelationType string
	Errors       []*metamodel.ValidationError
}

// ValidateRelationProperties validates relation properties against the metamodel.
func (w *Workspace) ValidateRelationProperties() []RelationPropertyError {
	meta := w.meta()
	var allErrors []RelationPropertyError
	for _, rel := range w.graph().AllEdges() {
		errs := meta.ValidateRelationProperties(rel.Type, rel.Properties)
		if len(errs) > 0 {
			allErrors = append(allErrors, RelationPropertyError{
				RelationKey:  rel.From + "--" + rel.Type + "--" + rel.To,
				RelationType: rel.Type,
				Errors:       errs,
			})
		}
	}
	return allErrors
}

// --- Custom Validations ---

// ValidationViolation is re-exported from the validation package.
type ValidationViolation = validation.Violation

// newValidationService creates a validation service with workspace and project root configured.
func (w *Workspace) newValidationService() *validation.Service {
	var root string
	if w.repo != nil {
		root = w.repo.Paths().Root
	}
	return validation.New(w.meta(), w.luaServices(), root)
}

// RunValidations executes all custom validation rules from the metamodel, filtered by scope.
func (w *Workspace) RunValidations(opts AnalyzeOptions) []ValidationViolation {
	return w.newValidationService().Check(nodesToDomain(w.graph().AllNodes()), opts.Scope)
}

// RunValidationsFiltered executes custom validation rules matching the given filters.
// Multiple filters are combined with OR (union of matching rules).
// If a filter has both RuleName and EntityType empty, all rules match.
func (w *Workspace) RunValidationsFiltered(opts AnalyzeOptions, filters []ValidationFilter) []ValidationViolation {
	svc := w.newValidationService()

	// Build set of rule names to run based on filters
	ruleNames := make(map[string]bool)
	for _, filter := range filters {
		for _, rule := range svc.Rules() {
			if matchesFilter(rule, filter) {
				ruleNames[rule.Name] = true
			}
		}
	}

	// Run only matching rules
	return svc.CheckRules(nodesToDomain(w.graph().AllNodes()), opts.Scope, ruleNames)
}

// nodesToDomain converts a slice of legacy model.Entity to entity.Entity for
// consumers that have already moved to the domain type. This conversion will
// go away when the graph itself flips to *entity.Entity.
func nodesToDomain(nodes []*model.Entity) []*entity.Entity {
	out := make([]*entity.Entity, len(nodes))
	for i, n := range nodes {
		out[i] = model.EntityToDomain(n)
	}
	return out
}

// matchesFilter returns true if the rule matches the filter criteria.
func matchesFilter(rule metamodel.ValidationRule, filter ValidationFilter) bool {
	// Rule name exact match
	if filter.RuleName != "" {
		return rule.Name == filter.RuleName
	}

	// Entity type match
	if filter.EntityType != "" {
		return rule.EntityType == filter.EntityType
	}

	// Empty filter matches all rules
	return true
}

// CountValidationsBySeverity returns counts of errors and warnings from violations.
func CountValidationsBySeverity(violations []ValidationViolation) (errors, warnings int) {
	return validation.CountBySeverity(violations)
}

// --- Summary Analysis ---

// AnalysisSummary contains counts from all analysis types.
type AnalysisSummary struct {
	Orphans            int
	Duplicates         int
	Gaps               int
	Cardinality        int
	PropertyErrors     int
	ValidationErrors   int
	ValidationWarnings int
}

// AnalyzeAll runs all analyses and returns a summary of counts.
func (w *Workspace) AnalyzeAll(opts AnalyzeOptions) *AnalysisSummary {
	summary := &AnalysisSummary{
		Orphans:     len(w.FindOrphansWithScope(opts)),
		Duplicates:  len(w.FindDuplicates(opts)),
		Gaps:        len(w.FindGaps(opts)),
		Cardinality: len(w.CheckCardinality(opts)),
	}

	// Count property errors
	for _, pe := range w.ValidateProperties(opts) {
		summary.PropertyErrors += len(pe.Errors)
	}

	// Count validation issues by severity
	violations := w.RunValidations(opts)
	summary.ValidationErrors, summary.ValidationWarnings = validation.CountBySeverity(violations)

	return summary
}

// filterByScope filters entities to only those in the scope.
// If scope is nil, returns the original slice unchanged.
func filterByScope(entities []*model.Entity, scope map[string]bool) []*model.Entity {
	if scope == nil {
		return entities
	}
	result := make([]*model.Entity, 0, len(entities))
	for _, e := range entities {
		if scope[e.ID] {
			result = append(result, e)
		}
	}
	return result
}

// inScope checks if an entity ID is in the scope.
// Returns true if scope is nil (no filtering) or if the ID exists in the scope map.
func inScope(entityID string, scope map[string]bool) bool {
	if scope == nil {
		return true
	}
	_, exists := scope[entityID]
	return exists
}

// normalizeTitle normalizes a title for duplicate detection.
func normalizeTitle(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}
