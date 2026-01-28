package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/output"
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

var analyzeOrphansCmd = &cobra.Command{
	Use:   "orphans",
	Short: "Find entities with no connections",
	RunE: func(cmd *cobra.Command, args []string) error {
		orphans := g.FindOrphans()

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
		entities := g.AllNodes()

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
		// Build a set of prefixes that belong to string ID types (should be skipped)
		stringIDPrefixes := make(map[string]bool)
		for _, entityDef := range meta.Entities {
			if entityDef.IsStringID() {
				for _, idPrefix := range entityDef.GetIDPrefixes() {
					// Normalize prefix (remove trailing dash if present)
					prefix := strings.TrimSuffix(idPrefix, "-")
					stringIDPrefixes[prefix] = true
				}
			}
		}

		// Group IDs by prefix (only for sequential ID types)
		prefixGroups := make(map[string][]int)

		for _, id := range g.AllIDs() {
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

		hasGaps := false

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
				hasGaps = true
				out.WriteWarning("Gaps in %s sequence:", prefix)
				gapStrs := make([]string, len(gaps))
				for i, n := range gaps {
					gapStrs[i] = fmt.Sprintf("%s%03d", prefix, n)
				}
				out.WriteMessage("  Missing: %s", strings.Join(gapStrs, ", "))
			}
		}

		if !hasGaps {
			out.WriteSuccess("No ID sequence gaps found")
		}

		return nil
	},
}

var analyzeCardinalityCmd = &cobra.Command{
	Use:   "cardinality",
	Short: "Check relation cardinality constraints",
	RunE: func(cmd *cobra.Command, args []string) error {
		violations := 0

		for relName, relDef := range meta.Relations {
			// Check source_min constraint
			if relDef.SourceMin != nil && *relDef.SourceMin > 0 {
				// For each entity type in From, check they have at least SourceMin outgoing relations
				for _, sourceType := range relDef.From {
					entities := g.NodesByType(sourceType)
					for _, e := range entities {
						count := 0
						for _, edge := range g.OutgoingEdges(e.ID) {
							if edge.Type == relName {
								count++
							}
						}
						if count < *relDef.SourceMin {
							out.WriteWarning("%s must have at least %d '%s' relation(s), has %d",
								e.ID, *relDef.SourceMin, relName, count)
							violations++
						}
					}
				}
			}

			// Check source_max constraint
			if relDef.SourceMax != nil {
				for _, sourceType := range relDef.From {
					entities := g.NodesByType(sourceType)
					for _, e := range entities {
						count := 0
						for _, edge := range g.OutgoingEdges(e.ID) {
							if edge.Type == relName {
								count++
							}
						}
						if count > *relDef.SourceMax {
							out.WriteWarning("%s has more than %d '%s' relation(s): %d",
								e.ID, *relDef.SourceMax, relName, count)
							violations++
						}
					}
				}
			}

			// Check target_min constraint
			// For each entity type in To, check they have at least TargetMin incoming relations of this type
			if relDef.TargetMin != nil && *relDef.TargetMin > 0 {
				for _, targetType := range relDef.To {
					entities := g.NodesByType(targetType)
					for _, e := range entities {
						count := 0
						for _, edge := range g.IncomingEdges(e.ID) {
							if edge.Type == relName {
								count++
							}
						}
						if count < *relDef.TargetMin {
							// Get the inverse relation name for the message if available
							relLabel := relName
							if relDef.Inverse != nil && relDef.Inverse.Name != "" {
								relLabel = relDef.Inverse.Name
							}
							out.WriteWarning("%s must have at least %d '%s' relation(s), has %d",
								e.ID, *relDef.TargetMin, relLabel, count)
							violations++
						}
					}
				}
			}

			// Check target_max constraint
			if relDef.TargetMax != nil {
				for _, targetType := range relDef.To {
					entities := g.NodesByType(targetType)
					for _, e := range entities {
						count := 0
						for _, edge := range g.IncomingEdges(e.ID) {
							if edge.Type == relName {
								count++
							}
						}
						if count > *relDef.TargetMax {
							// Get the inverse relation name for the message if available
							relLabel := relName
							if relDef.Inverse != nil && relDef.Inverse.Name != "" {
								relLabel = relDef.Inverse.Name
							}
							out.WriteWarning("%s has more than %d '%s' relation(s): %d",
								e.ID, *relDef.TargetMax, relLabel, count)
							violations++
						}
					}
				}
			}
		}

		if violations == 0 {
			out.WriteSuccess("All cardinality constraints satisfied")
		} else {
			out.WriteWarning("Found %d cardinality violations", violations)
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
		return runPropertyValidation()
	},
}

// runPropertyValidation validates all entity properties against the metamodel
func runPropertyValidation() error {
	entities := g.AllNodes()
	errorCount := 0

	// Group errors by entity for cleaner output
	type entityErrors struct {
		entity *model.Entity
		errs   []error
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
				errStrings[i] = err.Error()
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

// countPropertyErrors counts property validation errors across all entities
func countPropertyErrors() int {
	count := 0
	for _, entity := range g.AllNodes() {
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
  - Match entities using filter conditions (same syntax as --where)
  - Require matched entities to satisfy additional conditions
  - Have a severity of 'error' or 'warning'

Example metamodel configuration:
  validations:
    - name: accepted-needs-priority
      description: "Accepted requirements must have priority"
      entity_type: requirement
      match:
        - "status=accepted"
      require:
        - "priority!="
      severity: error`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runValidations()
	},
}

// runValidations executes custom validation rules and returns error/warning counts
func runValidations() error {
	rules := meta.Validations
	if len(rules) == 0 {
		out.WriteSuccess("No custom validation rules defined in metamodel")
		return nil
	}

	errorCount := 0
	warningCount := 0

	for _, rule := range rules {
		violations := checkValidationRule(rule)
		if len(violations) > 0 {
			// Determine severity indicator
			severity := rule.GetSeverity()
			if severity == "error" {
				errorCount += len(violations)
				out.WriteError("%s (%d):", rule.Description, len(violations))
			} else {
				warningCount += len(violations)
				out.WriteWarning("%s (%d):", rule.Description, len(violations))
			}

			for _, v := range violations {
				out.WriteMessage("  %s: %s", v.ID, v.Title())
			}
		}
	}

	if errorCount == 0 && warningCount == 0 {
		out.WriteSuccess("All %d validation rules passed", len(rules))
	} else {
		if errorCount > 0 {
			out.WriteError("Found %d errors, %d warnings across %d rules", errorCount, warningCount, len(rules))
		} else {
			out.WriteWarning("Found %d warnings across %d rules", warningCount, len(rules))
		}
	}

	return nil
}

// checkValidationRule checks a single validation rule against all applicable entities
func checkValidationRule(rule metamodel.ValidationRule) []*model.Entity {
	var violations []*model.Entity

	// Parse match filters
	matchFilters, err := filter.ParseAll(rule.Match)
	if err != nil {
		out.WriteError("Invalid match filter in rule %q: %v", rule.Name, err)
		return nil
	}

	// Parse require filters
	requireFilters, err := filter.ParseAll(rule.Require)
	if err != nil {
		out.WriteError("Invalid require filter in rule %q: %v", rule.Name, err)
		return nil
	}

	// Get entities to check
	var entities []*model.Entity
	if rule.EntityType != "" {
		entities = g.NodesByType(rule.EntityType)
	} else {
		entities = g.AllNodes()
	}

	for _, entity := range entities {
		// Get entity definition
		entityDef, ok := meta.GetEntityDef(entity.Type)
		if !ok {
			continue
		}

		// Check if entity matches the 'match' conditions
		if len(matchFilters) > 0 {
			matches, err := filter.MatchAll(entity, matchFilters, entityDef, meta)
			if err != nil {
				// Skip entities where filter can't be evaluated (e.g., missing property)
				continue
			}
			if !matches {
				// Entity doesn't match the conditions, rule doesn't apply
				continue
			}
		}

		// Entity matches - now check if it satisfies the 'require' conditions
		satisfies, err := filter.MatchAll(entity, requireFilters, entityDef, meta)
		if err != nil {
			// If we can't evaluate the require filter, treat as violation
			violations = append(violations, entity)
			continue
		}

		if !satisfies {
			violations = append(violations, entity)
		}
	}

	return violations
}

// countValidationIssues counts errors and warnings from validation rules
func countValidationIssues() (errors, warnings int) {
	for _, rule := range meta.Validations {
		violations := checkValidationRule(rule)
		if rule.IsError() {
			errors += len(violations)
		} else {
			warnings += len(violations)
		}
	}
	return
}

// countCardinalityViolations counts cardinality constraint violations
func countCardinalityViolations() int {
	violations := 0
	for relName, relDef := range meta.Relations {
		violations += countSourceMinViolations(relName, relDef)
		violations += countSourceMaxViolations(relName, relDef)
		violations += countTargetMinViolations(relName, relDef)
		violations += countTargetMaxViolations(relName, relDef)
	}
	return violations
}

// countSourceMinViolations checks source_min constraint violations
func countSourceMinViolations(relName string, relDef metamodel.RelationDef) int {
	if relDef.SourceMin == nil || *relDef.SourceMin == 0 {
		return 0
	}
	violations := 0
	for _, sourceType := range relDef.From {
		for _, e := range g.NodesByType(sourceType) {
			if countOutgoingByType(e.ID, relName) < *relDef.SourceMin {
				violations++
			}
		}
	}
	return violations
}

// countSourceMaxViolations checks source_max constraint violations
func countSourceMaxViolations(relName string, relDef metamodel.RelationDef) int {
	if relDef.SourceMax == nil {
		return 0
	}
	violations := 0
	for _, sourceType := range relDef.From {
		for _, e := range g.NodesByType(sourceType) {
			if countOutgoingByType(e.ID, relName) > *relDef.SourceMax {
				violations++
			}
		}
	}
	return violations
}

// countTargetMinViolations checks target_min constraint violations
func countTargetMinViolations(relName string, relDef metamodel.RelationDef) int {
	if relDef.TargetMin == nil || *relDef.TargetMin == 0 {
		return 0
	}
	violations := 0
	for _, targetType := range relDef.To {
		for _, e := range g.NodesByType(targetType) {
			if countIncomingByType(e.ID, relName) < *relDef.TargetMin {
				violations++
			}
		}
	}
	return violations
}

// countTargetMaxViolations checks target_max constraint violations
func countTargetMaxViolations(relName string, relDef metamodel.RelationDef) int {
	if relDef.TargetMax == nil {
		return 0
	}
	violations := 0
	for _, targetType := range relDef.To {
		for _, e := range g.NodesByType(targetType) {
			if countIncomingByType(e.ID, relName) > *relDef.TargetMax {
				violations++
			}
		}
	}
	return violations
}

// countOutgoingByType counts outgoing edges of a specific relation type
func countOutgoingByType(entityID, relName string) int {
	count := 0
	for _, edge := range g.OutgoingEdges(entityID) {
		if edge.Type == relName {
			count++
		}
	}
	return count
}

// countIncomingByType counts incoming edges of a specific relation type
func countIncomingByType(entityID, relName string) int {
	count := 0
	for _, edge := range g.IncomingEdges(entityID) {
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
		// Collect issue counts for summary
		orphanCount := len(g.FindOrphans())

		// Count cardinality violations
		cardinalityCount := countCardinalityViolations()

		// Count duplicates
		entities := g.AllNodes()
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

		// Count gap sequences with gaps (only for sequential ID types)
		gapCount := 0
		stringIDPrefixes := make(map[string]bool)
		for _, entityDef := range meta.Entities {
			if entityDef.IsStringID() {
				for _, idPrefix := range entityDef.GetIDPrefixes() {
					prefix := strings.TrimSuffix(idPrefix, "-")
					stringIDPrefixes[prefix] = true
				}
			}
		}
		prefixGroups := make(map[string][]int)
		for _, id := range g.AllIDs() {
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
		propertyErrorCount := countPropertyErrors()

		// Count validation issues
		validationErrors, validationWarnings := countValidationIssues()

		// Summary box
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

		out.WriteSectionHeader("Orphan Analysis")
		_ = analyzeOrphansCmd.RunE(cmd, args)

		out.WriteMessage("")
		out.WriteSectionHeader("Duplicate Analysis")
		_ = analyzeDuplicatesCmd.RunE(cmd, args)

		out.WriteMessage("")
		out.WriteSectionHeader("ID Gap Analysis")
		_ = analyzeGapsCmd.RunE(cmd, args)

		out.WriteMessage("")
		out.WriteSectionHeader("Cardinality Analysis")
		_ = analyzeCardinalityCmd.RunE(cmd, args)

		out.WriteMessage("")
		out.WriteSectionHeader("Property Validation")
		_ = runPropertyValidation()

		if len(meta.Validations) > 0 {
			out.WriteMessage("")
			out.WriteSectionHeader("Custom Validations")
			_ = runValidations()
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
	analyzeCmd.AddCommand(analyzeOrphansCmd)
	analyzeCmd.AddCommand(analyzeDuplicatesCmd)
	analyzeCmd.AddCommand(analyzeGapsCmd)
	analyzeCmd.AddCommand(analyzeCardinalityCmd)
	analyzeCmd.AddCommand(analyzePropertiesCmd)
	analyzeCmd.AddCommand(analyzeValidationsCmd)
	analyzeCmd.AddCommand(analyzeAllCmd)

	rootCmd.AddCommand(analyzeCmd)
}
