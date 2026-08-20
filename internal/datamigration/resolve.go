package datamigration

import (
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Resolve plans which migration files must run to carry a store from its
// current shape to the live schema's shape.
//
// Files run in lexicographic name order (the chain order, like SQL
// migrations); hashes and compatibility are the safety rails, not the
// ordering mechanism. Migrations are edges you MUST take; compatible gaps
// are edges you get for free: a store whose current shape differs from a
// file's `from` may still take the edge when CompareShapes classifies the
// gap as compatible (that difference was — or would have been — adopted by
// the gate). This is what lets multi-tenant stores at different hashes catch
// up without no-op migrations for additive changes.
//
// A file is skipped when it is already recorded in the marker's applied
// list, or when its `to` shape is where the walk already stands. After the
// last file, the remaining gap to the live schema must itself be compatible
// or Resolve fails, naming the deltas that still need a migration.
//
// Taking a free edge REBASES the walk onto the file's projections: any
// compatible divergence the store carried (an adopted-but-unmigrated
// additive property, dropped drift) is not represented in the marker the
// runner writes afterwards. That is safe by construction — the divergence
// was compatible, so the next gate evaluation re-classifies it against the
// live schema and re-adopts (additive) or re-ledgers (drift; the GC grace
// clock restarts, which fails toward retention, never toward deletion).
func Resolve(
	current metamodel.ShapeProjection, applied []string,
	live metamodel.ShapeProjection, files []*File,
) ([]*File, error) {
	currentHash := current.Hash()
	liveHash := live.Hash()

	appliedSet := make(map[string]bool, len(applied))
	for _, name := range applied {
		appliedSet[name] = true
	}

	pos := current
	posHash := currentHash
	var plan []*File
	for _, f := range files {
		if appliedSet[f.Name] {
			continue
		}
		if f.To == posHash {
			// The store already conforms to this file's outcome (it was
			// applied before the applied list existed, or the operator made
			// the same change by hand). Nothing to run.
			continue
		}
		if f.From != posHash {
			gap := metamodel.CompareShapes(pos, f.FromProjection)
			if !gap.Compatible() {
				return nil, incompatibleGapError(
					fmt.Sprintf("cannot reach migration %s: the store's shape (%s) differs incompatibly from the migration's from-shape (%s)",
						f.Name, short(posHash), short(f.From)), gap)
			}
		}
		plan = append(plan, f)
		pos = f.ToProjection
		posHash = f.To
	}

	if posHash != liveHash {
		gap := metamodel.CompareShapes(pos, live)
		if !gap.Compatible() {
			return nil, incompatibleGapError(
				fmt.Sprintf("after all migrations the store's shape (%s) still differs incompatibly from the live schema (%s) — run `rela migrate gen` to draft the missing migration",
					short(posHash), short(liveHash)), gap)
		}
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
