//go:build postgres || sqlite

package cli

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// printDerivedDrift prints the reconcile outcomes and reports whether any object
// drifted (was/would-be created, dropped, or is unenforced). When dryRun, the
// verbs are phrased as "would ...".
func printDerivedDrift(outcomes []store.DerivedObjectOutcome, dryRun bool) (drift bool) {
	var enforced int
	for _, o := range outcomes {
		switch o.State {
		case store.DerivedEnforced:
			enforced++
		case store.DerivedCreated:
			drift = true
			verb := "created"
			if dryRun {
				verb = "would create"
			}
			fmt.Printf("  + %s %s\n", verb, describeDerived(o.Spec))
		case store.DerivedDropped:
			drift = true
			verb := "dropped"
			if dryRun {
				verb = "would drop"
			}
			fmt.Printf("  - %s %s\n", verb, o.Reason)
		case store.DerivedUnenforced:
			drift = true
			if o.Spec.Kind == store.DerivedUnique {
				fmt.Printf("  ! NOT enforced: %s: %s (%d duplicate value group(s))\n",
					describeDerived(o.Spec), o.Reason, o.BlockingCount)
			} else {
				fmt.Printf("  ! NOT created: %s: %s\n", describeDerived(o.Spec), o.Reason)
			}
			for _, v := range o.SampleValues {
				fmt.Printf("      duplicate value: %s\n", v)
			}
		}
	}
	if !drift {
		fmt.Printf("Derived schema: up to date (%d object(s) enforced).\n", enforced)
	}
	return drift
}

// describeDerived names a derived object for the drift report.
func describeDerived(spec store.DerivedObjectSpec) string {
	switch spec.Kind {
	case store.DerivedUnique:
		return fmt.Sprintf("unique constraint on %s.%s", spec.Type, spec.Property)
	case store.DerivedListIndex:
		return fmt.Sprintf("list index on %s.%v ordered by %v", spec.Type, spec.Properties, spec.OrderBy)
	default:
		return fmt.Sprintf("query index on %s.%v", spec.Type, spec.Properties)
	}
}
