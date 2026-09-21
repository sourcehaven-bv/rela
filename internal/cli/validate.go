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

// Exit codes for `rela validate`. The three outcomes are distinct
// because they call for different actions, and conflating the last two
// is what BUG-NEQRY2 / BUG-4KPN2M were about: a run that could not read
// its input used to be indistinguishable from a clean one.
//
//	0 — every rule was evaluated over the whole project, and all passed.
//	1 — every rule was evaluated, and at least one violation was found.
//	2 — some input could not be read, so the run is INCOMPLETE. Whether
//	    the entities that were read are clean is not the question: rules
//	    that never saw the unreadable entities cannot have passed on
//	    them. Fix the named files and re-run.
//
// Exit 2 takes precedence over exit 1: an incomplete run cannot report a
// trustworthy violation count, so the incompleteness is the finding to
// act on first.
const (
	exitValidationFailed  = 1
	exitValidationPartial = 2
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
		fmt.Println("Validating schema...")
	}
	if result.MetamodelError != nil {
		fmt.Printf("  ✗ %v\n", result.MetamodelError)
		hasErrors = true
	} else if !quiet {
		fmt.Println("  ✓ schema is valid")
	}
	hasErrors = reportDataEntryValidation(result, hasErrors)
	for _, item := range []struct {
		name string
		err  error
	}{
		{"mail templates", result.MailTemplatesError}, {"schedules", result.SchedulesError},
	} {
		if item.err != nil {
			fmt.Printf("  ✗ %s: %v\n", item.name, item.err)
			hasErrors = true
		} else if !quiet {
			fmt.Printf("  ✓ %s are valid\n", item.name)
		}
	}
	if result.MailTemplatesPresent && !hasErrors {
		//nolint:contextcheck // appbuild.Discover does not take ctx; matches the entity-check path below
		mailSvc, discoverErr := appbuild.Discover(result.ProjectRoot, script.NewEngine())
		if discoverErr != nil {
			return fmt.Errorf("validate scheduled mail recipients: %w", discoverErr)
		}
		defer mailSvc.Close()
		if validateErr := mailSvc.ValidateScheduledMailRecipients(ctx); validateErr != nil {
			fmt.Printf("  ✗ scheduled mail recipients: %v\n", validateErr)
			hasErrors = true
		} else if !quiet {
			fmt.Println("  ✓ scheduled mail recipients are valid")
		}
	}

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
	if err != nil { // coverage-ignore: defensive: analysis.New only fails on nil deps; appbuild.Discover on a valid
		// project always supplies them
		return fmt.Errorf("initialize analysis service: %w", err)
	}

	outcome, err := runValidationChecks(ctx, checkSvc, checkAnalysis, out, result.Metamodel, c.Check)
	if err != nil {
		return err
	}
	return finishValidate(outcome, hasErrors || outcome.hasErrors)
}

// finishValidate maps the run's outcome onto the documented exit codes
// and prints the matching verdict.
//
// Incompleteness outranks violations: a run that could not read all its
// input cannot support any claim about the rules, including the claim
// that the violations it found are all of them.
func finishValidate(outcome checkOutcome, hasErrors bool) error {
	if outcome.incomplete() {
		reportIncompleteScan(outcome.scanErr, hasErrors)
		return errors.NewExitError(exitValidationPartial)
	}
	if hasErrors {
		if !quiet {
			fmt.Println("\nValidation failed.")
		}
		return errors.NewExitError(exitValidationFailed)
	}
	if !quiet {
		fmt.Println("\nAll validations passed.")
	}
	return nil
}

// reportIncompleteScan explains why the run is not a pass and names the
// files that could not be read, so the operator can fix them rather
// than re-run hoping for a different answer.
//
// This prints on stdout unconditionally — not gated on !quiet — because
// it is the reason for a non-zero exit, and a silent non-zero exit is
// the failure mode this pair of bugs is about in the other direction.
func reportIncompleteScan(scanErr error, hasErrors bool) {
	fmt.Println("\nValidation INCOMPLETE: some entities could not be read.")
	files := analysis.IncompleteScanFiles(scanErr)
	if len(files) > 0 {
		fmt.Println("\nFiles that could not be read:")
		for _, f := range files {
			fmt.Printf("  ✗ %s\n", f)
		}
	}
	// The full error text follows the file list: a backend that does not
	// name a path leaves the message as the only detail, and the yaml
	// error names the line even when the path was recovered.
	for line := range strings.SplitSeq(scanErr.Error(), "\n") {
		if line != "" {
			fmt.Printf("  %s\n", line)
		}
	}
	if hasErrors {
		fmt.Println("\nViolations were also found in the entities that could be read;" +
			" the list above is not necessarily complete.")
	}
	remedy := "Fix the errors above and re-run."
	if len(files) > 0 {
		remedy = "Fix the files above and re-run."
	}
	fmt.Println("\nRules were not evaluated over the whole project, so this run is not a pass. " + remedy)
}

// checkOutcome is what a set of entity checks concluded. The two fields
// are independent: a run can find violations, fail to read its input, or
// both, and the caller must be able to tell which.
type checkOutcome struct {
	// hasErrors reports that a check found a violation in the input it
	// managed to read.
	hasErrors bool
	// scanErr is non-nil when some input could not be read. Every check
	// contributes, so the operator sees all unreadable files at once
	// rather than one per re-run.
	scanErr error
}

// incomplete reports whether any check ran over input it could not
// fully read.
func (o checkOutcome) incomplete() bool { return o.scanErr != nil }

func runValidationChecks(
	ctx context.Context,
	checkSvc *appbuild.Services,
	checkAnalysis *analysis.Service,
	checkOut *output.Writer,
	meta metamodelAccessor,
	validateChecks []string,
) (checkOutcome, error) {
	checks, err := parseChecks(validateChecks, meta)
	if err != nil {
		return checkOutcome{}, err
	}
	opts := analysis.Options{}
	outcome := checkOutcome{}
	var scanErrs []error
	if checks.cardinality {
		found, scanErr, err := runCardinalityCheck(ctx, checkAnalysis, checkOut, opts)
		if err != nil {
			return checkOutcome{}, err
		}
		scanErrs = append(scanErrs, scanErr)
		if found {
			outcome.hasErrors = true
		}
	}
	if checks.properties {
		found, scanErr := runPropertiesCheck(ctx, checkSvc, checkOut, opts)
		scanErrs = append(scanErrs, scanErr)
		if found {
			outcome.hasErrors = true
		}
	}
	if checks.validations {
		found, scanErr := runValidationsCheck(ctx, checkAnalysis, checkOut, opts, checks.validationFilters)
		scanErrs = append(scanErrs, scanErr)
		if found {
			outcome.hasErrors = true
		}
	}
	outcome.scanErr = stderrors.Join(scanErrs...)
	return outcome, nil
}

// runCardinalityCheck runs cardinality validation. Returns true if
// violations were found; a store error aborts the check before any
// VIOLATION output is written (never report a fabricated violation —
// see the CheckCardinality error policy). The section header above the
// error return is the pre-existing plain-stdout banner every validate
// check prints on !quiet, including in JSON mode.
func runCardinalityCheck(
	ctx context.Context, checkAnalysis *analysis.Service, checkOut *output.Writer, opts analysis.Options,
) (found bool, scanErr, err error) {
	if !quiet {
		fmt.Println("\nChecking cardinality constraints...")
	}
	violations, err := checkAnalysis.CheckCardinality(ctx, opts)
	if err != nil {
		// An incomplete scan is reported to the caller, not returned as a
		// hard error: other checks still run, and the operator gets every
		// unreadable file in one pass.
		if analysis.IsIncompleteScan(err) {
			if !quiet && checkOut.Format != output.FormatJSON {
				checkOut.WriteError("Cardinality NOT checked: could not read all entities")
			}
			return false, err, nil
		}
		return false, nil, err
	}
	if len(violations) == 0 {
		if !quiet && checkOut.Format != output.FormatJSON {
			checkOut.WriteSuccess("All cardinality constraints satisfied")
		}
		return false, nil, nil
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
	return true, nil, nil
}

// runPropertiesCheck runs property validation. Returns whether errors
// were found, plus a non-nil scanErr when some entity could not be read
// — in which case "all properties are valid" is not a claim this check
// can make, whatever the entities it did read looked like.
func runPropertiesCheck(
	ctx context.Context, checkSvc *appbuild.Services, checkOut *output.Writer, opts analysis.Options,
) (bool, error) {
	if !quiet {
		fmt.Println("\nValidating entity properties...")
	}
	propErrors, scanErr := schema.ValidateEntityProperties(ctx, checkSvc.Store(), checkSvc.Meta())
	if scanErr != nil {
		scanErr = &analysis.IncompleteScanError{Op: "validate entity properties", Err: scanErr}
		if !quiet && checkOut.Format != output.FormatJSON {
			checkOut.WriteError("Entity properties NOT fully checked: could not read all entities")
		}
	}
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
		if !quiet && checkOut.Format != output.FormatJSON && scanErr == nil {
			checkOut.WriteSuccess("All entity properties are valid")
		}
		return false, scanErr
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
	return true, scanErr
}

// runValidationsCheck runs the custom Lua validation rules. Returns
// whether errors were found, plus a non-nil scanErr when the rules ran
// over an entity set the scan could not fully read — the exact case
// BUG-4KPN2M describes, where a rule reports a pass on an entity it
// never saw.
func runValidationsCheck(
	ctx context.Context,
	checkAnalysis *analysis.Service,
	checkOut *output.Writer,
	opts analysis.Options,
	filters []analysis.ValidationFilter,
) (bool, error) {
	if !quiet {
		fmt.Println("\nRunning custom validations...")
	}
	var result analysis.ValidationResult
	var scanErr error
	if len(filters) > 0 {
		result, scanErr = checkAnalysis.RunValidationsFiltered(ctx, opts, filters)
	} else {
		result, scanErr = checkAnalysis.RunValidations(ctx, opts)
	}
	if scanErr != nil && !quiet && checkOut.Format != output.FormatJSON {
		checkOut.WriteError("Validation rules NOT evaluated over all entities: could not read all input")
	}
	violations := result.Violations
	errorCount, warningCount := analysis.CountValidationsBySeverity(violations)
	if len(violations) == 0 && !result.HasErrors() {
		// Only claim a pass when the rules actually saw everything.
		if !quiet && checkOut.Format != output.FormatJSON && scanErr == nil {
			checkOut.WriteSuccess("All validation rules passed")
		}
		return false, scanErr
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
	return errorCount > 0 || result.HasErrors(), scanErr
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
	// Description is the rule's own text, identical across every violation
	// of the rule, so taking it from the last one is well-defined. (It was
	// not while Lua violations carried their per-entity message here — the
	// heading then showed one arbitrary entity's message as if it were the
	// rule.)
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
			checkOut.WriteMessage("%s", formatValidationViolationLine(v))
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
	if after, ok := strings.CutPrefix(filterStr, "@"); ok {
		entityType := after
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
