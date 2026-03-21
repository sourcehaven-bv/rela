package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/markdown"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/views"
)

var (
	analyzeViewName string
	analyzeEntryID  string

	// cachedScope holds the resolved scope to avoid re-executing the view multiple times.
	// Set by the first call to resolveAnalysisScope() in a command invocation.
	cachedScope     map[string]bool
	cachedScopeOnce bool
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze the entity graph",
	Long: `Runs various analysis checks on the entity graph.

Subcommands:
  orphans     - Find entities with no connections
  duplicates  - Find entities with similar titles
  gaps        - Find gaps in ID sequences
  cardinality - Check relation cardinality constraints
  properties  - Validate entity property values against metamodel
  validations - Run custom validation rules from metamodel
  all         - Run all analyses`,
}

// writeAnalysisJSON writes an analysis result in JSON format if JSON output is enabled.
// Returns true if JSON was written (caller should return), false if text output should be used.
// When count > 0, the status is set to "warning" and issuesFmt is used as the message format.
func writeAnalysisJSON(count int, details interface{}, successMsg, issuesFmt string) bool {
	if out.Format != "json" {
		return false
	}
	status := "success"
	message := successMsg
	if count > 0 {
		status = "warning"
		message = fmt.Sprintf(issuesFmt, count)
	}
	_ = out.WriteAnalysisResult(output.AnalysisResult{
		Status:  status,
		Message: message,
		Count:   count,
		Details: details,
	})
	return true
}

var analyzeOrphansCmd = &cobra.Command{
	Use:   "orphans",
	Short: "Find entities with no connections",
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, err := resolveAnalysisScope()
		if err != nil {
			return err
		}

		orphans := ws.FindOrphans()
		orphans = filterByScope(orphans, scope)
		filter.SortByID(orphans, false)

		if writeAnalysisJSON(len(orphans), orphans,
			"No orphan entities found", "Found %d orphan entities") {
			return nil
		}

		if len(orphans) == 0 {
			out.WriteSuccess("No orphan entities found")
			return nil
		}
		out.WriteWarning("Found %d orphan entities:", len(orphans))
		return out.WriteEntities(orphans)
	},
}

var analyzeDuplicatesCmd = &cobra.Command{
	Use:   "duplicates",
	Short: "Find entities with similar titles",
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, err := resolveAnalysisScope()
		if err != nil {
			return err
		}

		entities := ws.AllEntities()
		entities = filterByScope(entities, scope)

		// Group by normalized title
		titleGroups := make(map[string][]*model.Entity)
		for _, e := range entities {
			title := normalizeTitle(e.Title())
			if title != "" {
				titleGroups[title] = append(titleGroups[title], e)
			}
		}

		// Find duplicates
		var duplicates [][]*model.Entity
		for _, group := range titleGroups {
			if len(group) > 1 {
				duplicates = append(duplicates, group)
			}
		}

		// Handle JSON output format
		if out.Format == "json" {
			type duplicateGroup struct {
				Title    string          `json:"title"`
				Entities []*model.Entity `json:"entities"`
			}
			var details []duplicateGroup
			for _, group := range duplicates {
				details = append(details, duplicateGroup{
					Title:    group[0].Title(),
					Entities: group,
				})
			}
			writeAnalysisJSON(len(duplicates), details,
				"No duplicate titles found", "Found %d groups of potential duplicates")
			return nil
		}

		if len(duplicates) == 0 {
			out.WriteSuccess("No duplicate titles found")
			return nil
		}

		out.WriteWarning("Found %d groups of potential duplicates:", len(duplicates))
		for _, group := range duplicates {
			out.WriteMessage("")
			out.WriteMessage("  Title: %s", group[0].Title())
			for _, e := range group {
				out.WriteMessage("    - %s (%s)", e.ID, e.Type)
			}
		}

		return nil
	},
}

var analyzeGapsCmd = &cobra.Command{
	Use:   "gaps",
	Short: "Find gaps in ID sequences",
	Long: `Find gaps in ID sequences for entity types with sequential IDs.

Entity types configured with id_type: string are excluded from gap analysis
since they use manually-specified IDs that are not expected to be sequential.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, err := resolveAnalysisScope()
		if err != nil {
			return err
		}

		// Build a set of prefixes that belong to manual ID types (should be skipped)
		stringIDPrefixes := make(map[string]bool)
		for _, entityDef := range meta.Entities {
			if entityDef.IsManualID() {
				for _, idPrefix := range entityDef.GetIDPrefixes() {
					// Normalize prefix (remove trailing dash if present)
					prefix := strings.TrimSuffix(idPrefix, "-")
					stringIDPrefixes[prefix] = true
				}
			}
		}

		// Group IDs by prefix (only for sequential ID types)
		prefixGroups := make(map[string][]int)

		for _, id := range ws.EntityIDs() {
			// Filter by scope if specified
			if !inScope(id, scope) {
				continue
			}
			parsed, err := model.ParseEntityID(id)
			if err != nil || parsed.Prefix == "" {
				continue
			}
			// Skip if this prefix belongs to a string ID type
			if stringIDPrefixes[strings.TrimSuffix(parsed.Prefix, "-")] {
				continue
			}
			prefixGroups[parsed.Prefix] = append(prefixGroups[parsed.Prefix], parsed.Number)
		}

		// Collect all gaps
		type gapResult struct {
			Prefix  string   `json:"prefix"`
			Missing []string `json:"missing"`
		}
		var allGaps []gapResult

		for prefix, numbers := range prefixGroups {
			sort.Ints(numbers)

			// Find gaps
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
				allGaps = append(allGaps, gapResult{
					Prefix:  prefix,
					Missing: gapStrs,
				})
			}
		}

		if writeAnalysisJSON(len(allGaps), allGaps,
			"No ID sequence gaps found", "Found gaps in %d ID sequences") {
			return nil
		}

		if len(allGaps) == 0 {
			out.WriteSuccess("No ID sequence gaps found")
		} else {
			for _, gap := range allGaps {
				out.WriteWarning("Gaps in %s sequence:", gap.Prefix)
				out.WriteMessage("  Missing: %s", strings.Join(gap.Missing, ", "))
			}
		}

		return nil
	},
}

var analyzeCardinalityCmd = &cobra.Command{
	Use:   "cardinality",
	Short: "Check relation cardinality constraints",
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, err := resolveAnalysisScope()
		if err != nil {
			return err
		}

		type cardinalityViolation struct {
			EntityID     string `json:"entity_id"`
			RelationType string `json:"relation_type"`
			Constraint   string `json:"constraint"`
			Required     int    `json:"required"`
			Actual       int    `json:"actual"`
		}
		var allViolations []cardinalityViolation

		for relName, relDef := range meta.Relations {
			// Check min_outgoing constraint
			if relDef.MinOutgoing != nil && *relDef.MinOutgoing > 0 {
				// For each entity type in From, check they have at least MinOutgoing outgoing relations of this type
				for _, sourceType := range relDef.From {
					entities := filterByScope(ws.EntitiesByType(sourceType), scope)
					for _, e := range entities {
						count := 0
						for _, edge := range ws.OutgoingRelations(e.ID) {
							if edge.Type == relName {
								count++
							}
						}
						if count < *relDef.MinOutgoing {
							allViolations = append(allViolations, cardinalityViolation{
								EntityID:     e.ID,
								RelationType: relName,
								Constraint:   "min_outgoing",
								Required:     *relDef.MinOutgoing,
								Actual:       count,
							})
						}
					}
				}
			}

			// Check max_outgoing constraint
			if relDef.MaxOutgoing != nil {
				for _, sourceType := range relDef.From {
					entities := filterByScope(ws.EntitiesByType(sourceType), scope)
					for _, e := range entities {
						count := 0
						for _, edge := range ws.OutgoingRelations(e.ID) {
							if edge.Type == relName {
								count++
							}
						}
						if count > *relDef.MaxOutgoing {
							allViolations = append(allViolations, cardinalityViolation{
								EntityID:     e.ID,
								RelationType: relName,
								Constraint:   "max_outgoing",
								Required:     *relDef.MaxOutgoing,
								Actual:       count,
							})
						}
					}
				}
			}

			// Check min_incoming constraint
			// For each entity type in To, check they have at least MinIncoming incoming relations of this type
			if relDef.MinIncoming != nil && *relDef.MinIncoming > 0 {
				for _, targetType := range relDef.To {
					entities := filterByScope(ws.EntitiesByType(targetType), scope)
					for _, e := range entities {
						count := 0
						for _, edge := range ws.IncomingRelations(e.ID) {
							if edge.Type == relName {
								count++
							}
						}
						if count < *relDef.MinIncoming {
							// Get the inverse relation name for the message if available
							relLabel := relName
							if relDef.Inverse != nil && relDef.Inverse.GetID() != "" {
								relLabel = relDef.Inverse.GetID()
							}
							allViolations = append(allViolations, cardinalityViolation{
								EntityID:     e.ID,
								RelationType: relLabel,
								Constraint:   "min_incoming",
								Required:     *relDef.MinIncoming,
								Actual:       count,
							})
						}
					}
				}
			}

			// Check max_incoming constraint
			if relDef.MaxIncoming != nil {
				for _, targetType := range relDef.To {
					entities := filterByScope(ws.EntitiesByType(targetType), scope)
					for _, e := range entities {
						count := 0
						for _, edge := range ws.IncomingRelations(e.ID) {
							if edge.Type == relName {
								count++
							}
						}
						if count > *relDef.MaxIncoming {
							// Get the inverse relation name for the message if available
							relLabel := relName
							if relDef.Inverse != nil && relDef.Inverse.GetID() != "" {
								relLabel = relDef.Inverse.GetID()
							}
							allViolations = append(allViolations, cardinalityViolation{
								EntityID:     e.ID,
								RelationType: relLabel,
								Constraint:   "max_incoming",
								Required:     *relDef.MaxIncoming,
								Actual:       count,
							})
						}
					}
				}
			}
		}

		if writeAnalysisJSON(len(allViolations), allViolations,
			"All cardinality constraints satisfied", "Found %d cardinality violations") {
			return nil
		}

		for _, v := range allViolations {
			if strings.HasPrefix(v.Constraint, "min_") {
				out.WriteWarning("%s must have at least %d '%s' relation(s), has %d",
					v.EntityID, v.Required, v.RelationType, v.Actual)
			} else {
				out.WriteWarning("%s has more than %d '%s' relation(s): %d",
					v.EntityID, v.Required, v.RelationType, v.Actual)
			}
		}

		if len(allViolations) == 0 {
			out.WriteSuccess("All cardinality constraints satisfied")
		} else {
			out.WriteWarning("Found %d cardinality violations", len(allViolations))
		}

		return nil
	},
}

var analyzePropertiesCmd = &cobra.Command{
	Use:   "properties",
	Short: "Validate entity property values against metamodel",
	Long: `Validates all entity property values against the metamodel schema.

Checks for:
  - Invalid enum values (not in allowed list)
  - Invalid custom type values
  - Invalid date formats
  - Invalid integer/boolean values
  - Missing required properties
  - Entity IDs not matching configured patterns

This catches issues in manually-edited markdown files that bypass CLI validation.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, err := resolveAnalysisScope()
		if err != nil {
			return err
		}
		return runPropertyValidation(scope)
	},
}

// runPropertyValidation validates entity properties against the metamodel.
// If scope is non-nil, only entities in the scope are validated.
func runPropertyValidation(scope map[string]bool) error {
	entities := filterByScope(ws.AllEntities(), scope)
	errorCount := 0

	// Group errors by entity for cleaner output
	type entityErrors struct {
		entity *model.Entity
		errs   []*metamodel.ValidationError
	}
	var allErrors []entityErrors

	for _, entity := range entities {
		errs := meta.ValidateEntity(entity)
		if len(errs) > 0 {
			allErrors = append(allErrors, entityErrors{entity: entity, errs: errs})
			errorCount += len(errs)
		}
	}

	// Handle JSON output format
	if out.Format == "json" {
		var results []output.PropertyValidationResult
		for _, ee := range allErrors {
			errStrings := make([]string, len(ee.errs))
			for i, err := range ee.errs {
				errStrings[i] = err.Message
			}
			results = append(results, output.PropertyValidationResult{
				EntityID:   ee.entity.ID,
				EntityType: ee.entity.Type,
				Errors:     errStrings,
			})
		}

		status := "success"
		message := "All entity properties are valid"
		if errorCount > 0 {
			status = "error"
			message = fmt.Sprintf("Found %d property errors across %d entities", errorCount, len(allErrors))
		}

		return out.WriteAnalysisResult(output.AnalysisResult{
			Status:  status,
			Message: message,
			Count:   errorCount,
			Details: results,
		})
	}

	// Text output format
	if errorCount == 0 {
		out.WriteSuccess("All entity properties are valid")
		return nil
	}

	out.WriteError("Found %d property errors across %d entities:", errorCount, len(allErrors))
	for _, ee := range allErrors {
		out.WriteMessage("")
		out.WriteMessage("  %s (%s):", ee.entity.ID, ee.entity.Type)
		for _, err := range ee.errs {
			out.WriteMessage("    - %s", err.Error())
		}
	}

	return nil
}

// countPropertyErrors counts property validation errors across entities.
// If scope is non-nil, only entities in the scope are counted.
func countPropertyErrors(scope map[string]bool) int {
	count := 0
	for _, entity := range filterByScope(ws.AllEntities(), scope) {
		count += len(meta.ValidateEntity(entity))
	}
	return count
}

var analyzeValidationsCmd = &cobra.Command{
	Use:   "validations",
	Short: "Run custom validation rules from metamodel",
	Long: `Runs custom validation rules defined in the metamodel's 'validations' section.

Each validation rule can:
  - Target a specific entity type (or all types if not specified)
  - Use 'when' conditions to select which entities the rule applies to
  - Use 'then' conditions that matched entities must satisfy
  - Have a severity of 'error' or 'warning'

Example metamodel configuration:
  validations:
    - name: accepted-needs-priority
      description: "Accepted requirements must have priority"
      entity_type: requirement
      when:
        - "status=accepted"
      then:
        - "priority!="
      severity: error`,
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, err := resolveAnalysisScope()
		if err != nil {
			return err
		}
		return runValidations(scope)
	},
}

// runValidations executes custom validation rules and returns error/warning counts
// validationViolation represents a single validation rule violation for JSON output
type validationViolation struct {
	RuleName    string `json:"rule_name"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	EntityID    string `json:"entity_id"`
	EntityTitle string `json:"entity_title"`
}

// collectValidationViolations collects all validation violations and counts.
// If scope is non-nil, only violations for entities in the scope are collected.
func collectValidationViolations(
	rules []metamodel.ValidationRule,
	scope map[string]bool,
) (violations []validationViolation, errorCount, warningCount int) {
	for _, rule := range rules {
		ruleViolations := checkValidationRule(rule, scope)
		severity := rule.GetSeverity()
		for _, v := range ruleViolations {
			violations = append(violations, validationViolation{
				RuleName:    rule.Name,
				Description: rule.Description,
				Severity:    severity,
				EntityID:    v.ID,
				EntityTitle: v.Title(),
			})
			if severity == "error" {
				errorCount++
			} else {
				warningCount++
			}
		}
	}
	return violations, errorCount, warningCount
}

// writeValidationsTextOutput writes validation results in text format.
// If scope is non-nil, only violations for entities in the scope are shown.
func writeValidationsTextOutput(
	rules []metamodel.ValidationRule,
	scope map[string]bool,
	errorCount, warningCount int,
) {
	// Group violations by rule
	ruleViolations := make(map[string][]*model.Entity)
	for _, rule := range rules {
		violations := checkValidationRule(rule, scope)
		if len(violations) > 0 {
			ruleViolations[rule.Name] = violations
		}
	}

	for _, rule := range rules {
		violations := ruleViolations[rule.Name]
		if len(violations) > 0 {
			if rule.GetSeverity() == "error" {
				out.WriteError("%s (%d):", rule.Description, len(violations))
			} else {
				out.WriteWarning("%s (%d):", rule.Description, len(violations))
			}
			for _, v := range violations {
				out.WriteMessage("  %s: %s", v.ID, v.Title())
			}
		}
	}

	if errorCount == 0 && warningCount == 0 {
		out.WriteSuccess("All %d validation rules passed", len(rules))
		return
	}
	if errorCount > 0 {
		out.WriteError("Found %d errors, %d warnings across %d rules", errorCount, warningCount, len(rules))
	} else {
		out.WriteWarning("Found %d warnings across %d rules", warningCount, len(rules))
	}
}

// runValidations executes custom validation rules and returns error/warning counts.
// If scope is non-nil, only entities in the scope are validated.
func runValidations(scope map[string]bool) error {
	rules := meta.Validations
	if len(rules) == 0 {
		if out.Format == "json" {
			return out.WriteAnalysisResult(output.AnalysisResult{
				Status:  "success",
				Message: "No custom validation rules defined in metamodel",
				Count:   0,
				Details: []interface{}{},
			})
		}
		out.WriteSuccess("No custom validation rules defined in metamodel")
		return nil
	}

	allViolations, errorCount, warningCount := collectValidationViolations(rules, scope)

	if out.Format == "json" {
		status := "success"
		message := fmt.Sprintf("All %d validation rules passed", len(rules))
		if errorCount > 0 {
			status = "error"
			message = fmt.Sprintf("Found %d errors, %d warnings across %d rules",
				errorCount, warningCount, len(rules))
		} else if warningCount > 0 {
			status = "warning"
			message = fmt.Sprintf("Found %d warnings across %d rules", warningCount, len(rules))
		}
		return out.WriteAnalysisResult(output.AnalysisResult{
			Status:  status,
			Message: message,
			Count:   errorCount + warningCount,
			Details: allViolations,
		})
	}

	writeValidationsTextOutput(rules, scope, errorCount, warningCount)
	return nil
}

// checkValidationRule checks a single validation rule against applicable entities.
// If scope is non-nil, only entities in the scope are checked.
func checkValidationRule(rule metamodel.ValidationRule, scope map[string]bool) []*model.Entity {
	var violations []*model.Entity

	// Parse when filters (conditions that select which entities to check)
	whenFilters, err := filter.ParseAll(rule.When)
	if err != nil {
		out.WriteError("Invalid 'when' filter in rule %q: %v", rule.Name, err)
		return nil
	}

	// Parse then filters (conditions that matched entities must satisfy)
	thenFilters, err := filter.ParseAll(rule.Then)
	if err != nil {
		out.WriteError("Invalid 'then' filter in rule %q: %v", rule.Name, err)
		return nil
	}

	// Get entities to check
	var entities []*model.Entity
	if rule.EntityType != "" {
		entities = ws.EntitiesByType(rule.EntityType)
	} else {
		entities = ws.AllEntities()
	}
	entities = filterByScope(entities, scope)

	for _, entity := range entities {
		// Get entity definition
		entityDef, ok := meta.GetEntityDef(entity.Type)
		if !ok {
			continue
		}

		// Check if entity matches the 'when' conditions
		if len(whenFilters) > 0 {
			matches, err := filter.MatchAll(entity, whenFilters, entityDef, meta)
			if err != nil {
				// Skip entities where filter can't be evaluated (e.g., missing property)
				continue
			}
			if !matches {
				// Entity doesn't match the conditions, rule doesn't apply
				continue
			}
		}

		// Entity matches - now check if it satisfies the 'then' conditions
		if len(thenFilters) > 0 {
			satisfies, err := filter.MatchAll(entity, thenFilters, entityDef, meta)
			if err != nil {
				// If we can't evaluate the then filter, treat as violation
				violations = append(violations, entity)
				continue
			}

			if !satisfies {
				violations = append(violations, entity)
				continue
			}
		}

		// Check content rules
		if rule.Content != nil {
			if !markdown.CheckContentRule(entity, rule.Content) {
				violations = append(violations, entity)
			}
		}
	}

	return violations
}

// countValidationIssues counts errors and warnings from validation rules.
// If scope is non-nil, only entities in the scope are counted.
func countValidationIssues(scope map[string]bool) (errors, warnings int) {
	for _, rule := range meta.Validations {
		violations := checkValidationRule(rule, scope)
		if rule.IsError() {
			errors += len(violations)
		} else {
			warnings += len(violations)
		}
	}
	return
}

// countCardinalityViolations counts cardinality constraint violations.
// If scope is non-nil, only entities in the scope are counted.
func countCardinalityViolations(scope map[string]bool) int {
	violations := 0
	for relName, relDef := range meta.Relations {
		violations += countMinOutgoingViolations(relName, relDef, scope)
		violations += countMaxOutgoingViolations(relName, relDef, scope)
		violations += countMinIncomingViolations(relName, relDef, scope)
		violations += countMaxIncomingViolations(relName, relDef, scope)
	}
	return violations
}

// countMinOutgoingViolations checks min_outgoing constraint violations.
// If scope is non-nil, only entities in the scope are counted.
func countMinOutgoingViolations(relName string, relDef metamodel.RelationDef, scope map[string]bool) int {
	if relDef.MinOutgoing == nil || *relDef.MinOutgoing == 0 {
		return 0
	}
	violations := 0
	for _, sourceType := range relDef.From {
		for _, e := range filterByScope(ws.EntitiesByType(sourceType), scope) {
			if countOutgoingByType(e.ID, relName) < *relDef.MinOutgoing {
				violations++
			}
		}
	}
	return violations
}

// countMaxOutgoingViolations checks max_outgoing constraint violations.
// If scope is non-nil, only entities in the scope are counted.
func countMaxOutgoingViolations(relName string, relDef metamodel.RelationDef, scope map[string]bool) int {
	if relDef.MaxOutgoing == nil {
		return 0
	}
	violations := 0
	for _, sourceType := range relDef.From {
		for _, e := range filterByScope(ws.EntitiesByType(sourceType), scope) {
			if countOutgoingByType(e.ID, relName) > *relDef.MaxOutgoing {
				violations++
			}
		}
	}
	return violations
}

// countMinIncomingViolations checks min_incoming constraint violations.
// If scope is non-nil, only entities in the scope are counted.
func countMinIncomingViolations(relName string, relDef metamodel.RelationDef, scope map[string]bool) int {
	if relDef.MinIncoming == nil || *relDef.MinIncoming == 0 {
		return 0
	}
	violations := 0
	for _, targetType := range relDef.To {
		for _, e := range filterByScope(ws.EntitiesByType(targetType), scope) {
			if countIncomingByType(e.ID, relName) < *relDef.MinIncoming {
				violations++
			}
		}
	}
	return violations
}

// countMaxIncomingViolations checks max_incoming constraint violations.
// If scope is non-nil, only entities in the scope are counted.
func countMaxIncomingViolations(relName string, relDef metamodel.RelationDef, scope map[string]bool) int {
	if relDef.MaxIncoming == nil {
		return 0
	}
	violations := 0
	for _, targetType := range relDef.To {
		for _, e := range filterByScope(ws.EntitiesByType(targetType), scope) {
			if countIncomingByType(e.ID, relName) > *relDef.MaxIncoming {
				violations++
			}
		}
	}
	return violations
}

// countOutgoingByType counts outgoing edges of a specific relation type
func countOutgoingByType(entityID, relName string) int {
	count := 0
	for _, edge := range ws.OutgoingRelations(entityID) {
		if edge.Type == relName {
			count++
		}
	}
	return count
}

// countIncomingByType counts incoming edges of a specific relation type
func countIncomingByType(entityID, relName string) int {
	count := 0
	for _, edge := range ws.IncomingRelations(entityID) {
		if edge.Type == relName {
			count++
		}
	}
	return count
}

var analyzeAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Run all analyses",
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, err := resolveAnalysisScope()
		if err != nil {
			return err
		}

		// Collect issue counts for summary
		orphanCount := len(filterByScope(ws.FindOrphans(), scope))

		// Count cardinality violations
		cardinalityCount := countCardinalityViolations(scope)

		// Count duplicates
		entities := filterByScope(ws.AllEntities(), scope)
		titleGroups := make(map[string][]*model.Entity)
		for _, e := range entities {
			title := normalizeTitle(e.Title())
			if title != "" {
				titleGroups[title] = append(titleGroups[title], e)
			}
		}
		duplicateCount := 0
		for _, group := range titleGroups {
			if len(group) > 1 {
				duplicateCount++
			}
		}

		// Count gap sequences with gaps (only for auto ID types)
		gapCount := 0
		stringIDPrefixes := make(map[string]bool)
		for _, entityDef := range meta.Entities {
			if entityDef.IsManualID() {
				for _, idPrefix := range entityDef.GetIDPrefixes() {
					prefix := strings.TrimSuffix(idPrefix, "-")
					stringIDPrefixes[prefix] = true
				}
			}
		}
		prefixGroups := make(map[string][]int)
		for _, id := range ws.EntityIDs() {
			// Filter by scope
			if !inScope(id, scope) {
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
		for _, numbers := range prefixGroups {
			sort.Ints(numbers)
			for i := 1; i < len(numbers); i++ {
				if numbers[i] != numbers[i-1]+1 {
					gapCount++
					break
				}
			}
		}

		// Count property errors
		propertyErrorCount := countPropertyErrors(scope)

		// Count validation issues
		validationErrors, validationWarnings := countValidationIssues(scope)

		// Handle JSON output format
		if out.Format == "json" {
			type allAnalysisSummary struct {
				Orphans            int `json:"orphans"`
				Cardinality        int `json:"cardinality"`
				Duplicates         int `json:"duplicates"`
				Gaps               int `json:"gaps"`
				Properties         int `json:"properties"`
				ValidationErrors   int `json:"validation_errors"`
				ValidationWarnings int `json:"validation_warnings"`
			}

			summary := allAnalysisSummary{
				Orphans:            orphanCount,
				Cardinality:        cardinalityCount,
				Duplicates:         duplicateCount,
				Gaps:               gapCount,
				Properties:         propertyErrorCount,
				ValidationErrors:   validationErrors,
				ValidationWarnings: validationWarnings,
			}

			totalIssues := orphanCount + cardinalityCount + duplicateCount +
				gapCount + propertyErrorCount + validationErrors
			status := "success"
			message := "All analyses passed"
			if validationErrors > 0 || propertyErrorCount > 0 {
				status = "error"
				message = fmt.Sprintf("Found %d issues requiring attention", totalIssues)
			} else if totalIssues > 0 || validationWarnings > 0 {
				status = "warning"
				message = fmt.Sprintf("Found %d issues and %d warnings", totalIssues, validationWarnings)
			}

			return out.WriteAnalysisResult(output.AnalysisResult{
				Status:  status,
				Message: message,
				Count:   totalIssues + validationWarnings,
				Details: summary,
			})
		}

		// Text output format
		summaryItems := []string{
			fmt.Sprintf("Orphans: %d", orphanCount),
			fmt.Sprintf("Cardinality: %d", cardinalityCount),
			fmt.Sprintf("Duplicates: %d", duplicateCount),
			fmt.Sprintf("Gaps: %d", gapCount),
			fmt.Sprintf("Properties: %d", propertyErrorCount),
		}
		if len(meta.Validations) > 0 {
			summaryItems = append(summaryItems, fmt.Sprintf("Validation Errors: %d", validationErrors))
			summaryItems = append(summaryItems, fmt.Sprintf("Validation Warnings: %d", validationWarnings))
		}
		out.WriteSummaryBox(summaryItems)
		out.WriteMessage("")

		var errs []error

		out.WriteSectionHeader("Orphan Analysis")
		if err := analyzeOrphansCmd.RunE(cmd, args); err != nil {
			errs = append(errs, fmt.Errorf("orphan analysis: %w", err))
		}

		out.WriteMessage("")
		out.WriteSectionHeader("Duplicate Analysis")
		if err := analyzeDuplicatesCmd.RunE(cmd, args); err != nil {
			errs = append(errs, fmt.Errorf("duplicate analysis: %w", err))
		}

		out.WriteMessage("")
		out.WriteSectionHeader("ID Gap Analysis")
		if err := analyzeGapsCmd.RunE(cmd, args); err != nil {
			errs = append(errs, fmt.Errorf("gap analysis: %w", err))
		}

		out.WriteMessage("")
		out.WriteSectionHeader("Cardinality Analysis")
		if err := analyzeCardinalityCmd.RunE(cmd, args); err != nil {
			errs = append(errs, fmt.Errorf("cardinality analysis: %w", err))
		}

		out.WriteMessage("")
		out.WriteSectionHeader("Property Validation")
		if err := runPropertyValidation(scope); err != nil {
			errs = append(errs, fmt.Errorf("property validation: %w", err))
		}

		if len(meta.Validations) > 0 {
			out.WriteMessage("")
			out.WriteSectionHeader("Custom Validations")
			if err := runValidations(scope); err != nil {
				errs = append(errs, fmt.Errorf("custom validations: %w", err))
			}
		}

		if len(errs) > 0 {
			// Return first error (all have been logged via their respective output)
			return errs[0]
		}

		return nil
	},
}

func normalizeTitle(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	// Remove extra whitespace
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

func init() {
	// Scope flags (inherited by all subcommands)
	analyzeCmd.PersistentFlags().StringVar(&analyzeViewName, "view", "",
		"Scope analysis to entities in a view")
	analyzeCmd.PersistentFlags().StringVar(&analyzeEntryID, "entry", "",
		"Entry entity ID for the view (required with --view)")

	analyzeCmd.AddCommand(analyzeOrphansCmd)
	analyzeCmd.AddCommand(analyzeDuplicatesCmd)
	analyzeCmd.AddCommand(analyzeGapsCmd)
	analyzeCmd.AddCommand(analyzeCardinalityCmd)
	analyzeCmd.AddCommand(analyzePropertiesCmd)
	analyzeCmd.AddCommand(analyzeValidationsCmd)
	analyzeCmd.AddCommand(analyzeAllCmd)

	rootCmd.AddCommand(analyzeCmd)
}

// resolveAnalysisScope resolves the --view and --entry flags to a set of entity IDs.
// Returns nil scope if no view is specified (analyze full graph).
// The result is cached to avoid re-executing the view when called from multiple subcommands.
func resolveAnalysisScope() (map[string]bool, error) {
	// Return cached result if already resolved
	if cachedScopeOnce {
		return cachedScope, nil
	}

	scope, err := doResolveAnalysisScope()
	if err != nil {
		return nil, err
	}

	// Cache the result
	cachedScope = scope
	cachedScopeOnce = true
	return scope, nil
}

// resetAnalysisScopeCache clears the cached scope. Called at the start of analyze commands.
func resetAnalysisScopeCache() {
	cachedScope = nil
	cachedScopeOnce = false
}

// doResolveAnalysisScope performs the actual scope resolution.
func doResolveAnalysisScope() (map[string]bool, error) {
	if analyzeViewName == "" {
		return nil, nil //nolint:nilnil // nil scope means no filtering
	}

	if analyzeEntryID == "" {
		return nil, fmt.Errorf("--entry is required when using --view")
	}

	// Load views file
	viewsFile, err := ws.LoadViews()
	if err != nil {
		return nil, fmt.Errorf("failed to load views: %w", err)
	}

	// Get the view definition
	viewDef, ok := viewsFile.GetView(analyzeViewName)
	if !ok {
		return nil, fmt.Errorf("view not found: %s", analyzeViewName)
	}

	// Validate the view against the metamodel
	if validationErr := viewDef.Validate(meta, analyzeViewName); validationErr != nil {
		return nil, fmt.Errorf("view validation failed: %w", validationErr)
	}

	// Create view engine and execute
	engine := views.NewEngine(ws.Graph(), meta)
	result, err := engine.Execute(viewDef, analyzeEntryID)
	if err != nil {
		return nil, fmt.Errorf("view execution failed: %w", err)
	}

	return result.EntityIDs(), nil
}

// filterByScope filters entities to only those in the scope.
// If scope is nil, returns the original slice unchanged (no copy made).
// If scope is non-nil, returns a new slice containing only entities whose IDs are in the scope.
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
