package pgstore

import (
	"context"
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
func (s *Store) GraphQuery(ctx context.Context, q store.GraphQuery) iter.Seq2[*entity.Entity, error] {
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
	matchedSQL, matchedArgs := buildGraphQuerySQL(q, true)
	if err = s.db.QueryRow(ctx, matchedSQL, matchedArgs...).Scan(&matched); err != nil {
		return 0, 0, fmt.Errorf("pgstore: graph count (matched): %w", err)
	}
	if err = s.db.QueryRow(ctx, `SELECT count(*) FROM entities WHERE type = $1`, q.EntityType).Scan(&total); err != nil {
		return 0, 0, fmt.Errorf("pgstore: graph count (total): %w", err)
	}
	return matched, total, nil
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
// InheritThrough / EntityInheritThrough, Depth, EntityDepth) flows
// through [sqlBuilder.arg], which returns a positional placeholder
// (`$N`) and appends the value to args. The Sprintf calls in this
// file substitute only:
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

	var withParts []string
	var existsParts []string

	if q.HasInbound != nil {
		w, ex := buildPredicateSQL(b, "in", *q.HasInbound, typeArg, store.DirectionIncoming)
		withParts = append(withParts, w...)
		existsParts = append(existsParts, ex)
	}
	if q.HasOutbound != nil {
		w, ex := buildPredicateSQL(b, "out", *q.HasOutbound, typeArg, store.DirectionOutgoing)
		withParts = append(withParts, w...)
		existsParts = append(existsParts, ex)
	}

	// Branch the SELECT list + ORDER BY: count queries skip column
	// fetching and ordering; row queries return the standard entity
	// columns and stable id-ascending order. The rest of the query
	// (WITH, FROM, WHERE, EXISTS chain) is identical.
	selectList := "e.id, e.type, e.properties, e.content, e.updated_at"
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
	sb.WriteString("SELECT " + selectList + " FROM entities e WHERE e.type = " + typeArg)
	if len(q.WhereIDs) > 0 {
		whereIDsArg := b.arg(q.WhereIDs)
		sb.WriteString(" AND e.id = ANY(" + whereIDsArg + ")")
	}
	for _, ex := range existsParts {
		sb.WriteString(" AND EXISTS (")
		sb.WriteString(ex)
		sb.WriteByte(')')
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
	endpointsArg := b.arg(p.Endpoints)

	// endpointSrc: SQL expression yielding the set of endpoint IDs
	// the EXISTS clause matches against. Without InheritThrough this
	// is just the unnested input; with it, a recursive CTE.
	endpointSrc := fmt.Sprintf(`SELECT unnest(%s::text[]) COLLATE "C"`, endpointsArg)
	if len(p.InheritThrough) > 0 && p.Depth > 0 {
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
    SELECT e0.id, e0.id, 0 FROM entities e0 WHERE e0.type = %s
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
	fmt.Fprintf(&existsSB, "%s IN (%s) AND %s IN (%s)",
		endpointCol, endpointSrc, entityCol, entityJoin)

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
