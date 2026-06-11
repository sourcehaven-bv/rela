package cli

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/analysis"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
	"github.com/Sourcehaven-BV/rela/internal/schema"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// metamodelAccessor is the narrow consumer-side interface validate
// needs from a metamodel.
type metamodelAccessor interface {
	HasEntityType(entityType string) bool
	HasValidationRule(ruleName string) bool
}

// Known check types.
const (
	checkCardinality = "cardinality"
	checkProperties  = "properties"
	checkValidations = "validations"
	checkAll         = "all"
)

// ValidateCmd validates project configuration files.
type ValidateCmd struct {
	Check []string `help:"Run validation checks: cardinality, properties, validations, all, or validations:filter."`
}

// Run dispatches `rela validate`.
func (c *ValidateCmd) Run(ctx context.Context) error {
	startDir := projectPath
	if startDir == "" {
		startDir = os.Getenv("RELA_PROJECT")
	}

	result, err := projectsetup.Validate(startDir)
	if err != nil {
		return err
	}

	hasErrors := false
	if !quiet {
		fmt.Println("Validating metamodel.yaml...")
	}
	if result.MetamodelError != nil {
		fmt.Printf("  ✗ %v\n", result.MetamodelError)
		hasErrors = true
	} else if !quiet {
		fmt.Println("  ✓ metamodel.yaml is valid")
	}
	hasErrors = reportDataEntryValidation(result, hasErrors)

	if len(c.Check) == 0 {
		if hasErrors {
			fmt.Println("\nValidation failed.")
			return errors.NewExitError(1)
		}
		if !quiet {
			fmt.Println("\nAll configuration files are valid.")
		}
		return nil
	}

	if result.MetamodelError != nil {
		fmt.Println("\nSkipping entity checks (metamodel has errors)")
		return errors.NewExitError(1)
	}

	//nolint:contextcheck // appbuild.Discover does not take ctx; matches rela-server bootstrap
	checkSvc, err := appbuild.Discover(startDir, script.NewEngine())
	if err != nil {
		return fmt.Errorf("failed to initialize project services: %w", err)
	}
	checkAnalysis, err := analysis.New(analysis.Deps{
		Store:       checkSvc.Store(),
		Meta:        checkSvc.Meta(),
		Tracer:      checkSvc.Tracer(),
		LuaReadDeps: checkSvc.LuaReadDeps(),
		LuaCache:    checkSvc.ScriptEngine().LuaCache(),
		FS:          checkSvc.FS(),
		Paths:       checkSvc.Paths(),
	})
	if err != nil {
		return fmt.Errorf("initialize analysis service: %w", err)
	}

	checkErrors, err := runValidationChecks(ctx, checkSvc, checkAnalysis, out, result.Metamodel, c.Check)
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

func runValidationChecks(
	ctx context.Context,
	checkSvc *appbuild.Services,
	checkAnalysis *analysis.Service,
	checkOut *output.Writer,
	meta metamodelAccessor,
	validateChecks []string,
) (bool, error) {
	checks, err := parseChecks(validateChecks, meta)
	if err != nil {
		return false, err
	}
	opts := analysis.Options{}
	hasErrors := false
	if checks.cardinality {
		if runCardinalityCheck(ctx, checkAnalysis, checkOut, opts) {
			hasErrors = true
		}
	}
	if checks.properties {
		if runPropertiesCheck(ctx, checkSvc, checkOut, opts) {
			hasErrors = true
		}
	}
	if checks.validations {
		if runValidationsCheck(ctx, checkAnalysis, checkOut, opts, checks.validationFilters) {
			hasErrors = true
		}
	}
	return hasErrors, nil
}

// runCardinalityCheck runs cardinality validation. Returns true if errors found.
func runCardinalityCheck(
	ctx context.Context, checkAnalysis *analysis.Service, checkOut *output.Writer, opts analysis.Options,
) bool {
	if !quiet {
		fmt.Println("\nChecking cardinality constraints...")
	}
	violations := checkAnalysis.CheckCardinality(ctx, opts)
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
			Count:   len(violations), Details: violations,
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
func runPropertiesCheck(
	ctx context.Context, checkSvc *appbuild.Services, checkOut *output.Writer, opts analysis.Options,
) bool {
	if !quiet {
		fmt.Println("\nValidating entity properties...")
	}
	propErrors := schema.ValidateEntityProperties(ctx, checkSvc.Store(), checkSvc.Meta())
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
				EntityID: ee.EntityID, EntityType: ee.EntityType, Errors: errStrings,
			})
		}
		_ = checkOut.WriteAnalysisResult(output.AnalysisResult{
			Status:  "error",
			Message: fmt.Sprintf("Found %d property errors", errorCount),
			Count:   errorCount, Details: results,
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

func runValidationsCheck(
	ctx context.Context,
	checkAnalysis *analysis.Service,
	checkOut *output.Writer,
	opts analysis.Options,
	filters []analysis.ValidationFilter,
) bool {
	if !quiet {
		fmt.Println("\nRunning custom validations...")
	}
	var result analysis.ValidationResult
	if len(filters) > 0 {
		result = checkAnalysis.RunValidationsFiltered(ctx, opts, filters)
	} else {
		result = checkAnalysis.RunValidations(ctx, opts)
	}
	violations := result.Violations
	errorCount, warningCount := analysis.CountValidationsBySeverity(violations)
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
			Count:   errorCount + warningCount, Details: violations,
		})
	} else {
		if len(violations) > 0 {
			outputValidationViolations(checkOut, violations, errorCount, warningCount)
		}
		renderValidationErrorsTo(checkOut, result.ScriptErrors, result.LoadErrors)
	}
	return errorCount > 0 || result.HasErrors()
}

func renderValidationErrorsTo(
	checkOut *output.Writer,
	scriptErrors []*lua.ScriptError,
	loadErrors []analysis.ValidationLoadError,
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

func outputValidationViolations(
	checkOut *output.Writer,
	violations []analysis.ValidationViolation,
	errorCount, warningCount int,
) {
	ruleViolations := make(map[string][]analysis.ValidationViolation)
	ruleDescriptions := make(map[string]string)
	ruleSeverities := make(map[string]string)
	for _, v := range violations {
		ruleViolations[v.RuleName] = append(ruleViolations[v.RuleName], v)
		ruleDescriptions[v.RuleName] = v.Description
		ruleSeverities[v.RuleName] = v.Severity
	}
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

type parsedChecks struct {
	cardinality       bool
	properties        bool
	validations       bool
	validationFilters []analysis.ValidationFilter
}

func parseChecks(checks []string, meta metamodelAccessor) (*parsedChecks, error) {
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

func parseValidationFilter(filterStr string, meta metamodelAccessor) (analysis.ValidationFilter, error) {
	if strings.HasPrefix(filterStr, "@") {
		entityType := strings.TrimPrefix(filterStr, "@")
		if entityType == "" {
			return analysis.ValidationFilter{}, stderrors.New("empty entity type in validation filter")
		}
		if !meta.HasEntityType(entityType) {
			return analysis.ValidationFilter{}, fmt.Errorf("unknown entity type in validation filter: %s", entityType)
		}
		return analysis.ValidationFilter{EntityType: entityType}, nil
	}
	if !meta.HasValidationRule(filterStr) {
		return analysis.ValidationFilter{}, fmt.Errorf("unknown validation rule: %s", filterStr)
	}
	return analysis.ValidationFilter{RuleName: filterStr}, nil
}

func reportDataEntryValidation(result *projectsetup.ValidateResult, hasErrors bool) bool {
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
