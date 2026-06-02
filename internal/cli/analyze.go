package cli

import (
	"context"
	stderrors "errors"
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/analysis"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/schema"
)

// AnalyzeCmd is the parent of analyze subcommands.
type AnalyzeCmd struct {
	Orphans       AnalyzeOrphansCmd       `cmd:"" help:"Find entities with no connections."`
	Duplicates    AnalyzeDuplicatesCmd    `cmd:"" help:"Find entities with similar titles."`
	Gaps          AnalyzeGapsCmd          `cmd:"" help:"Find gaps in ID sequences."`
	Cardinality   AnalyzeCardinalityCmd   `cmd:"" help:"Check relation cardinality constraints."`
	RelationOrder AnalyzeRelationOrderCmd `cmd:"" name:"relation-order" help:"Find duplicate or missing values on managed relation order properties."`
	Properties    AnalyzePropertiesCmd    `cmd:"" help:"Validate entity property values against metamodel."`
	Validations   AnalyzeValidationsCmd   `cmd:"" help:"Run custom validation rules from metamodel."`
	All           AnalyzeAllCmd           `cmd:"" help:"Run all analyses."`
	Schema        AnalyzeSchemaCmd        `cmd:"" help:"Analyze metamodel schema usage."`
}

// resolveAnalyzeOpts returns the analysis options. Kept for symmetry
// with the original implementation; today there are no flag-driven
// scope or filter overrides at the top level.
func resolveAnalyzeOpts() (*analysis.Options, error) {
	return &analysis.Options{}, nil
}

// writeAnalysisJSON writes an analysis result if JSON output is enabled.
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
		Status: status, Message: message, Count: count, Details: details,
	})
	return true
}

// AnalyzeOrphansCmd finds entities with no connections.
type AnalyzeOrphansCmd struct{}

// Run dispatches `rela analyze orphans`.
func (c *AnalyzeOrphansCmd) Run(ctx context.Context, svc *cliServices) error {
	opts, err := resolveAnalyzeOpts()
	if err != nil {
		return err
	}
	orphans := svc.FindOrphansWithScope(ctx, *opts)
	filter.SortByID(orphans, storeEntityRecord, false)

	orphansMsg := "No orphan entities found"
	if writeAnalysisJSON(len(orphans), orphans, orphansMsg, "Found %d orphan entities") {
		return nil
	}

	if len(orphans) == 0 {
		out.WriteSuccess("No orphan entities found")
		return nil
	}
	out.WriteWarning("Found %d orphan entities:", len(orphans))
	return out.WriteEntities(orphans)
}

// AnalyzeDuplicatesCmd finds entities with similar titles.
type AnalyzeDuplicatesCmd struct{}

// Run dispatches `rela analyze duplicates`.
func (c *AnalyzeDuplicatesCmd) Run(ctx context.Context, svc *cliServices) error {
	opts, err := resolveAnalyzeOpts()
	if err != nil {
		return err
	}
	duplicates := svc.FindDuplicates(ctx, *opts)

	if out.Format == "json" {
		type duplicateGroup struct {
			Title    string           `json:"title"`
			Entities []*entity.Entity `json:"entities"`
		}
		var details []duplicateGroup
		for _, group := range duplicates {
			details = append(details, duplicateGroup{Title: group.Title, Entities: group.Entities})
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
}

// AnalyzeGapsCmd finds gaps in ID sequences.
type AnalyzeGapsCmd struct{}

// Run dispatches `rela analyze gaps`.
func (c *AnalyzeGapsCmd) Run(ctx context.Context, svc *cliServices) error {
	opts, err := resolveAnalyzeOpts()
	if err != nil {
		return err
	}
	allGaps := svc.FindGaps(ctx, *opts)
	gapsMsg := "No ID sequence gaps found"
	if writeAnalysisJSON(len(allGaps), allGaps, gapsMsg, "Found gaps in %d ID sequences") {
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
}

// AnalyzeCardinalityCmd checks relation cardinality constraints.
type AnalyzeCardinalityCmd struct{}

// Run dispatches `rela analyze cardinality`.
func (c *AnalyzeCardinalityCmd) Run(ctx context.Context, svc *cliServices) error {
	opts, err := resolveAnalyzeOpts()
	if err != nil {
		return err
	}
	violations := svc.CheckCardinality(ctx, *opts)
	cardMsg := "All cardinality constraints satisfied"
	if writeAnalysisJSON(len(violations), violations, cardMsg, "Found %d cardinality violations") {
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
}

// AnalyzeRelationOrderCmd finds duplicate or missing values on managed
// relation order properties. Soft condition: engine tolerates them
// (duplicates sort stably, missing values sort last) — use this to
// surface markdown files needing cleanup after hand-edits or imports.
type AnalyzeRelationOrderCmd struct{}

// Run dispatches `rela analyze relation-order`.
func (c *AnalyzeRelationOrderCmd) Run(ctx context.Context, svc *cliServices) error {
	opts, err := resolveAnalyzeOpts()
	if err != nil {
		return err
	}
	issues := svc.CheckRelationOrder(ctx, *opts)

	if writeAnalysisJSON(len(issues), issues,
		"All orderable relations have consistent order values",
		"Found %d relation-order issue(s)") {

		return nil
	}

	for _, iss := range issues {
		out.WriteWarning("%s (%s) on %s side of %q: %d %s value(s) at %s",
			iss.EntityID, iss.EntityType, iss.Side, iss.RelationType,
			iss.Count, iss.Kind, iss.Property)
	}

	if len(issues) == 0 {
		out.WriteSuccess("All orderable relations have consistent order values")
	} else {
		out.WriteWarning("Found %d relation-order issue(s)", len(issues))
	}
	return nil
}

// AnalyzePropertiesCmd validates entity property values against metamodel.
type AnalyzePropertiesCmd struct{}

// Run dispatches `rela analyze properties`.
func (c *AnalyzePropertiesCmd) Run(ctx context.Context, svc *cliServices) error {
	opts, err := resolveAnalyzeOpts()
	if err != nil {
		return err
	}
	return runPropertyValidation(ctx, svc, *opts)
}

func runPropertyValidation(ctx context.Context, svc *cliServices, opts analysis.Options) error {
	allEntityErrors := schema.ValidateEntityProperties(ctx, svc.Store(), svc.Meta())
	if opts.Scope != nil {
		filtered := allEntityErrors[:0]
		for _, ee := range allEntityErrors {
			if opts.Scope[ee.EntityID] {
				filtered = append(filtered, ee)
			}
		}
		allEntityErrors = filtered
	}
	allRelationErrors := schema.ValidateRelationProperties(ctx, svc.Store(), svc.Meta())

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
	allEntityErrors []schema.PropertyError,
	allRelationErrors []schema.RelationPropertyError,
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
		Status: status, Message: message, Count: errorCount, Details: details,
	})
}

func writePropertyValidationText(
	allEntityErrors []schema.PropertyError,
	allRelationErrors []schema.RelationPropertyError,
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

// AnalyzeValidationsCmd runs custom validation rules from metamodel.
type AnalyzeValidationsCmd struct{}

// Run dispatches `rela analyze validations`.
func (c *AnalyzeValidationsCmd) Run(ctx context.Context, svc *cliServices) error {
	opts, err := resolveAnalyzeOpts()
	if err != nil {
		return err
	}
	return runValidations(ctx, svc, *opts)
}

func runValidations(ctx context.Context, svc *cliServices, opts analysis.Options) error {
	rules := svc.Meta().Validations
	if len(rules) == 0 {
		return writeNoValidationRules()
	}
	result := svc.RunValidations(ctx, opts)
	errorCount, warningCount := countValidationViolationsBySeverity(result.Violations)
	if out.Format == "json" {
		return writeValidationsJSON(rules, result, errorCount, warningCount)
	}
	return writeValidationsText(rules, result, errorCount, warningCount)
}

func writeNoValidationRules() error {
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

func countValidationViolationsBySeverity(violations []analysis.ValidationViolation) (errs, warns int) {
	for _, v := range violations {
		if v.Severity == "error" {
			errs++
		} else {
			warns++
		}
	}
	return errs, warns
}

func writeValidationsJSON(
	rules []metamodel.ValidationRule,
	result analysis.ValidationResult,
	errorCount, warningCount int,
) error {
	status := "success"
	message := fmt.Sprintf("All %d validation rules passed", len(rules))
	if errorCount > 0 || result.HasErrors() {
		status = "error"
		message = fmt.Sprintf("Found %d errors, %d warnings across %d rules",
			errorCount, warningCount, len(rules))
	} else if warningCount > 0 {
		status = "warning"
		message = fmt.Sprintf("Found %d warnings across %d rules", warningCount, len(rules))
	}
	if jsonErr := out.WriteAnalysisResult(output.AnalysisResult{
		Status: status, Message: message, Count: errorCount + warningCount, Details: result.Violations,
	}); jsonErr != nil {
		return jsonErr
	}
	if errorCount > 0 || result.HasErrors() {
		return errors.NewExitError(1)
	}
	return nil
}

func writeValidationsText(
	rules []metamodel.ValidationRule,
	result analysis.ValidationResult,
	errorCount, warningCount int,
) error {
	ruleViolations := make(map[string][]analysis.ValidationViolation)
	for _, v := range result.Violations {
		ruleViolations[v.RuleName] = append(ruleViolations[v.RuleName], v)
	}
	for _, rule := range rules {
		writeRuleViolations(rule, ruleViolations[rule.Name])
	}
	renderValidationErrors(result.ScriptErrors, result.LoadErrors)
	writeValidationsSummary(len(rules), errorCount, warningCount, result.HasErrors())
	if errorCount > 0 || result.HasErrors() {
		return errors.NewExitError(1)
	}
	return nil
}

func writeRuleViolations(rule metamodel.ValidationRule, vs []analysis.ValidationViolation) {
	if len(vs) == 0 {
		return
	}
	if rule.GetSeverity() == "error" {
		out.WriteError("%s (%d):", rule.Description, len(vs))
	} else {
		out.WriteWarning("%s (%d):", rule.Description, len(vs))
	}
	for _, v := range vs {
		out.WriteMessage("  %s: %s", v.EntityID, v.EntityTitle)
	}
}

func writeValidationsSummary(ruleCount, errorCount, warningCount int, hasErrors bool) {
	if errorCount == 0 && warningCount == 0 && !hasErrors {
		out.WriteSuccess("All %d validation rules passed", ruleCount)
		return
	}
	if errorCount > 0 {
		out.WriteError("Found %d errors, %d warnings across %d rules",
			errorCount, warningCount, ruleCount)
	} else if warningCount > 0 {
		out.WriteWarning("Found %d warnings across %d rules", warningCount, ruleCount)
	}
}

func renderValidationErrors(scriptErrors []*lua.ScriptError, loadErrors []analysis.ValidationLoadError) {
	if len(scriptErrors) > 0 {
		out.WriteError("Validation script errors (%d):", len(scriptErrors))
		for _, se := range scriptErrors {
			out.WriteMessage("%s", formatScriptError(se))
		}
	}
	if len(loadErrors) > 0 {
		out.WriteError("Validation load errors (%d):", len(loadErrors))
		for _, le := range loadErrors {
			out.WriteMessage("  %s: %s", le.RuleName, le.Message)
		}
	}
}

// AnalyzeAllCmd runs all analyses.
type AnalyzeAllCmd struct{}

// Run dispatches `rela analyze all`.
func (c *AnalyzeAllCmd) Run(ctx context.Context, svc *cliServices) error {
	opts, err := resolveAnalyzeOpts()
	if err != nil {
		return err
	}
	summary := svc.AnalyzeAll(ctx, *opts)
	if out.Format == "json" {
		return writeAnalyzeAllJSON(summary)
	}
	writeAnalyzeAllSummary(svc, summary)
	return runAnalyzeAllSections(ctx, svc, *opts)
}

type allAnalysisSummary struct {
	Orphans                int `json:"orphans"`
	Cardinality            int `json:"cardinality"`
	Duplicates             int `json:"duplicates"`
	Gaps                   int `json:"gaps"`
	Properties             int `json:"properties"`
	ValidationErrors       int `json:"validation_errors"`
	ValidationWarnings     int `json:"validation_warnings"`
	ValidationScriptErrors int `json:"validation_script_errors"`
	ValidationLoadErrors   int `json:"validation_load_errors"`
}

func writeAnalyzeAllJSON(summary *analysis.Summary) error {
	jsonSummary := allAnalysisSummary{
		Orphans:                summary.Orphans,
		Cardinality:            summary.Cardinality,
		Duplicates:             summary.Duplicates,
		Gaps:                   summary.Gaps,
		Properties:             summary.PropertyErrors,
		ValidationErrors:       summary.ValidationErrors,
		ValidationWarnings:     summary.ValidationWarnings,
		ValidationScriptErrors: summary.ValidationScriptErrors,
		ValidationLoadErrors:   summary.ValidationLoadErrors,
	}
	validationFailures := summary.ValidationScriptErrors + summary.ValidationLoadErrors
	totalIssues := summary.Orphans + summary.Cardinality + summary.Duplicates +
		summary.Gaps + summary.PropertyErrors + summary.ValidationErrors + validationFailures
	status := "success"
	message := "All analyses passed"
	if summary.ValidationErrors > 0 || summary.PropertyErrors > 0 || validationFailures > 0 {
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

func writeAnalyzeAllSummary(svc *cliServices, summary *analysis.Summary) {
	summaryItems := []string{
		fmt.Sprintf("Orphans: %d", summary.Orphans),
		fmt.Sprintf("Cardinality: %d", summary.Cardinality),
		fmt.Sprintf("Duplicates: %d", summary.Duplicates),
		fmt.Sprintf("Gaps: %d", summary.Gaps),
		fmt.Sprintf("Properties: %d", summary.PropertyErrors),
	}
	if len(svc.Meta().Validations) > 0 {
		summaryItems = append(summaryItems,
			fmt.Sprintf("Validation Errors: %d", summary.ValidationErrors),
			fmt.Sprintf("Validation Warnings: %d", summary.ValidationWarnings))
		if summary.ValidationScriptErrors > 0 {
			summaryItems = append(summaryItems,
				fmt.Sprintf("Validation Script Errors: %d", summary.ValidationScriptErrors))
		}
		if summary.ValidationLoadErrors > 0 {
			summaryItems = append(summaryItems,
				fmt.Sprintf("Validation Load Errors: %d", summary.ValidationLoadErrors))
		}
	}
	out.WriteSummaryBox(summaryItems)
	out.WriteMessage("")
}

func runAnalyzeAllSections(ctx context.Context, svc *cliServices, opts analysis.Options) error {
	var errs []error
	out.WriteSectionHeader("Orphan Analysis")
	if err := (&AnalyzeOrphansCmd{}).Run(ctx, svc); err != nil {
		errs = append(errs, fmt.Errorf("orphan analysis: %w", err))
	}
	out.WriteMessage("")
	out.WriteSectionHeader("Duplicate Analysis")
	if err := (&AnalyzeDuplicatesCmd{}).Run(ctx, svc); err != nil {
		errs = append(errs, fmt.Errorf("duplicate analysis: %w", err))
	}
	out.WriteMessage("")
	out.WriteSectionHeader("ID Gap Analysis")
	if err := (&AnalyzeGapsCmd{}).Run(ctx, svc); err != nil {
		errs = append(errs, fmt.Errorf("gap analysis: %w", err))
	}
	out.WriteMessage("")
	out.WriteSectionHeader("Cardinality Analysis")
	if err := (&AnalyzeCardinalityCmd{}).Run(ctx, svc); err != nil {
		errs = append(errs, fmt.Errorf("cardinality analysis: %w", err))
	}
	out.WriteMessage("")
	out.WriteSectionHeader("Property Validation")
	if err := runPropertyValidation(ctx, svc, opts); err != nil {
		errs = append(errs, fmt.Errorf("property validation: %w", err))
	}
	if len(svc.Meta().Validations) > 0 {
		out.WriteMessage("")
		out.WriteSectionHeader("Custom Validations")
		if err := runValidations(ctx, svc, opts); err != nil {
			errs = append(errs, fmt.Errorf("custom validations: %w", err))
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// AnalyzeSchemaCmd analyzes metamodel schema usage.
type AnalyzeSchemaCmd struct {
	Threshold int  `default:"0" help:"Show types with instance count <= threshold (0 = only unused)."`
	Cleanup   bool `help:"Remove unused types from metamodel and config files."`
	DryRun    bool `name:"dry-run" help:"Preview cleanup changes without modifying files."`
}

// Run dispatches `rela analyze schema`.
func (c *AnalyzeSchemaCmd) Run(svc *cliServices) error {
	if c.Threshold < 0 {
		return stderrors.New("--threshold must be non-negative")
	}
	dataEntry := loadDataEntryConfig(svc)
	analysisResult := schema.Analyze(svc.Meta(), &schema.StoreCounter{Store: svc.Store()}, dataEntry, c.Threshold)

	if c.Cleanup {
		return runSchemaCleanup(svc, analysisResult, c.DryRun)
	}
	return outputSchemaAnalysis(analysisResult)
}

func loadDataEntryConfig(svc *cliServices) *dataentryconfig.Config {
	data, err := svc.Config().Load(context.Background(), dataentryconfig.ConfigFile)
	if err != nil {
		return nil
	}
	var cfg dataentryconfig.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return &cfg
}

func runSchemaCleanup(svc *cliServices, analysisResult *schema.Analysis, dryRun bool) error {
	plan := schema.PlanCleanup(analysisResult)
	if plan.IsEmpty() {
		if out.Format == "json" {
			return out.WriteAnalysisResult(output.AnalysisResult{
				Status: "success", Message: "No unused types to clean up",
				Count: 0, Details: plan,
			})
		}
		out.WriteSuccess("No unused types to clean up")
		return nil
	}

	if out.Format == "json" {
		status := "success"
		message := fmt.Sprintf("Planned %d changes", plan.TotalChanges())
		if dryRun {
			message = fmt.Sprintf("Would make %d changes (dry-run)", plan.TotalChanges())
		}
		return out.WriteAnalysisResult(output.AnalysisResult{
			Status: status, Message: message,
			Count: plan.TotalChanges(), Details: plan,
		})
	}

	if dryRun {
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

	if dryRun {
		out.WriteSuccess("Would make %d changes (dry-run, no files modified)", plan.TotalChanges())
		return nil
	}

	projectRoot := filepath.Dir(svc.Paths().MetamodelPath)
	if err := schema.ExecuteCleanup(plan, projectRoot, false); err != nil {
		return err
	}
	out.WriteSuccess("Made %d changes", plan.TotalChanges())
	return nil
}

func outputSchemaAnalysis(an *schema.Analysis) error {
	totalUnused := an.TotalUnused()
	totalLowUsage := an.TotalLowUsage()
	totalIssues := totalUnused + totalLowUsage
	if out.Format == "json" {
		return outputSchemaAnalysisJSON(an, totalUnused, totalLowUsage, totalIssues)
	}
	return outputSchemaAnalysisText(an, totalUnused, totalLowUsage, totalIssues)
}

func outputSchemaAnalysisJSON(an *schema.Analysis, totalUnused, totalLowUsage, totalIssues int) error {
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
		Status: status, Message: message, Count: totalIssues, Details: an,
	})
}

func outputSchemaAnalysisText(an *schema.Analysis, totalUnused, totalLowUsage, totalIssues int) error {
	if totalIssues == 0 {
		out.WriteSuccess("All schema types are in use")
		return nil
	}
	outputUnusedTypes("Unused Entity Types", an.UnusedEntityTypes)
	outputUnusedTypes("Unused Relation Types", an.UnusedRelationTypes)
	outputUnusedCustomTypes(an.UnusedCustomTypes)
	outputLowUsageTypes("Low-Usage Entity Types", an.LowUsageEntityTypes)
	outputLowUsageTypes("Low-Usage Relation Types", an.LowUsageRelationTypes)

	out.WriteWarning("Found %d unused types and %d low-usage types", totalUnused, totalLowUsage)
	if totalUnused > 0 {
		out.WriteMessage("Run with --cleanup to remove unused types")
	}
	return nil
}

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
