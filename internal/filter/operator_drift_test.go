package filter_test

import (
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// internal/metamodel duplicates this package's operator set to extract the
// property name from a validation rule's `where:` clause — it cannot import
// filter (arch-lint). The copy is only used to decide whether to RUN a
// load-time check, so drift costs a diagnostic rather than correctness. Still
// worth catching: a new operator here would silently stop that check firing
// for clauses using it.
//
// The guard lives on this side because this is the definition; the copy is
// exported solely so this test can see it.
func TestOperatorSetsAgree(t *testing.T) {
	mine := filter.Operators()
	theirs := metamodel.WhereOperators()

	slices.Sort(mine)
	slices.Sort(theirs)
	if !slices.Equal(mine, theirs) {
		t.Errorf("operator sets have drifted:\n  internal/filter:    %v\n  internal/metamodel: %v\n"+
			"update metamodel.whereOperators to match", mine, theirs)
	}
}
