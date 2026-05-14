package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
)

// errStrictWarnings is the sentinel error returned by CLI commands
// when --strict is set and the underlying write surfaced soft
// validation warnings. Cobra renders the error and exits non-zero.
var errStrictWarnings = errors.New("validation warnings (--strict)")

// printValidationWarnings prints DEC-HWZHA soft-validation warnings to
// stderr in a stable scriptable format:
//
//	WARNING: <code> at <path>: <detail>
//
// One line per warning. Stderr (not stdout) so the command's normal
// output pipeline isn't polluted. Returns silently when the slice is
// empty.
func printValidationWarnings(warnings []entitymanager.Warning) {
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "WARNING: %s at %s: %s\n", w.Code, w.Path, w.Detail)
	}
}
