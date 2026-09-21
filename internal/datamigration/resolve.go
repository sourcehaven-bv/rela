package datamigration

import (
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Resolve plans which migration files must run to carry a store from its
// current shape to the live schema's shape.
//
// A file runs when its NAME is absent from the store's applied list. That is
// the whole rule, and it is the one every mainstream migration tool uses
// (Rails' schema_migrations, Django's django_migrations, Flyway's
// flyway_schema_history). Files run in lexicographic name order, which for the
// timestamp-prefixed names the generator mints is chronological order.
//
// Keying on the name rather than on a shape edge is what makes a DATA-ONLY
// migration expressible: a backfill, a de-duplication or a correction of values
// an old bug wrote has the same schema shape before and after, so under the
// previous hash-edge model it could not be represented at all (TKT-XCJ0Y2).
//
// # What the applied list does and does not guarantee
//
// It is now the ONLY thing preventing a double-apply — the previous model had
// a second, independent check (skip a file whose to-shape the store had
// already reached) that a name-keyed list cannot express. Step idempotency is
// therefore load-bearing rather than merely advisable: every step must be safe
// to re-run, because re-running after a crash is the documented recovery path
// and a restored-from-backup applied list will replay whatever it has
// forgotten.
//
// # The residual shape check
//
// After the plan, the remaining gap between the last file's to-shape (or the
// store's current shape, when nothing is pending) and the live schema must be
// compatible. That is not what decides the plan — it is a diagnostic, and it
// is how an operator learns they edited schema.yaml without writing the
// migration it needs.
func Resolve(
	current metamodel.ShapeProjection, applied []string,
	live metamodel.ShapeProjection, files []*File,
) ([]*File, error) {
	appliedSet := make(map[string]bool, len(applied))
	for _, name := range applied {
		appliedSet[name] = true
	}

	// pos tracks the shape the store will conform to once the plan has run, so
	// the residual check below compares the live schema against the END of the
	// plan rather than against where the store stands today.
	pos := current
	var plan []*File
	for _, f := range files {
		if appliedSet[f.Name] {
			continue
		}
		plan = append(plan, f)
		pos = f.ToProjection
	}

	if gap := metamodel.CompareShapes(pos, live); !gap.Compatible() {
		return nil, incompatibleGapError(
			"the schema has changed in a way the pending migrations do not cover "+
				"— run `rela migrate gen` to draft the missing migration", gap)
	}
	return plan, nil
}

func incompatibleGapError(msg string, gap metamodel.ShapeReport) error {
	var b strings.Builder
	b.WriteString("datamigration: ")
	b.WriteString(msg)
	for _, d := range gap.ByTier(metamodel.TierMigration) {
		b.WriteString("\n  - ")
		b.WriteString(d.Detail)
	}
	return fmt.Errorf("%s", b.String())
}
