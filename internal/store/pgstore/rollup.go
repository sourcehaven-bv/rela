package pgstore

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The SQL roll-up is the point of this file: without it [store.Rollup]
// falls back to the Go fold, which costs more queries per column.
var _ store.Rollupper = (*Store)(nil)

// Rollup implements [store.Rollupper] in one statement: the related rows
// are the graph source of q.Related (the caller's gate), joined to the
// identity-scoped edges of the sources (through the gated path, when q has
// one) and grouped per source. The fold rules match [store.RollupGeneric],
// which is the reference.
func (s *Store) Rollup(ctx context.Context, q store.RollupQuery) (map[string]store.RollupResult, error) {
	if err := store.ValidateRollupQuery(q); err != nil {
		return nil, err
	}
	if err := checkGraphQueryScope(q.Related); err != nil {
		return nil, err
	}
	for _, step := range q.Path {
		if err := checkGraphQueryScope(step.Through); err != nil {
			return nil, err
		}
	}
	out := map[string]store.RollupResult{}
	if len(q.Sources) == 0 {
		return out, nil
	}
	sqlText, args, err := buildRollupSQL(q)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("pgstore: rollup: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			src   string
			count int
			value *string
		)
		if err := rows.Scan(&src, &count, &value); err != nil {
			return nil, fmt.Errorf("pgstore: rollup scan: %w", err)
		}
		res := store.RollupResult{Count: count}
		if value != nil {
			v, err := rollupValue(q, *value)
			if err != nil {
				return nil, err
			}
			res.Value = v
		}
		out[src] = res
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgstore: rollup: %w", err)
	}
	return out, nil
}

// rollupValue decodes the text the statement returns for the aggregate.
func rollupValue(q store.RollupQuery, text string) (any, error) {
	switch {
	case q.Func == store.RollupLatest:
		// [pick, pick-visible] as JSON.
		var pair [2]any
		if err := json.Unmarshal([]byte(text), &pair); err != nil {
			return nil, fmt.Errorf("pgstore: rollup latest: %w", err)
		}
		if ok, _ := pair[1].(bool); !ok {
			return nil, nil //nolint:nilnil // a hidden pick is "no value", not a failure
		}
		return store.RollupScalar(pair[0]), nil
	case q.Func == store.RollupSum && q.Decimal:
		// numeric sums exactly and keeps the longest scale summed, as the
		// generic fold renders it.
		return text, nil
	case q.Func == store.RollupSum || q.Numeric:
		// The sum is exact numeric; a total outside int64 has no value, as
		// in the generic fold. min and max are always in range.
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return nil, nil //nolint:nilerr,nilnil // out of range is "no value", not a failure
		}
		return n, nil
	default:
		return text, nil
	}
}

// buildRollupSQL renders q as one grouped statement. Every name is a bind
// argument.
func buildRollupSQL(q store.RollupQuery) (sqlText string, args []any, err error) {
	b := &sqlBuilder{}
	with, pairs := rollupPairsSQL(b, q)

	typeArg := b.arg(q.Related.EntityType)
	relWith, source := graphSource(b, q.Related, typeArg, "e.id IN (SELECT p.id FROM "+pairs+" p)")
	with = append(with, relWith...)

	requireWith, require := rowConditionsSQL(b, "rq", q.Require, typeArg)
	pickWith, pick := rowConditionsSQL(b, "pk", q.PickRequire, typeArg)
	with = append(append(with, requireWith...), pickWith...)

	rowsSQL := fmt.Sprintf("SELECT e.id, e.properties, %s AS pickok FROM (SELECT e.* FROM %s) e WHERE %s",
		pick, source, require)

	agg := rollupAggregateSQL(b, q)
	sqlText = withClause(with) + fmt.Sprintf(
		"SELECT p.src, count(*)::int, %s FROM %s p JOIN (%s) x ON x.id = p.id GROUP BY p.src",
		agg, pairs, rowsSQL)
	if b.err != nil {
		return "", nil, b.err
	}
	return sqlText, b.args, nil
}

// rollupPairsSQL renders the walk from q's sources as CTEs and returns the
// name of the last: the distinct (src, id) pairs of a source and an entity
// the last relation reaches from it, before Related gates that entity. A
// path step's pairs keep only the entities its Through selects, so a gated
// out entity breaks every path through it. DISTINCT makes a related row
// reached by several paths count once per source.
func rollupPairsSQL(b *sqlBuilder, q store.RollupQuery) (with []string, last string) {
	prev := ""
	for i, step := range q.Path {
		name := fmt.Sprintf("rup%d", i)
		with = append(with, name+"_raw AS ("+rollupHopSQL(b, q.Sources, prev, step.Relation, step.SourceIsTo)+")")
		typeArg := b.arg(step.Through.EntityType)
		gateWith, source := graphSourcePrefixed(b, name+"_", step.Through, typeArg,
			"e.id IN (SELECT p.id FROM "+name+"_raw p)")
		with = append(with, gateWith...)
		with = append(with, fmt.Sprintf(
			"%s AS (SELECT p.src, p.id FROM %s_raw p WHERE p.id IN (SELECT e.id FROM %s))", name, name, source))
		prev = name
	}
	last = fmt.Sprintf("rup%d", len(q.Path))
	with = append(with, last+" AS ("+rollupHopSQL(b, q.Sources, prev, q.Relation, q.SourceIsTo)+")")
	return with, last
}

// rollupHopSQL selects the distinct (src, id) pairs one identity-scoped
// relation reaches: from the sources when prev is "", else from the ids of
// the pairs CTE prev.
func rollupHopSQL(b *sqlBuilder, sources []string, prev, relation string, sourceIsTo bool) string {
	nearCol, farCol := "from_id", "to_id"
	if sourceIsTo {
		nearCol, farCol = farCol, nearCol
	}
	relArg := b.arg(relation)
	if prev == "" {
		return fmt.Sprintf("SELECT DISTINCT r.%s AS src, r.%s AS id FROM relations r "+
			"WHERE r.rel_type = %s AND r.from_face = '' AND r.%s = ANY(%s)",
			nearCol, farCol, relArg, nearCol, b.arg(sources))
	}
	return fmt.Sprintf("SELECT DISTINCT p.src, r.%s AS id FROM %s p JOIN relations r ON r.%s = p.id "+
		"WHERE r.rel_type = %s AND r.from_face = ''", farCol, prev, nearCol, relArg)
}

// rowConditionsSQL renders conds as one boolean over e: every condition
// holds, a condition holding when any of its queries matches the row.
func rowConditionsSQL(
	b *sqlBuilder, prefix string, conds []store.RowCondition, typeArg string,
) (with []string, cond string) {
	all := make([]string, 0, 1+len(conds))
	all = append(all, "TRUE")
	for i, c := range conds {
		anyOf := []string{"FALSE"}
		for j, when := range c.Any {
			if when == nil {
				anyOf = append(anyOf, "TRUE")
				continue
			}
			w, conj := fieldGrantCond(b, fmt.Sprintf("%s%d_%d_", prefix, i, j), *when, typeArg)
			with = append(with, w...)
			if len(conj) == 0 {
				conj = []string{"TRUE"}
			}
			anyOf = append(anyOf, "("+strings.Join(conj, " AND ")+")")
		}
		all = append(all, "("+strings.Join(anyOf, " OR ")+")")
	}
	return with, "(" + strings.Join(all, " AND ") + ")"
}

// rollupAggregateSQL renders the aggregate column as text, or NULL for a
// count. The value shapes match [store.RollupNumber] and the non-empty
// strings of [store.RollupGeneric].
func rollupAggregateSQL(b *sqlBuilder, q store.RollupQuery) string {
	switch q.Func {
	case store.RollupCount:
		return "NULL::text"
	case store.RollupSum:
		if q.Decimal {
			return "sum(" + decimalSQL(b, q.Value) + ")::text"
		}
		return "sum(" + integralSQL(b, q.Value) + ")::text"
	case store.RollupMin, store.RollupMax:
		fn := "min"
		if q.Func == store.RollupMax {
			fn = "max"
		}
		if q.Numeric {
			return fn + "(" + integralSQL(b, q.Value) + ")::text"
		}
		return fn + "(" + nonEmptyStringSQL(b, q.Value) + ` COLLATE "C")`
	case store.RollupLatest:
		by := nonEmptyStringSQL(b, q.By)
		pick := fmt.Sprintf("jsonb_build_array(x.properties -> %s, x.pickok)", b.arg(q.Value))
		return fmt.Sprintf(`((array_agg(%s ORDER BY %s COLLATE "C" DESC, x.id ASC) FILTER (WHERE %s IS NOT NULL))[1])::text`,
			pick, by, by)
	default:
		return b.fail(fmt.Errorf("%w: rollup: unknown function %d", store.ErrInvalidQuery, q.Func))
	}
}

// integralSQL is the property as numeric when it is an integral JSON
// number in the int64 range, else NULL. The nested CASE keeps the cast from
// running on a non-number.
func integralSQL(b *sqlBuilder, prop string) string {
	p := b.arg(prop)
	n := fmt.Sprintf("(x.properties ->> %s)::numeric", p)
	// trunc() drops the scale, so 5.0 reads back as "5" and parses as the
	// generic fold's 5.
	return fmt.Sprintf("(CASE WHEN jsonb_typeof(x.properties -> %s) = 'number' THEN "+
		"CASE WHEN %s = trunc(%s) AND %s BETWEEN -9223372036854775808 AND 9223372036854775807 THEN trunc(%s) END END)",
		p, n, n, n, n)
}

// decimalSQL is the property as numeric when it is a JSON number or a
// string in plain decimal notation, else NULL: the values
// [store.RollupDecimal] reads.
func decimalSQL(b *sqlBuilder, prop string) string {
	p := b.arg(prop)
	return fmt.Sprintf("(CASE jsonb_typeof(x.properties -> %s) WHEN 'number' THEN (x.properties ->> %s)::numeric "+
		"WHEN 'string' THEN CASE WHEN (x.properties ->> %s) ~ '^[+-]?[0-9]+([.][0-9]+)?$' "+
		"THEN (x.properties ->> %s)::numeric END END)", p, p, p, p)
}

// nonEmptyStringSQL is the property's text when it is a non-empty JSON
// string, else NULL.
func nonEmptyStringSQL(b *sqlBuilder, prop string) string {
	p := b.arg(prop)
	return fmt.Sprintf("(CASE WHEN jsonb_typeof(x.properties -> %s) = 'string' AND (x.properties ->> %s) <> '' "+
		"THEN x.properties ->> %s END)", p, p, p)
}
