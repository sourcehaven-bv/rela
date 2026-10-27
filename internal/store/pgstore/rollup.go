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
// identity-scoped edges of the sources and grouped per source. The fold
// rules match [store.RollupGeneric], which is the reference.
func (s *Store) Rollup(ctx context.Context, q store.RollupQuery) (map[string]store.RollupResult, error) {
	if err := store.ValidateRollupQuery(q); err != nil {
		return nil, err
	}
	if err := checkGraphQueryScope(q.Related); err != nil {
		return nil, err
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
	srcCol, relCol := "from_id", "to_id"
	if q.SourceIsTo {
		srcCol, relCol = relCol, srcCol
	}
	relArg := b.arg(q.Relation)
	srcArg := b.arg(q.Sources)
	edges := fmt.Sprintf("SELECT r.%s FROM relations r WHERE r.rel_type = %s AND r.from_face = '' AND r.%s = ANY(%s)",
		relCol, relArg, srcCol, srcArg)

	typeArg := b.arg(q.Related.EntityType)
	with, source := graphSource(b, q.Related, typeArg, "e.id IN ("+edges+")")

	requireWith, require := rowConditionsSQL(b, "rq", q.Require, typeArg)
	pickWith, pick := rowConditionsSQL(b, "pk", q.PickRequire, typeArg)
	with = append(append(with, requireWith...), pickWith...)

	rowsSQL := fmt.Sprintf("SELECT e.id, e.properties, %s AS pickok FROM (SELECT e.* FROM %s) e WHERE %s",
		pick, source, require)

	agg := rollupAggregateSQL(b, q)
	sqlText = withClause(with) + fmt.Sprintf(
		"SELECT r.%s, count(*)::int, %s FROM relations r JOIN (%s) x ON x.id = r.%s "+
			"WHERE r.rel_type = %s AND r.from_face = '' AND r.%s = ANY(%s) GROUP BY r.%s",
		srcCol, agg, rowsSQL, relCol, relArg, srcCol, srcArg, srcCol)
	if b.err != nil {
		return "", nil, b.err
	}
	return sqlText, b.args, nil
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
	return fmt.Sprintf("(CASE WHEN jsonb_typeof(x.properties -> %s) = 'number' THEN "+
		"CASE WHEN %s = trunc(%s) AND %s BETWEEN -9223372036854775808 AND 9223372036854775807 THEN %s END END)",
		p, n, n, n, n)
}

// nonEmptyStringSQL is the property's text when it is a non-empty JSON
// string, else NULL.
func nonEmptyStringSQL(b *sqlBuilder, prop string) string {
	p := b.arg(prop)
	return fmt.Sprintf("(CASE WHEN jsonb_typeof(x.properties -> %s) = 'string' AND (x.properties ->> %s) <> '' "+
		"THEN x.properties ->> %s END)", p, p, p)
}
