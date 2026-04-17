package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/schema"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

var (
	// Schema analysis flags
	schemaThreshold int
	schemaCleanup   bool
	schemaDryRun    bool
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
  schema      - Analyze metamodel schema usage (find unused types)
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
		opts, err := resolveAnalyzeOpts()
		if err != nil {
			return err
		}

		orphans := ws.FindOrphansWithScope(*opts)
		filter.SortByID(orphans, modelEntityRecord, false)

		if writeAnalysisJSON(len(orphans), orphans,
			"No orphan entities found", "Found %d orphan entities") {
			return nil
		}

		if len(orphans) == 0 {
			out.WriteSuccess("No orphan entities found")
			return nil
		}
		out.WriteWarning("Found %d orphan entities:", len(orphans))
		return out.WriteEntities(modelToEntitySlice(orphans))
	},
}

var analyzeDuplicatesCmd = &cobra.Command{
	Use:   "duplicates",
	Short: "Find entities with similar titles",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := resolveAnalyzeOpts()
		if err != nil {
			return err
		}

		duplicates := ws.FindDuplicates(*opts)

		// Handle JSON output format
		if out.Format == "json" {
			type duplicateGroup struct {
				Title    string           `json:"title"`
				Entities []*entity.Entity `json:"entities"`
			}
			var details []duplicateGroup
			for _, group := range duplicates {
				details = append(details, duplicateGroup{
					Title:    group.Title,
					Entities: modelToEntitySlice(group.Entities),
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
			out.WriteMessage("  Title: %s", group.Title)
			for _, e := range group.Entities {
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
		opts, err := resolveAnalyzeOpts()
		if err != nil {
			return err
		}

		allGaps := ws.FindGaps(*opts)

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
		opts, err := resolveAnalyzeOpts()
		if err != nil {
			return err
		}

		violations := ws.CheckCardinality(*opts)

		if writeAnalysisJSON(len(violations), violations,
			"All cardinality constraints satisfied", "Found %d cardinality violations") {
			return nil
		}

		for _, v := range violations {
			if strings.HasPrefix(v.Constraint, "min_") {
				out.WriteWarning("%s must have at least %d '%s' relation(s), has %d",
					v.EntityID, v.Required, v.RelationType, v.Actual)
			} else {
				out.WriteWarning("%s has more than %d '%s' relation(s): %d",
					v.EntityID, v.Required, v.RelationType, v.Actual)
			}
		}

		if len(violations) == 0 {
			out.WriteSuccess("All cardinality constraints satisfied")
		} else {
			out.WriteWarning("Found %d cardinality violations", len(violations))
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
		opts, err := resolveAnalyzeOpts()
		if err != nil {
			return err
		}
		return runPropertyValidation(*opts)
	},
}

// runPropertyValidation validates entity and relation properties against the metamodel.
func runPropertyValidation(opts workspace.AnalyzeOptions) error {
	allEntityErrors := ws.ValidateProperties(opts)
	allRelationErrors := ws.ValidateRelationProperties()

	errorCount := 0
	for _, ee := range allEntityErrors {
		errorCount += len(ee.Errors)
	}
	for _, re := range allRelationErrors {
		errorCount += len(re.Errors)
	}

	if out.Format == "json" {
		return writePropertyValidationJSON(allEntityErrors, allRelationErrors, errorCount)
	}

	return writePropertyValidationText(allEntityErrors, allRelationErrors, errorCount)
}

func writePropertyValidationJSON(
	allEntityErrors []workspace.PropertyError,
	allRelationErrors []workspace.RelationPropertyError,
	errorCount int,
) error {
	entityResults := make([]output.PropertyValidationResult, 0, len(allEntityErrors))
	for _, ee := range allEntityErrors {
		errStrings := make([]string, len(ee.Errors))
		for i, err := range ee.Errors {
			errStrings[i] = err.Message
		}
		entityResults = append(entityResults, output.PropertyValidationResult{
			EntityID:   ee.EntityID,
			EntityType: ee.EntityType,
			Errors:     errStrings,
		})
	}

	relationResults := make([]output.RelationPropertyValidationResult, 0, len(allRelationErrors))
	for _, re := range allRelationErrors {
		errStrings := make([]string, len(re.Errors))
		for i, err := range re.Errors {
			errStrings[i] = err.Message
		}
		relationResults = append(relationResults, output.RelationPropertyValidationResult{
			RelationKey:  re.RelationKey,
			RelationType: re.RelationType,
			Errors:       errStrings,
		})
	}

	status := "success"
	message := "All entity and relation properties are valid"
	if errorCount > 0 {
		status = "error"
		message = fmt.Sprintf("Found %d property errors across %d entities and %d relations",
			errorCount, len(allEntityErrors), len(allRelationErrors))
	}

	details := make(map[string]interface{})
	if len(entityResults) > 0 {
		details["entities"] = entityResults
	}
	if len(relationResults) > 0 {
		details["relations"] = relationResults
	}

	return out.WriteAnalysisResult(output.AnalysisResult{
		Status:  status,
		Message: message,
		Count:   errorCount,
		Details: details,
	})
}

func writePropertyValidationText(
	allEntityErrors []workspace.PropertyError,
	allRelationErrors []workspace.RelationPropertyError,
	errorCount int,
) error {
	if errorCount == 0 {
		out.WriteSuccess("All entity and relation properties are valid")
		return nil
	}

	out.WriteError("Found %d property errors:", errorCount)

	if len(allEntityErrors) > 0 {
		out.WriteMessage("")
		out.WriteMessage("Entities (%d):", len(allEntityErrors))
		for _, ee := range allEntityErrors {
			out.WriteMessage("  %s (%s):", ee.EntityID, ee.EntityType)
			for _, err := range ee.Errors {
				out.WriteMessage("    - %s", err.Error())
			}
		}
	}

	if len(allRelationErrors) > 0 {
		out.WriteMessage("")
		out.WriteMessage("Relations (%d):", len(allRelationErrors))
		for _, re := range allRelationErrors {
			out.WriteMessage("  %s:", re.RelationKey)
			for _, err := range re.Errors {
				out.WriteMessage("    - %s", err.Error())
			}
		}
	}

	return nil
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
		opts, err := resolveAnalyzeOpts()
		if err != nil {
			return err
		}
		return runValidations(*opts)
	},
}

// runValidations executes custom validation rules.
func runValidations(opts workspace.AnalyzeOptions) error {
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

	violations := ws.RunValidations(opts)

	// Count by severity
	errorCount, warningCount := 0, 0
	for _, v := range violations {
		if v.Severity == "error" {
			errorCount++
		} else {
			warningCount++
		}
	}

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
			Details: violations,
		})
	}

	// Group violations by rule for text output
	ruleViolations := make(map[string][]workspace.ValidationViolation)
	for _, v := range violations {
		ruleViolations[v.RuleName] = append(ruleViolations[v.RuleName], v)
	}

	for _, rule := range rules {
		vs := ruleViolations[rule.Name]
		if len(vs) > 0 {
			if rule.GetSeverity() == "error" {
				out.WriteError("%s (%d):", rule.Description, len(vs))
			} else {
				out.WriteWarning("%s (%d):", rule.Description, len(vs))
			}
			for _, v := range vs {
				out.WriteMessage("  %s: %s", v.EntityID, v.EntityTitle)
			}
		}
	}

	if errorCount == 0 && warningCount == 0 {
		out.WriteSuccess("All %d validation rules passed", len(rules))
		return nil
	}
	if errorCount > 0 {
		out.WriteError("Found %d errors, %d warnings across %d rules", errorCount, warningCount, len(rules))
	} else {
		out.WriteWarning("Found %d warnings across %d rules", warningCount, len(rules))
	}
	return nil
}

var analyzeAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Run all analyses",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := resolveAnalyzeOpts()
		if err != nil {
			return err
		}

		// Get summary from workspace
		summary := ws.AnalyzeAll(*opts)

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

			jsonSummary := allAnalysisSummary{
				Orphans:            summary.Orphans,
				Cardinality:        summary.Cardinality,
				Duplicates:         summary.Duplicates,
				Gaps:               summary.Gaps,
				Properties:         summary.PropertyErrors,
				ValidationErrors:   summary.ValidationErrors,
				ValidationWarnings: summary.ValidationWarnings,
			}

			totalIssues := summary.Orphans + summary.Cardinality + summary.Duplicates +
				summary.Gaps + summary.PropertyErrors + summary.ValidationErrors
			status := "success"
			message := "All analyses passed"
			if summary.ValidationErrors > 0 || summary.PropertyErrors > 0 {
				status = "error"
				message = fmt.Sprintf("Found %d issues requiring attention", totalIssues)
			} else if totalIssues > 0 || summary.ValidationWarnings > 0 {
				status = "warning"
				message = fmt.Sprintf("Found %d issues and %d warnings", totalIssues, summary.ValidationWarnings)
			}

			return out.WriteAnalysisResult(output.AnalysisResult{
				Status:  status,
				Message: message,
				Count:   totalIssues + summary.ValidationWarnings,
				Details: jsonSummary,
			})
		}

		// Text output format
		summaryItems := []string{
			fmt.Sprintf("Orphans: %d", summary.Orphans),
			fmt.Sprintf("Cardinality: %d", summary.Cardinality),
			fmt.Sprintf("Duplicates: %d", summary.Duplicates),
			fmt.Sprintf("Gaps: %d", summary.Gaps),
			fmt.Sprintf("Properties: %d", summary.PropertyErrors),
		}
		if len(meta.Validations) > 0 {
			summaryItems = append(summaryItems, fmt.Sprintf("Validation Errors: %d", summary.ValidationErrors))
			summaryItems = append(summaryItems, fmt.Sprintf("Validation Warnings: %d", summary.ValidationWarnings))
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
		if err := runPropertyValidation(*opts); err != nil {
			errs = append(errs, fmt.Errorf("property validation: %w", err))
		}

		if len(meta.Validations) > 0 {
			out.WriteMessage("")
			out.WriteSectionHeader("Custom Validations")
			if err := runValidations(*opts); err != nil {
				errs = append(errs, fmt.Errorf("custom validations: %w", err))
			}
		}

		if len(errs) > 0 {
			return errs[0]
		}

		return nil
	},
}

var analyzeSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Analyze metamodel schema usage",
	Long: `Analyze metamodel schema usage to find unused or underused types.

Shows:
  - Entity types with no instances
  - Relation types with no instances
  - Custom types (enums) not referenced by any property
  - Types with few instances (when --threshold is set)

Use --cleanup to remove unused types from metamodel.yaml and update
data-entry.yaml accordingly.

Examples:
  rela analyze schema              # Show unused types
  rela analyze schema --threshold 2   # Include types with ≤2 instances
  rela analyze schema --cleanup       # Remove unused types
  rela analyze schema --cleanup --dry-run  # Preview cleanup changes`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if schemaThreshold < 0 {
			return fmt.Errorf("--threshold must be non-negative")
		}

		// Load optional config files
		dataEntry := loadDataEntryConfig()

		// Run analysis
		analysis := schema.Analyze(meta, &schema.StoreCounter{Store: ws.Store()}, dataEntry, schemaThreshold)

		// Handle cleanup mode
		if schemaCleanup {
			return runSchemaCleanup(analysis)
		}

		// Output results
		return outputSchemaAnalysis(analysis)
	},
}

// loadDataEntryConfig loads data-entry.yaml if it exists.
func loadDataEntryConfig() *dataentryconfig.Config {
	data, err := ws.ReadProjectFile(dataentryconfig.ConfigFile)
	if err != nil {
		return nil // File doesn't exist or can't be read
	}
	var cfg dataentryconfig.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil // Invalid YAML
	}
	return &cfg
}

// runSchemaCleanup executes the cleanup plan.
func runSchemaCleanup(analysis *schema.Analysis) error {
	plan := schema.PlanCleanup(analysis)

	if plan.IsEmpty() {
		if out.Format == "json" {
			return out.WriteAnalysisResult(output.AnalysisResult{
				Status:  "success",
				Message: "No unused types to clean up",
				Count:   0,
				Details: plan,
			})
		}
		out.WriteSuccess("No unused types to clean up")
		return nil
	}

	// Output the plan
	if out.Format == "json" {
		status := "success"
		message := fmt.Sprintf("Planned %d changes", plan.TotalChanges())
		if schemaDryRun {
			message = fmt.Sprintf("Would make %d changes (dry-run)", plan.TotalChanges())
		}
		return out.WriteAnalysisResult(output.AnalysisResult{
			Status:  status,
			Message: message,
			Count:   plan.TotalChanges(),
			Details: plan,
		})
	}

	// Text output
	if schemaDryRun {
		out.WriteMessage("Dry-run: would make the following changes:")
	} else {
		out.WriteMessage("Making the following changes:")
	}

	for _, change := range plan.MetamodelChanges {
		out.WriteMessage("  %s: %s %s", change.File, change.Action, change.Target)
	}
	for _, change := range plan.DataEntryChanges {
		out.WriteMessage("  %s: %s %s", change.File, change.Action, change.Target)
	}

	if schemaDryRun {
		out.WriteSuccess("Would make %d changes (dry-run, no files modified)", plan.TotalChanges())
		return nil
	}

	// Execute cleanup
	projectRoot := filepath.Dir(ws.Paths().MetamodelPath)
	if err := schema.ExecuteCleanup(plan, projectRoot, false); err != nil {
		return err
	}

	out.WriteSuccess("Made %d changes", plan.TotalChanges())
	return nil
}

// outputSchemaAnalysis outputs the schema analysis results.
func outputSchemaAnalysis(analysis *schema.Analysis) error {
	totalUnused := analysis.TotalUnused()
	totalLowUsage := analysis.TotalLowUsage()
	totalIssues := totalUnused + totalLowUsage

	if out.Format == "json" {
		return outputSchemaAnalysisJSON(analysis, totalUnused, totalLowUsage, totalIssues)
	}
	return outputSchemaAnalysisText(analysis, totalUnused, totalLowUsage, totalIssues)
}

// outputSchemaAnalysisJSON outputs schema analysis in JSON format.
func outputSchemaAnalysisJSON(analysis *schema.Analysis, totalUnused, totalLowUsage, totalIssues int) error {
	status := "success"
	message := "All schema types are in use"
	if totalUnused > 0 {
		status = "warning"
		message = fmt.Sprintf("Found %d unused types", totalUnused)
		if totalLowUsage > 0 {
			message = fmt.Sprintf("Found %d unused types and %d low-usage types", totalUnused, totalLowUsage)
		}
	} else if totalLowUsage > 0 {
		status = "warning"
		message = fmt.Sprintf("Found %d low-usage types", totalLowUsage)
	}
	return out.WriteAnalysisResult(output.AnalysisResult{
		Status:  status,
		Message: message,
		Count:   totalIssues,
		Details: analysis,
	})
}

// outputSchemaAnalysisText outputs schema analysis in text format.
func outputSchemaAnalysisText(analysis *schema.Analysis, totalUnused, totalLowUsage, totalIssues int) error {
	if totalIssues == 0 {
		out.WriteSuccess("All schema types are in use")
		return nil
	}

	outputUnusedTypes("Unused Entity Types", analysis.UnusedEntityTypes)
	outputUnusedTypes("Unused Relation Types", analysis.UnusedRelationTypes)
	outputUnusedCustomTypes(analysis.UnusedCustomTypes)
	outputLowUsageTypes("Low-Usage Entity Types", analysis.LowUsageEntityTypes)
	outputLowUsageTypes("Low-Usage Relation Types", analysis.LowUsageRelationTypes)

	out.WriteWarning("Found %d unused types and %d low-usage types", totalUnused, totalLowUsage)
	if totalUnused > 0 {
		out.WriteMessage("Run with --cleanup to remove unused types")
	}
	return nil
}

// outputUnusedTypes outputs a section of unused types with their references.
func outputUnusedTypes(header string, usages []schema.TypeUsage) {
	if len(usages) == 0 {
		return
	}
	out.WriteSectionHeader(header)
	for _, usage := range usages {
		if len(usage.References) == 0 {
			out.WriteWarning("  %s (0 instances, can be removed)", usage.Name)
		} else {
			out.WriteWarning("  %s (0 instances, %d references)", usage.Name, len(usage.References))
			for _, ref := range usage.References {
				out.WriteMessage("    - %s: %s (%s)", ref.File, ref.Section, ref.Kind)
			}
		}
	}
	out.WriteMessage("")
}

// outputUnusedCustomTypes outputs unused custom types (no references to show).
func outputUnusedCustomTypes(usages []schema.TypeUsage) {
	if len(usages) == 0 {
		return
	}
	out.WriteSectionHeader("Unused Custom Types")
	for _, usage := range usages {
		out.WriteWarning("  %s (not referenced, can be removed)", usage.Name)
	}
	out.WriteMessage("")
}

// outputLowUsageTypes outputs a section of low-usage types.
func outputLowUsageTypes(header string, usages []schema.TypeUsage) {
	if len(usages) == 0 {
		return
	}
	out.WriteSectionHeader(header)
	for _, usage := range usages {
		out.WriteMessage("  %s (%d instances)", usage.Name, usage.Count)
	}
	out.WriteMessage("")
}

func init() {
	// Schema subcommand flags
	analyzeSchemaCmd.Flags().IntVar(&schemaThreshold, "threshold", 0,
		"Show types with instance count <= threshold (0 = only unused)")
	analyzeSchemaCmd.Flags().BoolVar(&schemaCleanup, "cleanup", false,
		"Remove unused types from metamodel and config files")
	analyzeSchemaCmd.Flags().BoolVar(&schemaDryRun, "dry-run", false,
		"Preview cleanup changes without modifying files")

	analyzeCmd.AddCommand(analyzeOrphansCmd)
	analyzeCmd.AddCommand(analyzeDuplicatesCmd)
	analyzeCmd.AddCommand(analyzeGapsCmd)
	analyzeCmd.AddCommand(analyzeCardinalityCmd)
	analyzeCmd.AddCommand(analyzePropertiesCmd)
	analyzeCmd.AddCommand(analyzeValidationsCmd)
	analyzeCmd.AddCommand(analyzeAllCmd)
	analyzeCmd.AddCommand(analyzeSchemaCmd)

	rootCmd.AddCommand(analyzeCmd)
}

// resolveAnalyzeOpts returns the analysis options.
func resolveAnalyzeOpts() (*workspace.AnalyzeOptions, error) {
	return &workspace.AnalyzeOptions{}, nil
}
