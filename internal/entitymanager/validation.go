package entitymanager

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// ValidationMode is the per-deployment validation policy (DEC-OYV2AK). It
// moves the line between the hard errors that refuse a write and the soft
// conditions that ride along as warnings. It adds no mechanism: a strict
// error takes the same 422 path a structural error takes.
type ValidationMode int

const (
	// ValidationAdvisory is DEC-HWZHA unchanged, and the default. Only a
	// structural impossibility refuses a write; a missing required
	// property, a type mismatch and an invalid value are warnings.
	ValidationAdvisory ValidationMode = iota

	// ValidationStrict also refuses a write whose final state has a
	// missing required property, a type mismatch or an invalid value. The
	// data-entry UI cannot produce those states through its own controls,
	// so on a deployment where the UI is the way in, their arrival means a
	// non-UI writer or a stale client.
	ValidationStrict
)

// ParseValidationMode reads the configured spelling of a mode. Empty is
// [ValidationAdvisory].
func ParseValidationMode(s string) (ValidationMode, error) {
	switch strings.TrimSpace(s) {
	case "", "advisory":
		return ValidationAdvisory, nil
	case "strict":
		return ValidationStrict, nil
	}
	return ValidationAdvisory, fmt.Errorf("validation mode %q is not supported (valid: advisory, strict)", s)
}

// String returns the configured spelling of m.
func (m ValidationMode) String() string {
	switch m {
	case ValidationAdvisory:
		return "advisory"
	case ValidationStrict:
		return "strict"
	}
	return fmt.Sprintf("ValidationMode(%d)", int(m))
}

// advisory returns a copy of d that judges in [ValidationAdvisory]. A dry run
// that lists what is still missing uses it: a strict 422 would hide the very
// list it exists to show.
func (d Deps) advisory() Deps {
	d.ValidationMode = ValidationAdvisory
	return d
}

// structuralErrors returns the errors in errs that refuse a write on every
// deployment, whatever its mode. A pre-check uses it when the mode is applied
// later, to the state the write persists.
func structuralErrors(errs []*metamodel.ValidationError) []*metamodel.ValidationError {
	hard, _ := partitionValidationErrors(ValidationAdvisory, errs)
	return hard
}

// isHard reports whether err refuses a write under mode m.
func (m ValidationMode) isHard(err *metamodel.ValidationError) bool {
	return !err.IsSoft() || m == ValidationStrict
}

// partitionValidationErrors splits a slice of metamodel validation errors
// into hard errors, which must abort the write, and soft conditions, which
// are surfaced as warnings on a successful write (DEC-HWZHA). mode decides
// which side a soft-by-type error lands on (DEC-OYV2AK). Warnings are sorted
// by Path for stable client-facing ordering.
//
// See *metamodel.ValidationError.IsSoft for the categorization rule.
func partitionValidationErrors(mode ValidationMode, errs []*metamodel.ValidationError) (
	hard []*metamodel.ValidationError, warnings []entity.Warning,
) {
	for _, err := range errs {
		if !mode.isHard(err) {
			warnings = append(warnings, entity.Warning{
				Code:   warningCodeFor(err.Type),
				Path:   propertyPointer(err.Property),
				Detail: err.Message,
			})
		} else {
			hard = append(hard, err)
		}
	}
	sort.Slice(warnings, func(i, j int) bool {
		return warnings[i].Path < warnings[j].Path
	})
	return hard, warnings
}

// checkWriteRules refuses e when it violates a validation rule declared
// `enforce: write` (TKT-5CESLHD). Nil means e may be written.
//
// The rules read only e (see internal/writerule), so checking the exact
// value the write persists is atomic with the write without holding a
// transaction open for it.
func checkWriteRules(ctx context.Context, deps Deps, e *entity.Entity) error {
	if errs := deps.writeRules.Check(ctx, e); len(errs) > 0 {
		return newValidationError(errs)
	}
	return nil
}

// warningCodeFor maps a soft [metamodel.ValidationErrorType] to its
// stable warning code. Unknown types fall back to "validation_warning"
// — should never happen if IsSoft is honored, but defensive.
func warningCodeFor(t metamodel.ValidationErrorType) string {
	//exhaustive:ignore // hard-validation types fall through to the default fallback.
	switch t {
	case metamodel.ValidationErrorRequired:
		return "required_property_unset"
	case metamodel.ValidationErrorInvalidType:
		return "property_type_mismatch"
	case metamodel.ValidationErrorInvalidValue:
		return "property_value_invalid"
	}
	return "validation_warning" // coverage-ignore: unreachable-default: warningCodeFor is only called on soft errors,
	// and the three soft types (Required/InvalidType/InvalidValue) all have explicit cases above
}

// propertyPointer constructs an RFC 6901 JSON Pointer for a property
// name. Property names containing `/` or `~` are escaped per the
// spec (~ first, then /). Empty property name (entity-level errors)
// produces "/properties/" — callers should ensure entity-level errors
// don't reach here, but the function is defensive.
func propertyPointer(name string) string {
	escaped := strings.ReplaceAll(name, "~", "~0")
	escaped = strings.ReplaceAll(escaped, "/", "~1")
	return "/properties/" + escaped
}
