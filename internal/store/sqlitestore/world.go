package sqlitestore

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// worldSQL builds the two expressions a world-scoped query needs: the per-row
// RANK (lower wins within a family) and the CANDIDATE predicate (which rows may
// compete at all). It is the SQLite twin of pgstore's worldSQL and must stay
// semantically identical to it — the conformance suite asserts ONE contract
// across all backends.
//
// SQL rather than [storeutil.WorldPrimes], which is the fs/mem helper. The
// reason is the same one that made pgstore choose SQL: those two backends
// resolve against an in-memory index they already hold, so buffering a family
// costs nothing, whereas here every candidate row is a row fetched from the
// database. Pushing rank + candidate into the query lets ONE statement return
// the primes — pagination included — instead of over-fetching whole families
// into Go to discard most of them. It is also what keeps LIMIT meaningful:
// resolving in Go would make LIMIT count candidate rows rather than primes.
//
// A world is a per-FAMILY ranked preference, not a row predicate: at most one
// state per entity survives, and which one wins depends on what else exists for
// that id. `face IN (a, b)` cannot express that — an entity holding both
// coordinates would yield two rows and break the at-most-one-prime invariant.
// SQLite has no DISTINCT ON, so callers pair these expressions with
// ROW_NUMBER() OVER (PARTITION BY id ORDER BY rank), which is the equivalent.
//
// Rank semantics mirror [storeutil.WorldPrimes] and pgstore exactly:
//
//   - a chain coordinate at position i ranks i (lower wins);
//   - under `otherwise: default` the DEFAULT row ranks len(chain), so it loses
//     to every real coordinate but still beats absence;
//   - under `otherwise: exclude` the default row is not a candidate unless the
//     chain names it — absence IS the publication bit;
//   - a type ABSENT from the scope keeps its default state (rule 1), which is
//     the trailing arm of both expressions.
//
// Coordinates come from the compiled WorldScope, never from user input, and are
// bound as parameters rather than interpolated. Types are sorted so the
// generated SQL is deterministic, which keeps SQLite's statement cache warm and
// the builder's unit tests readable.
//
// # Why each expression carries its OWN args
//
// SQLite binds POSITIONAL '?' placeholders, where pgstore binds numbered $N.
// pgstore can therefore emit a coordinate once and reference it from both
// expressions; here every placeholder consumes the next argument in statement
// order, so the two expressions cannot share bindings. Returning
// (expr, args) pairs makes that explicit — a caller splices each expression
// wherever it belongs and appends the matching args at that point, and the
// compiler stops it pairing one expression with the other's bindings.
//
// The caller must have established the world is non-default; passing the
// default world here would generate a pointless CASE over zero types.
//
// alias qualifies the column references (empty for the flat entity listings).
// It is a PARAMETER rather than a post-hoc string rewrite of the result:
// rewriting generated SQL with ReplaceAll would corrupt any column whose name
// merely CONTAINS "face" — from_face is one — and would break silently the
// moment this function emits a new column.
func worldSQL(w store.WorldScope, alias string) (rank string, rankArgs []any, candidate string, candArgs []any) {
	col := func(name string) string {
		if alias == "" {
			return name
		}
		return alias + "." + name
	}
	types := w.Types()
	sort.Strings(types)

	var rankArms, candArms, typeTests []string
	var typeTestArgs []any
	for _, typ := range types {
		res, ok := w.For(typ)
		if !ok {
			continue // unreachable: Types() lists only scoped types
		}
		typeTests = append(typeTests, col("type")+" = ?")
		typeTestArgs = append(typeTestArgs, typ)

		var whens, coords []string
		var whenArgs, coordArgs []any
		for i, coord := range res.Chain {
			whens = append(whens, fmt.Sprintf("WHEN %s = ? THEN %d", col("face"), i))
			whenArgs = append(whenArgs, string(coord))
			coords = append(coords, col("face")+" = ?")
			coordArgs = append(coordArgs, string(coord))
		}
		if res.Fallback == store.FallbackDefaultState {
			// Last resort: must rank BELOW every coordinate, hence len(chain)
			// rather than 0.
			whens = append(whens, fmt.Sprintf("WHEN %s = '' THEN %d", col("face"), len(res.Chain)))
			coords = append(coords, col("face")+" = ''")
		}

		if len(coords) == 0 {
			// Empty chain under `otherwise: exclude`: the type contributes
			// nothing. An explicit false arm keeps the trailing arm meaning
			// ONLY "unscoped type" (rule 1) rather than silently absorbing
			// this case. SQLite has no boolean literal, so 0 is false.
			rankArms = append(rankArms, "WHEN "+col("type")+" = ? THEN 0")
			rankArgs = append(rankArgs, typ)
			candArms = append(candArms, "("+col("type")+" = ? AND 0)")
			candArgs = append(candArgs, typ)
			continue
		}
		rankArms = append(rankArms,
			"WHEN "+col("type")+" = ? THEN (CASE "+strings.Join(whens, " ")+" ELSE 0 END)")
		rankArgs = append(rankArgs, typ)
		rankArgs = append(rankArgs, whenArgs...)

		candArms = append(candArms,
			"("+col("type")+" = ? AND ("+strings.Join(coords, " OR ")+"))")
		candArgs = append(candArgs, typ)
		candArgs = append(candArgs, coordArgs...)
	}

	if len(rankArms) == 0 {
		// No scoped types: every type takes rule 1.
		return "0", nil, col("face") + " = ''", nil
	}

	rank = "CASE " + strings.Join(rankArms, " ") + " ELSE 0 END"
	// Rule 1 as the trailing arm: a type the world says nothing about
	// contributes its default state, and ranks 0 because it is then the only
	// candidate in its family.
	candidate = "(" + strings.Join(candArms, " OR ") +
		" OR (NOT (" + strings.Join(typeTests, " OR ") + ") AND " + col("face") + " = ''))"
	candArgs = append(candArgs, typeTestArgs...)
	return rank, rankArgs, candidate, candArgs
}
