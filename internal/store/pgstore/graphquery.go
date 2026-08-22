package pgstore

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/graphquerynaive"
)

// GraphQuery is the SQL-native implementation of [store.GraphQueryer].
// Builds a single query (one recursive CTE per active transitive
// expansion + WHERE EXISTS for the predicate match) and streams rows.
//
// When no predicate is configured the query collapses to a plain
// SELECT by type; when only one of HasInbound / HasOutbound is set,
// only that EXISTS clause is emitted. Both nil → degenerate
// "everything of this type" answer (covered by the conformance suite).
// checkGraphQueryScope mirrors checkQueryScope for the graph shape: the
// world must reach BOTH query types or the list path and the ACL
// pushdown path diverge (TKT-WAV8XP F1/F5). Removed in PR-C with its
// EntityQuery twin.
func checkGraphQueryScope(q store.GraphQuery) error {
	if !q.World.IsDefaultWorld() {
		return errors.New(
			"pgstore: world-scoped graph queries are not implemented yet (TKT-WAV8XP PR-C); " +
				"the SQL pushdown lands there and this refusal is removed with it")
	}
	return nil
}

func (s *Store) GraphQuery(ctx context.Context, q store.GraphQuery) iter.Seq2[*entity.Entity, error] {
	if err := checkGraphQueryScope(q); err != nil {
		return func(yield func(*entity.Entity, error) bool) { yield(nil, err) }
	}
	sqlText, args := buildGraphQuerySQL(q, false)
	return func(yield func(*entity.Entity, error) bool) {
		rows, err := s.db.Query(ctx, sqlText, args...)
		if err != nil {
			yield(nil, fmt.Errorf("pgstore: graph query: %w", err))
			return
		}
		defer rows.Close()
		for rows.Next() {
			e, scanErr := scanEntity(rows)
			if scanErr != nil {
				if !yield(nil, scanErr) {
					return
				}
				continue
			}
			if !yield(e, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(nil, err)
		}
	}
}

// GraphCount runs the predicate query as `SELECT count(*)` and a
// separate unconditional count of entities of the type. Two
// round-trips beats one COUNT FILTER because both rely on the same
// recursive CTE shape — duplicating the WITH RECURSIVE inside a
// single FILTER expression saves nothing.
func (s *Store) GraphCount(ctx context.Context, q store.GraphQuery) (matched, total int, err error) {
	if scopeErr := checkGraphQueryScope(q); scopeErr != nil {
		return 0, 0, scopeErr
	}
	matchedSQL, matchedArgs := buildGraphQuerySQL(q, true)
	if err = s.db.QueryRow(ctx, matchedSQL, matchedArgs...).Scan(&matched); err != nil {
		return 0, 0, fmt.Errorf("pgstore: graph count (matched): %w", err)
	}
	if err = s.db.QueryRow(ctx,
		`SELECT count(*) FROM entities WHERE type = $1 AND pointer = ''`, q.EntityType).Scan(&total); err != nil {
		return 0, 0, fmt.Errorf("pgstore: graph count (total): %w", err)
	}
	return matched, total, nil
}

// MatchingIDs runs the predicate query restricted to the candidate id
// set via `e.id = ANY($ids)`, returning a map keyed by every input id
// (true = matched, false = no-match). Push-down: a single SQL round
// trip regardless of |ids|.
func (s *Store) MatchingIDs(ctx context.Context, q store.GraphQuery, ids []string) (map[string]bool, error) {
	if err := checkGraphQueryScope(q); err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = false
	}
	if len(out) == 0 {
		return out, nil
	}
	sqlText, args := buildMatchingIDsSQL(q, ids)
	rows, err := s.db.Query(ctx, sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("pgstore: matching ids: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("pgstore: matching ids scan: %w", err)
		}
		out[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgstore: matching ids: %w", err)
	}
	return out, nil
}

// buildMatchingIDsSQL builds the same query shape as
// [buildGraphQuerySQL] but selects only `e.id` and restricts the
// candidate set via `e.id = ANY(:ids)`. Parameterised — ids never
// reaches the SQL text.
func buildMatchingIDsSQL(q store.GraphQuery, ids []string) (sqlText string, args []any) {
	b := &sqlBuilder{}
	typeArg := b.arg(q.EntityType)

	withParts, condParts := buildPredicateParts(b, q, typeArg)
	idsArg := b.arg(ids)

	var sb strings.Builder
	if len(withParts) > 0 {
		sb.WriteString("WITH RECURSIVE ")
		sb.WriteString(strings.Join(withParts, ",\n"))
		sb.WriteByte('\n')
	}
	// e.pointer = '': graph queries evaluate the default world
	// (TKT-DOFYR1); relation traversal stays tail-unscoped to match
	// graphquerynaive over ListRelations.
	sb.WriteString("SELECT e.id FROM entities e WHERE e.pointer = '' AND e.type = " + typeArg)
	sb.WriteString(" AND e.id = ANY(" + idsArg + ")")
	for _, c := range condParts {
		sb.WriteString(" AND " + c)
	}
	return sb.String(), b.args
}

// buildPredicateParts emits the CTE definitions and the WHERE-clause
// conjuncts shared by [buildMatchingIDsSQL] and [buildGraphQuerySQL] —
// relation predicates (as EXISTS / NOT EXISTS) and property predicates
// (as jsonb comparisons). Kept in one place so the two query shapes
// cannot drift.
func buildPredicateParts(b *sqlBuilder, q store.GraphQuery, typeArg string) (with, conds []string) {
	if q.HasInbound != nil {
		w, ex := buildPredicateSQL(b, "in", *q.HasInbound, typeArg, store.DirectionIncoming)
		with = append(with, w...)
		conds = append(conds, existsCond(ex, q.HasInbound.Negate))
	}
	if q.HasOutbound != nil {
		w, ex := buildPredicateSQL(b, "out", *q.HasOutbound, typeArg, store.DirectionOutgoing)
		with = append(with, w...)
		conds = append(conds, existsCond(ex, q.HasOutbound.Negate))
	}
	for _, p := range q.Props {
		conds = append(conds, propCond(b, p))
	}
	return with, conds
}

// existsCond wraps an EXISTS body, negating it for an absence query.
func existsCond(body string, negate bool) string {
	if negate {
		return "NOT EXISTS (" + body + ")"
	}
	return "EXISTS (" + body + ")"
}

// propCond renders one property predicate as a jsonb comparison.
//
// This MUST agree with [propmatch.Decide] on every value shape, because
// the same predicate is answered by the naive backend on fs/mem — the
// backend-parity rule in CLAUDE.md. Two shapes need explicit handling
// beyond a plain `->>` comparison:
//
//   - LISTS (multi-select). `->>` renders an array as its JSON text
//     (`["a", "b"]`), so a plain `= 'a'` would never match, while the
//     naive side matches if ANY element equals the target. Equality
//     therefore branches to the containment operator `?` for arrays.
//     Likewise an EMPTY array is empty to the naive side but renders as
//     the two-character string `[]`, which is neither NULL nor blank.
//   - ABSENT vs JSON null vs empty string. All three are the SAME
//     "empty" state; `->>` already collapses the first two to SQL NULL.
//
// The NOT-equal cases carry an explicit emptiness guard deliberately.
// SQL's three-valued logic drops a NULL row on its own (NULL <> 'x' is
// NULL, not true), but a present-but-empty value would otherwise
// satisfy the comparison and widen the filter to include unset rows —
// the asymmetry pinned by the conformance suite. COALESCE is needed on
// the `?` operator for the same reason: it yields NULL, not false, for
// a missing key.
func propCond(b *sqlBuilder, p store.PropPredicate) string {
	propArg := b.arg(p.Property)
	txt := fmt.Sprintf("(e.properties ->> %s)", propArg)
	jsn := fmt.Sprintf("(e.properties -> %s)", propArg)

	// isEmpty covers: missing key, JSON null, empty string, empty array.
	isEmpty := fmt.Sprintf(
		"(%s IS NULL OR %s = '' OR (jsonb_typeof(%s) = 'array' AND jsonb_array_length(%s) = 0))",
		txt, txt, jsn, jsn)

	switch {
	case p.Value == "" && p.Op == store.PropEqual:
		return isEmpty
	case p.Value == "" && p.Op == store.PropNotEqual:
		return "NOT " + isEmpty
	case p.Op == store.PropNotEqual:
		return fmt.Sprintf("(NOT %s AND NOT %s)", isEmpty, equalsCond(b, txt, jsn, p.Value))
	default:
		return equalsCond(b, txt, jsn, p.Value)
	}
}

// equalsCond renders value equality, matching [propmatch] semantics: a
// scalar compares by its text form, an array matches when ANY element
// equals the target (multi-select). COALESCE keeps the containment test
// false rather than NULL when the key is absent, so a surrounding NOT
// behaves.
func equalsCond(b *sqlBuilder, txt, jsn, value string) string {
	valArg := b.arg(value)
	return fmt.Sprintf(
		"(CASE WHEN jsonb_typeof(%s) = 'array' THEN COALESCE(%s ? %s, false) ELSE %s = %s END)",
		jsn, jsn, valArg, txt, valArg)
}

// buildGraphQuerySQL emits the SQL + args list for a GraphQuery. When
// countOnly is true the outer SELECT becomes `SELECT count(*)`
// instead of streaming entity columns. The function is exported via
// package-private tests so the CTE shape can be pinned without
// running against a live database.
//
// The query is built up in pieces so each optional predicate / CTE
// appears only when it can contribute. This keeps query plans honest:
// PostgreSQL's planner won't optimize away a CTE that's "always
// trivial", so collapsing them in Go saves real work.
//
// **SQL injection safety.** Every caller-supplied value
// (q.EntityType, RelationPredicate.Endpoints / OfTypes /
// InheritThrough / EntityInheritThrough, Depth, EntityDepth, and the
// MatchingIDs candidate id slice) flows through [sqlBuilder.arg],
// which returns a positional placeholder (`$N`) and appends the
// value to args. The Sprintf calls in this file substitute only:
//
//   - those placeholder strings (already `$N`-formatted by arg)
//   - compile-time string literals (CTE names like
//     `in_endpoint_closure`, column names like `r.from_id`)
//   - the `prefix` argument to buildPredicateSQL, which is one of
//     the in-package constants `"in"` / `"out"`
//
// User data never reaches the SQL text. The same property holds
// when [BuildGraphQuerySQLForTest] is invoked from tests — the
// builder treats all input the same way.
func buildGraphQuerySQL(q store.GraphQuery, countOnly bool) (sqlText string, args []any) {
	b := &sqlBuilder{}
	// $1 is always q.EntityType.
	typeArg := b.arg(q.EntityType)

	withParts, condParts := buildPredicateParts(b, q, typeArg)

	// Branch the SELECT list + ORDER BY: count queries skip column
	// fetching and ordering; row queries return the standard entity
	// columns and stable id-ascending order. The rest of the query
	// (WITH, FROM, WHERE, EXISTS chain) is identical.
	selectList := "e.id, e.type, e.pointer, e.properties, e.content, e.updated_at"
	orderBy := " ORDER BY e.id"
	if countOnly {
		selectList = "count(*)"
		orderBy = ""
	}

	var sb strings.Builder
	if len(withParts) > 0 {
		sb.WriteString("WITH RECURSIVE ")
		sb.WriteString(strings.Join(withParts, ",\n"))
		sb.WriteByte('\n')
	}
	sb.WriteString("SELECT " + selectList + " FROM entities e WHERE e.pointer = '' AND e.type = " + typeArg)
	for _, c := range condParts {
		sb.WriteString(" AND " + c)
	}
	sb.WriteString(orderBy)

	return sb.String(), b.args
}

// buildPredicateSQL emits (CTE definitions, EXISTS clause) for one
// HasInbound / HasOutbound predicate. The CTE name uses prefix so
// in/out predicates don't collide when both are set.
//
// The endpoint and entity expansions are independent: each emits its
// own CTE only when [Predicate.InheritThrough] / EntityInheritThrough
// is non-empty AND the corresponding Depth is > 0. When omitted, the
// EXISTS query references the seed directly.
func buildPredicateSQL(
	b *sqlBuilder, prefix string,
	p store.RelationPredicate, typeArg string, dir store.Direction,
) (with []string, exists string) {
	// The endpoints arg is registered LAZILY: with no endpoints named
	// ("any endpoint") nothing references it, and an unreferenced
	// placeholder makes PostgreSQL reject the statement with "could not
	// determine data type of parameter" (42P18).
	var endpointsArg string
	if len(p.Endpoints) > 0 {
		endpointsArg = b.arg(p.Endpoints)
	}

	// endpointSrc: SQL expression yielding the set of endpoint IDs
	// the EXISTS clause matches against. Without InheritThrough this
	// is just the unnested input; with it, a recursive CTE.
	var endpointSrc string
	if endpointsArg != "" {
		endpointSrc = fmt.Sprintf(`SELECT unnest(%s::text[]) COLLATE "C"`, endpointsArg)
	}
	if len(p.Endpoints) > 0 && len(p.InheritThrough) > 0 && p.Depth > 0 {
		throughArg := b.arg(p.InheritThrough)
		depthArg := b.arg(cappedDepth(p.Depth))
		cteName := prefix + "_endpoint_closure"
		with = append(with, fmt.Sprintf(`%s(id, depth) AS (
    SELECT unnest(%s::text[]) COLLATE "C", 0
    UNION
    SELECT r.to_id, c.depth + 1
    FROM relations r
    JOIN %s c ON r.from_id = c.id
    WHERE r.rel_type = ANY(%s)
      AND c.depth < %s
)`, cteName, endpointsArg, cteName, throughArg, depthArg))
		endpointSrc = "SELECT id FROM " + cteName
	}

	// entitySrc: SQL expression yielding (id, root) pairs for the
	// candidate-entity expansion. Without EntityInheritThrough each
	// entity maps to itself; with it, a recursive CTE that walks
	// ancestors and remembers the original root entity ID.
	entityJoin := "e.id"
	if len(p.EntityInheritThrough) > 0 && p.EntityDepth > 0 {
		entityThroughArg := b.arg(p.EntityInheritThrough)
		entityDepthArg := b.arg(cappedDepth(p.EntityDepth))
		cteName := prefix + "_entity_closure"
		with = append(with, fmt.Sprintf(`%s(id, root, depth) AS (
    SELECT e0.id, e0.id, 0 FROM entities e0 WHERE e0.pointer = '' AND e0.type = %s
    UNION
    SELECT r.to_id, c.root, c.depth + 1
    FROM relations r
    JOIN %s c ON r.from_id = c.id
    WHERE r.rel_type = ANY(%s)
      AND c.depth < %s
)`, cteName, typeArg, cteName, entityThroughArg, entityDepthArg))
		entityJoin = fmt.Sprintf("(SELECT id FROM %s WHERE root = e.id)", cteName)
	}

	// Direction picks which side of the relation matches the endpoint.
	endpointCol, entityCol := "r.from_id", "r.to_id"
	if dir == store.DirectionOutgoing {
		endpointCol, entityCol = "r.to_id", "r.from_id"
	}

	// Build the EXISTS body. OfTypes is optional: when omitted, all
	// relation types match (consistent with naive impl's behavior).
	var existsSB strings.Builder
	existsSB.WriteString("SELECT 1 FROM relations r WHERE ")
	if len(p.OfTypes) > 0 {
		typesArg := b.arg(p.OfTypes)
		fmt.Fprintf(&existsSB, "r.rel_type = ANY(%s) AND ", typesArg)
	}
	// An empty Endpoints list means "any endpoint" — the predicate is
	// then purely about the edge existing, which is what an absence
	// query ("has no implements edge at all") needs. Emitting the
	// endpoint constraint against an empty array would match nothing and
	// make every negated any-endpoint query match everything.
	if len(p.Endpoints) > 0 {
		fmt.Fprintf(&existsSB, "%s IN (%s) AND ", endpointCol, endpointSrc)
	}
	fmt.Fprintf(&existsSB, "%s IN (%s)", entityCol, entityJoin)

	return with, existsSB.String()
}

// cappedDepth bounds depth at the naive impl's cap so the SQL and
// the Go impl agree on the recursion ceiling. Negative inputs are
// treated as 0 (no expansion); the conformance suite's Depth=0
// no-op case pins this.
func cappedDepth(d int) int {
	if d < 0 {
		return 0
	}
	if d > graphquerynaive.DepthCap {
		return graphquerynaive.DepthCap
	}
	return d
}

// sqlBuilder accumulates positional placeholders and their values.
// Each call to arg appends to args and returns the matching $N.
// Building queries this way (instead of fmt-substituting values into
// the string) keeps every value parameterised — never interpolated
// into SQL text, never a SQL-injection surface even when callers
// pass arbitrary strings.
type sqlBuilder struct {
	args []any
}

func (b *sqlBuilder) arg(v any) string {
	b.args = append(b.args, v)
	return fmt.Sprintf("$%d", len(b.args))
}
