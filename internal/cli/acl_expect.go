package cli

import (
	"fmt"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/aclmap"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// loadExpectation reads and validates the expectations file at path; see
// [aclmap.Expectation] for its shape.
func loadExpectation(path string, meta *metamodel.Metamodel) (*aclmap.Expectation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read expectations file: %w", err)
	}
	return aclmap.ParseExpectation(data, path, meta)
}

// writeExpectationText prints the comparison: `+` for access held but not
// expected, `-` for access expected but not held.
func writeExpectationText(r *aclmap.ExpectationResult) {
	if r.Match {
		out.WriteMessage("Expectations %s: access matches.", r.File)
		return
	}
	out.WriteMessage("Expectations %s: %d difference(s). + held but not expected, - expected but not held.",
		r.File, len(r.Unexpected)+len(r.Missing))
	for _, c := range r.Unexpected {
		out.WriteMessage("  + %s", c)
	}
	for _, c := range r.Missing {
		out.WriteMessage("  - %s", c)
	}
}
