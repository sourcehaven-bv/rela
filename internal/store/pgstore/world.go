package pgstore

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// worldSQL builds the two expressions a world-scoped query needs: the
// per-row RANK (lower wins within a family) and the CANDIDATE predicate
// (which rows may compete at all). TKT-WAV8XP PR-C.
//
// A world is a per-FAMILY ranked preference, not a row predicate: at most
// one state per entity survives, and which one wins depends on what else
// exists for that id. `face IN (a, b)` cannot express that — an entity
// holding both coordinates would yield two rows and break the
// at-most-one-prime invariant everything leans on. Hence the SQL shape
// `DISTINCT ON (id) ... ORDER BY id, rank`.
//
// Rank semantics mirror [storeutil.WorldPrimes] exactly, so pgstore and
// fs/mem cannot drift — the conformance suite asserts ONE contract for
// all three backends:
//
//   - a chain coordinate at position i ranks i (lower wins);
//   - under `otherwise: default` the DEFAULT row ranks len(chain), so it
//     loses to every real coordinate but still beats absence;
//   - under `otherwise: exclude` the default row is not a candidate
//     unless the chain names it — absence IS the publication bit;
//   - a type ABSENT from the scope keeps its default state (rule 1),
//     which is the trailing arm of both expressions.
//
// Coordinates come from the compiled WorldScope, never from user input,
// and are bound as parameters rather than interpolated. Types are sorted
// so the generated SQL is deterministic: a stable string keeps PostgreSQL's
// plan cache warm and makes the builder's unit tests readable.
//
// The caller must have established the world is non-default; passing the
// default world here would generate a pointless CASE over zero types.
//
// alias qualifies the column references ("e" for the graph queries,
// which alias the entities table; empty for the flat entity listings).
// It is a PARAMETER rather than a post-hoc string rewrite of the result:
// rewriting generated SQL with ReplaceAll would corrupt any column whose
// name merely CONTAINS "face" — from_face is one — and would break
// silently the moment this function emits a new column.
func worldSQL(w store.WorldScope, alias string, args *[]any) (rank, candidate string) {
	col := func(name string) string {
		if alias == "" {
			return name
		}
		return alias + "." + name
	}
	types := w.Types()
	sort.Strings(types)

	var rankArms, candArms, typeTests []string
	for _, typ := range types {
		res, ok := w.For(typ)
		if !ok {
			continue // unreachable: Types() lists only scoped types
		}
		*args = append(*args, typ)
		typeArg := len(*args)
		typeTests = append(typeTests, fmt.Sprintf("%s = $%d", col("type"), typeArg))

		var whens, coords []string
		for i, coord := range res.Chain {
			*args = append(*args, string(coord))
			c := len(*args)
			whens = append(whens, fmt.Sprintf("WHEN %s = $%d THEN %d", col("face"), c, i))
			coords = append(coords, fmt.Sprintf("%s = $%d", col("face"), c))
		}
		if res.Fallback == store.FallbackDefaultState {
			// Last resort: must rank BELOW every coordinate, hence
			// len(chain) rather than 0.
			whens = append(whens, fmt.Sprintf("WHEN %s = '' THEN %d", col("face"), len(res.Chain)))
			coords = append(coords, col("face")+" = ''")
		}

		if len(coords) == 0 {
			// Empty chain under `otherwise: exclude`: the type
			// contributes nothing. An explicit false arm keeps the
			// trailing arm meaning ONLY "unscoped type" (rule 1)
			// rather than silently absorbing this case.
			rankArms = append(rankArms, fmt.Sprintf("WHEN %s = $%d THEN 0", col("type"), typeArg))
			candArms = append(candArms, fmt.Sprintf("(%s = $%d AND false)", col("type"), typeArg))
			continue
		}
		rankArms = append(rankArms,
			fmt.Sprintf("WHEN %s = $%d THEN (CASE %s ELSE 0 END)", col("type"), typeArg, strings.Join(whens, " ")))
		candArms = append(candArms,
			fmt.Sprintf("(%s = $%d AND (%s))", col("type"), typeArg, strings.Join(coords, " OR ")))
	}

	if len(rankArms) == 0 {
		// No scoped types: every type takes rule 1.
		return "0", col("face") + " = ''"
	}

	rank = "CASE " + strings.Join(rankArms, " ") + " ELSE 0 END"
	// Rule 1 as the trailing arm: a type the world says nothing about
	// contributes its default state, and ranks 0 because it is then the
	// only candidate in its family.
	candidate = "(" + strings.Join(candArms, " OR ") +
		" OR (NOT (" + strings.Join(typeTests, " OR ") + ") AND " + col("face") + " = ''))"
	return rank, candidate
}
