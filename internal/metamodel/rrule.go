package metamodel

import (
	"errors"
	"fmt"
	"strings"

	rrule "github.com/teambition/rrule-go"
)

// ValidateRrule validates an RRULE string. It strips the "RRULE:" prefix if
// present, parses the rule, and rejects INTERVAL > 1 without DTSTART (which
// would cause interval cadence drift).
//
// This function is the single source of truth for RRULE validation, used by
// both the metamodel property validator and the Lua rrule_next helper.
func ValidateRrule(s string) error {
	cleaned := strings.TrimPrefix(s, "RRULE:")

	opt, err := rrule.StrToROption(cleaned)
	if err != nil {
		return fmt.Errorf("invalid RRULE: %w", err)
	}

	if opt.Interval > 1 && opt.Dtstart.IsZero() {
		return errors.New("INTERVAL > 1 requires DTSTART in the RRULE string")
	}

	return nil
}
