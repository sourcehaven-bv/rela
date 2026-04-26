package cli

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/schema"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// validateChecks holds the --check flag values.
var validateChecks []string

// Known check types.
const (
	checkCardinality = "cardinality"
	checkProperties  = "properties"
	checkValidations = "validations"
	checkAll         = "all"
)

var validateCmd = &cobra.Command{
	Use:         "validate",
	Short:       "Validate project configuration files",
	Annotations: map[string]string{skipProjectDiscovery: "true"},
	Long: `Validate metamodel.yaml and data-entry.yaml configuration files.

By default, checks for:
- Unknown/misspelled keys
- Invalid cross-references (forms, lists, views)
- Invalid entity types, relations, and properties
- View traversal correctness
- Dashboard and command configuration

With --check flags, also runs entity/relation validation checks:
- cardinality: Check relation cardinality constraints
- properties: Validate entity property values against metamodel
- validations: Run custom validation rules from metamodel
- all: Run all validation checks

Validation rules can be filtered by name or entity type:
- validations:rule-name  Run a specific validation rule by name
- validations:@type      Run all validation rules for an entity type

Examples:
  rela validate                                    # Validate config files only
  rela validate --check cardinality                # Also check cardinality
  rela validate --check all                        # Run all checks
  rela validate --check validations:@ticket        # Run ticket validation rules
  rela validate --check validations:ready-tickets-need-effort  # Run specific rule
  rela validate --check cardinality --check validations:@planning-checklist`,
	RunE: runValidate,
}

func init() {
	validateCmd.Flags().StringArrayVar(&validateChecks, "check", nil,
		"Run validation checks: cardinality, properties, validations, all, or validations:filter")
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, _ []string) error {
	// Determine start directory: flag > env var > cwd
	startDir := projectPath
	if startDir == "" {
		startDir = os.Getenv("RELA_PROJECT")
	}

	// Always validate config files first
	result, err := workspace.Validate(startDir)
	if err != nil {
		return err
	}

	hasErrors := false

	// Report metamodel validation
	if !quiet {
		fmt.Println("Validating metamodel.yaml...")
	}
	if result.MetamodelError != nil {
		fmt.Printf("  ✗ %v\n", result.MetamodelError)
		hasErrors = true
	} else if !quiet {
		fmt.Println("  ✓ metamodel.yaml is valid")
	}

	// Report data-entry validation
	hasErrors = reportDataEntryValidation(result, hasErrors)

	// If no --check flags, we're done with config validation
	if len(validateChecks) == 0 {
		if hasErrors {
			fmt.Println("\nValidation failed.")
			return errors.NewExitError(1)
		}
		if !quiet {
			fmt.Println("\nAll configuration files are valid.")
		}
		return nil
	}

	// For --check flags, we need a workspace with the full graph
	if result.MetamodelError != nil {
		fmt.Println("\nSkipping entity checks (metamodel has errors)")
		return errors.NewExitError(1)
	}

	// Initialize workspace for entity checks (no Lua executor needed for validation)
	checkWs, err := workspace.Discover(startDir, workspace.NopScriptExecutor)
	if err != nil {
		return fmt.Errorf("failed to initialize workspace: %w", err)
	}

	// Run requested checks
	checkErrors, err := runValidationChecks(cmd.Context(), checkWs, out, result.Metamodel)
	if err != nil {
		return err
	}
	hasErrors = hasErrors || checkErrors

	if hasErrors {
		if !quiet {
			fmt.Println("\nValidation failed.")
		}
		return errors.NewExitError(1)
	}

	if !quiet {
		fmt.Println("\nAll validations passed.")
	}
	return nil
}

// runValidationChecks runs the requested validation checks and returns true if errors were found.
func runValidationChecks(
	ctx context.Context,
	checkWs *workspace.Workspace,
	checkOut *output.Writer,
	meta workspace.MetamodelAccessor,
) (bool, error) {
	// Parse check flags
	checks, err := parseChecks(validateChecks, meta)
	if err != nil {
		return false, err
	}

	opts := workspace.AnalyzeOptions{}
	hasErrors := false

	if checks.cardinality {
		if runCardinalityCheck(checkWs, checkOut, opts) {
			hasErrors = true
		}
	}

	if checks.properties {
		if runPropertiesCheck(checkWs, checkOut, opts) {
			hasErrors = true
		}
	}

	if checks.validations {
		if runValidationsCheck(ctx, checkWs, checkOut, opts, checks.validationFilters) {
			hasErrors = true
		}
	}

	return hasErrors, nil
}

// runCardinalityCheck runs cardinality validation. Returns true if errors found.
func runCardinalityCheck(checkWs *workspace.Workspace, checkOut *output.Writer, opts workspace.AnalyzeOptions) bool {
	if !quiet {
		fmt.Println("\nChecking cardinality constraints...")
	}
	violations := checkWs.CheckCardinality(opts)
	if len(violations) == 0 {
		if !quiet && checkOut.Format != output.FormatJSON {
			checkOut.WriteSuccess("All cardinality constraints satisfied")
		}
		return false
	}

	if checkOut.Format == output.FormatJSON {
		_ = checkOut.WriteAnalysisResult(output.AnalysisResult{
			Status:  "error",
			Message: fmt.Sprintf("Found %d cardinality violations", len(violations)),
			Count:   len(violations),
			Details: violations,
		})
	} else {
		for _, v := range violations {
			if strings.HasPrefix(v.Constraint, "min_") {
				checkOut.WriteWarning("%s must have at least %d '%s' relation(s), has %d",
					v.EntityID, v.Required, v.RelationType, v.Actual)
			} else {
				checkOut.WriteWarning("%s has more than %d '%s' relation(s): %d",
					v.EntityID, v.Required, v.RelationType, v.Actual)
			}
		}
	}
	return true
}

// runPropertiesCheck runs property validation. Returns true if errors found.
func runPropertiesCheck(checkWs *workspace.Workspace, checkOut *output.Writer, opts workspace.AnalyzeOptions) bool {
	if !quiet {
		fmt.Println("\nValidating entity properties...")
	}
	propErrors := schema.ValidateEntityProperties(checkWs.Store(), checkWs.Meta())
	if opts.Scope != nil {
		filtered := propErrors[:0]
		for _, pe := range propErrors {
			if opts.Scope[pe.EntityID] {
				filtered = append(filtered, pe)
			}
		}
		propErrors = filtered
	}
	errorCount := 0
	for _, pe := range propErrors {
		errorCount += len(pe.Errors)
	}
	if errorCount == 0 {
		if !quiet && checkOut.Format != output.FormatJSON {
			checkOut.WriteSuccess("All entity properties are valid")
		}
		return false
	}

	if checkOut.Format == output.FormatJSON {
		var results []output.PropertyValidationResult
		for _, ee := range propErrors {
			errStrings := make([]string, len(ee.Errors))
			for i, err := range ee.Errors {
				errStrings[i] = err.Message
			}
			results = append(results, output.PropertyValidationResult{
				EntityID:   ee.EntityID,
				EntityType: ee.EntityType,
				Errors:     errStrings,
			})
		}
		_ = checkOut.WriteAnalysisResult(output.AnalysisResult{
			Status:  "error",
			Message: fmt.Sprintf("Found %d property errors", errorCount),
			Count:   errorCount,
			Details: results,
		})
	} else {
		checkOut.WriteError("Found %d property errors across %d entities:", errorCount, len(propErrors))
		for _, ee := range propErrors {
			checkOut.WriteMessage("")
			checkOut.WriteMessage("  %s (%s):", ee.EntityID, ee.EntityType)
			for _, err := range ee.Errors {
				checkOut.WriteMessage("    - %s", err.Error())
			}
		}
	}
	return true
}

// runValidationsCheck runs custom validation rules. Returns true if errors found.
func runValidationsCheck(
	ctx context.Context,
	checkWs *workspace.Workspace,
	checkOut *output.Writer,
	opts workspace.AnalyzeOptions,
	filters []workspace.ValidationFilter,
) bool {
	if !quiet {
		fmt.Println("\nRunning custom validations...")
	}

	var result workspace.ValidationResult
	if len(filters) > 0 {
		result = checkWs.RunValidationsFiltered(ctx, opts, filters)
	} else {
		result = checkWs.RunValidations(ctx, opts)
	}
	violations := result.Violations

	errorCount, warningCount := workspace.CountValidationsBySeverity(violations)
	if len(violations) == 0 && !result.HasErrors() {
		if !quiet && checkOut.Format != output.FormatJSON {
			checkOut.WriteSuccess("All validation rules passed")
		}
		return false
	}

	if checkOut.Format == output.FormatJSON {
		status := "warning"
		if errorCount > 0 || result.HasErrors() {
			status = "error"
		}
		_ = checkOut.WriteAnalysisResult(output.AnalysisResult{
			Status:  status,
			Message: fmt.Sprintf("Found %d errors, %d warnings", errorCount, warningCount),
			Count:   errorCount + warningCount,
			Details: violations,
		})
	} else {
		if len(violations) > 0 {
			outputValidationViolations(checkOut, violations, errorCount, warningCount)
		}
		renderValidationErrorsTo(checkOut, result.ScriptErrors, result.LoadErrors)
	}

	return errorCount > 0 || result.HasErrors()
}

// renderValidationErrorsTo is the writer-explicit counterpart of
// renderValidationErrors: validate.go uses a per-call output.Writer
// (checkOut) rather than the package-global out.
func renderValidationErrorsTo(
	checkOut *output.Writer,
	scriptErrors []*lua.ScriptError,
	loadErrors []workspace.ValidationLoadError,
) {
	if len(scriptErrors) > 0 {
		checkOut.WriteError("Validation script errors (%d):", len(scriptErrors))
		for _, se := range scriptErrors {
			checkOut.WriteMessage("%s", formatScriptError(se))
		}
	}
	if len(loadErrors) > 0 {
		checkOut.WriteError("Validation load errors (%d):", len(loadErrors))
		for _, le := range loadErrors {
			checkOut.WriteMessage("  %s: %s", le.RuleName, le.Message)
		}
	}
}

// outputValidationViolations prints validation violations grouped by rule.
func outputValidationViolations(
	checkOut *output.Writer,
	violations []workspace.ValidationViolation,
	errorCount, warningCount int,
) {
	// Group violations by rule
	ruleViolations := make(map[string][]workspace.ValidationViolation)
	ruleDescriptions := make(map[string]string)
	ruleSeverities := make(map[string]string)
	for _, v := range violations {
		ruleViolations[v.RuleName] = append(ruleViolations[v.RuleName], v)
		ruleDescriptions[v.RuleName] = v.Description
		ruleSeverities[v.RuleName] = v.Severity
	}

	// Sort rule names for deterministic output
	ruleNames := make([]string, 0, len(ruleViolations))
	for ruleName := range ruleViolations {
		ruleNames = append(ruleNames, ruleName)
	}
	sort.Strings(ruleNames)

	for _, ruleName := range ruleNames {
		vs := ruleViolations[ruleName]
		if ruleSeverities[ruleName] == "error" {
			checkOut.WriteError("%s (%d):", ruleDescriptions[ruleName], len(vs))
		} else {
			checkOut.WriteWarning("%s (%d):", ruleDescriptions[ruleName], len(vs))
		}
		for _, v := range vs {
			checkOut.WriteMessage("  %s: %s", v.EntityID, v.EntityTitle)
		}
	}

	if errorCount > 0 {
		checkOut.WriteError("Found %d errors, %d warnings", errorCount, warningCount)
	} else {
		checkOut.WriteWarning("Found %d warnings", warningCount)
	}
}

// parsedChecks holds the parsed check configuration.
type parsedChecks struct {
	cardinality       bool
	properties        bool
	validations       bool
	validationFilters []workspace.ValidationFilter
}

// parseChecks parses and validates --check flag values.
func parseChecks(checks []string, meta workspace.MetamodelAccessor) (*parsedChecks, error) {
	result := &parsedChecks{}

	for _, check := range checks {
		switch {
		case check == checkAll:
			result.cardinality = true
			result.properties = true
			result.validations = true

		case check == checkCardinality:
			result.cardinality = true

		case check == checkProperties:
			result.properties = true

		case check == checkValidations:
			result.validations = true

		case strings.HasPrefix(check, checkValidations+":"):
			result.validations = true
			filterStr := strings.TrimPrefix(check, checkValidations+":")
			if filterStr == "" {
				return nil, fmt.Errorf("empty validation filter in --check %s", check)
			}

			filter, err := parseValidationFilter(filterStr, meta)
			if err != nil {
				return nil, err
			}
			result.validationFilters = append(result.validationFilters, filter)

		default:
			return nil, fmt.Errorf("unknown check type: %s (valid: cardinality, properties, validations, all)", check)
		}
	}

	return result, nil
}

// parseValidationFilter parses a validation filter string.
func parseValidationFilter(filterStr string, meta workspace.MetamodelAccessor) (workspace.ValidationFilter, error) {
	if strings.HasPrefix(filterStr, "@") {
		// Entity type filter
		entityType := strings.TrimPrefix(filterStr, "@")
		if entityType == "" {
			return workspace.ValidationFilter{}, stderrors.New("empty entity type in validation filter")
		}
		// Validate entity type exists
		if !meta.HasEntityType(entityType) {
			return workspace.ValidationFilter{}, fmt.Errorf("unknown entity type in validation filter: %s", entityType)
		}
		return workspace.ValidationFilter{EntityType: entityType}, nil
	}

	// Rule name filter - validate it exists
	if !meta.HasValidationRule(filterStr) {
		return workspace.ValidationFilter{}, fmt.Errorf("unknown validation rule: %s", filterStr)
	}
	return workspace.ValidationFilter{RuleName: filterStr}, nil
}

// reportDataEntryValidation reports data-entry.yaml validation results.
func reportDataEntryValidation(result *workspace.ValidateResult, hasErrors bool) bool {
	if result.DataEntrySkipped {
		if quiet {
			return hasErrors
		}
		if result.MetamodelError != nil {
			fmt.Println("  ⚠ Skipping data-entry validation (metamodel has errors)")
		} else {
			fmt.Printf("Skipping %s (file not found)\n", dataentryconfig.ConfigFile)
		}
		return hasErrors
	}

	if !quiet {
		fmt.Printf("Validating %s...\n", dataentryconfig.ConfigFile)
	}
	if result.DataEntryError != nil {
		fmt.Printf("  ✗ %v\n", result.DataEntryError)
		return true
	}
	if !quiet {
		fmt.Printf("  ✓ %s is valid\n", dataentryconfig.ConfigFile)
	}
	return hasErrors
}
