package entitymanager

import (
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// partitionValidationErrors splits a slice of metamodel validation
// errors into hard structural errors (which must abort the write) and
// soft conditions (per DEC-HWZHA — surfaced as warnings on a
// successful write). Warnings are sorted by Path for stable
// client-facing ordering.
//
// See *metamodel.ValidationError.IsSoft for the categorization rule.
func partitionValidationErrors(errs []*metamodel.ValidationError) (
	hard []*metamodel.ValidationError, warnings []Warning,
) {
	for _, err := range errs {
		if err.IsSoft() {
			warnings = append(warnings, Warning{
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
	return "validation_warning"
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
