package sqlitestore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/graphquerynaive"
)

// The SQL roll-up is the point of this file: without it [store.Rollup]
// falls back to the Go fold, which costs more queries per column.
var _ store.Rollupper = (*Store)(nil)

// Rollup implements [store.Rollupper] in one statement, as pgstore's does:
// the related rows are the graph source of q.Related (the caller's gate),
// joined to the identity-scoped edges of the sources and grouped per source.
// A query whose names the builder cannot render is answered by
// [store.RollupGeneric], the reference both backends are tested against.
func (s *Store) Rollup(ctx context.Context, q store.RollupQuery) (map[string]store.RollupResult, error) {
	if err := store.ValidateRollupQuery(q); err != nil {
		return nil, err
	}
	if err := graphquerynaive.CheckEndpointShape(q.Related); err != nil {
		return nil, err
	}
	out := map[string]store.RollupResult{}
	if len(q.Sources) == 0 {
		return out, nil
	}
	sqlText, args, ok := buildRollupSQL(q)
	if !ok {
		return store.RollupGeneric(ctx, s, q)
	}
	type row struct {
		src   string
		count int
		value sql.NullString
	}
	got, err := collectRows(ctx, s.q(), sqlText, args, func(sc scanner) (row, error) {
		var r row
		return r, sc.Scan(&r.src, &r.count, &r.value)
	})
	if err != nil {
		return nil, err
	}
	for _, r := range got {
		res := store.RollupResult{Count: r.count}
		if r.value.Valid {
			v, err := rollupValue(q, r.value.String)
			if err != nil {
				return nil, err
			}
			res.Value = v
		}
		out[r.src] = res
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
			return nil, fmt.Errorf("sqlitestore: rollup latest: %w", err)
		}
		if ok, _ := pair[1].(bool); !ok {
			return nil, nil //nolint:nilnil // a hidden pick is "no value", not a failure
		}
		return store.RollupScalar(pair[0]), nil
	case q.Func == store.RollupSum:
		return splitSum(text)
	case q.Numeric:
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("sqlitestore: rollup: %s is not a 64-bit integer: %w", text, err)
		}
		return n, nil
	default:
		return text, nil
	}
}

// sumHalfBits is the width of the low half of a split sum.
const sumHalfBits = 32

// splitSum joins the "high,low" halves rollupAggregateSQL returns for a
// sum: total = high*2^32 + low. A total outside int64 has no value, as in
// the generic fold.
func splitSum(text string) (any, error) {
	hi, lo, ok := strings.Cut(text, ",")
	h, okh := new(big.Int).SetString(hi, 10)
	l, okl := new(big.Int).SetString(lo, 10)
	if !ok || !okh || !okl {
		return nil, fmt.Errorf("sqlitestore: rollup sum: malformed %q", text)
	}
	total := h.Lsh(h, sumHalfBits).Add(h, l)
	if !total.IsInt64() {
		return nil, nil //nolint:nilnil // out of range is "no value", not a failure
	}
	return total.Int64(), nil
}

// buildRollupSQL renders q as one grouped statement, or reports !ok when a
// name cannot be rendered.
func buildRollupSQL(q store.RollupQuery) (sqlText string, args []any, ok bool) {
	b := &sqlBuilder{}
	srcCol, relCol := "from_id", "to_id"
	if q.SourceIsTo {
		srcCol, relCol = relCol, srcCol
	}
	relArg := b.arg(q.Relation)
	srcList := b.jsonList(q.Sources)
	edges := fmt.Sprintf("SELECT r0.%s FROM relations r0 WHERE r0.rel_type = %s AND r0.from_face = '' AND r0.%s IN (%s)",
		relCol, relArg, srcCol, srcList)

	typeArg := b.arg(q.Related.EntityType)
	with, source := graphSource(b, q.Related, typeArg, edges)
	seed := "SELECT e0.id FROM entities e0 WHERE e0.id IN (" + edges + ") AND +e0.type = " + typeArg

	requireWith, require := rowConditionsSQL(b, "rq", q.Require, seed)
	pickWith, pick := rowConditionsSQL(b, "pk", q.PickRequire, seed)
	with = append(append(with, requireWith...), pickWith...)

	order := "1"
	if q.Func == store.RollupLatest {
		by := nonEmptyStringSQL(b, "x", q.By)
		order = "(" + by + " IS NULL), " + by + " DESC, x.id"
	}
	rowsSQL := fmt.Sprintf(
		"SELECT e.id, e.properties, %s AS pickok FROM (SELECT e.* FROM %s) e WHERE %s", pick, source, require)
	joined := fmt.Sprintf(
		"SELECT r.%s AS src, x.id, x.properties, x.pickok, "+
			"ROW_NUMBER() OVER (PARTITION BY r.%s ORDER BY %s) AS rk "+
			"FROM relations r JOIN (%s) x ON x.id = r.%s "+
			"WHERE r.rel_type = %s AND r.from_face = '' AND r.%s IN (%s)",
		srcCol, srcCol, order, rowsSQL, relCol, relArg, srcCol, srcList)

	agg := rollupAggregateSQL(b, q)
	sqlText = withClause(with) + "SELECT x.src, count(*), " + agg + " FROM (" + joined + ") x GROUP BY x.src"
	return sqlText, b.args, !b.unsafe
}

// rowConditionsSQL renders conds as one boolean over e: every condition
// holds, a condition holding when any of its queries matches the row. A
// query reads every face of its relation endpoints, and its FaceIn restricts
// the row's own face, as a field grant's `when:` does.
func rowConditionsSQL(
	b *sqlBuilder, prefix string, conds []store.RowCondition, seed string,
) (with []string, cond string) {
	all := make([]string, 0, 1+len(conds))
	all = append(all, "1")
	for i, c := range conds {
		anyOf := []string{"0"}
		for j, when := range c.Any {
			if when == nil {
				anyOf = append(anyOf, "1")
				continue
			}
			w := *when
			w.Faces = store.AllFaces()
			cw, pre, post := buildPredicatePartsPrefixed(b, fmt.Sprintf("%s%d_%d_", prefix, i, j), w, seed)
			with = append(with, cw...)
			conj := append(append([]string{"1"}, pre...), post...)
			if len(w.FaceIn) > 0 {
				faces := make([]string, len(w.FaceIn))
				for k, f := range w.FaceIn {
					faces[k] = f.String()
				}
				conj = append(conj, "e.face IN ("+b.jsonList(faces)+")")
			}
			anyOf = append(anyOf, "("+strings.Join(conj, " AND ")+")")
		}
		all = append(all, "("+strings.Join(anyOf, " OR ")+")")
	}
	return with, "(" + strings.Join(all, " AND ") + ")"
}

// rollupAggregateSQL renders the aggregate column as text, or NULL for a
// count. The value shapes match [store.RollupNumber] and the non-empty
// strings of [store.RollupGeneric]. SQLite's sum raises on integer
// overflow, as the generic fold does.
func rollupAggregateSQL(b *sqlBuilder, q store.RollupQuery) string {
	switch q.Func {
	case store.RollupSum:
		// SQLite's integer sum raises on overflow, so the value is summed
		// in two halves that cannot overflow for any realistic row count,
		// and splitSum joins them exactly.
		v := integralSQL(b, q.Value)
		return "CASE WHEN count(" + v + ") > 0 THEN CAST(sum(" + v + " / 4294967296) AS TEXT) || ',' || " +
			"CAST(sum(" + v + " % 4294967296) AS TEXT) END"
	case store.RollupMin, store.RollupMax:
		fn := "min"
		if q.Func == store.RollupMax {
			fn = "max"
		}
		if q.Numeric {
			return "CAST(" + fn + "(" + integralSQL(b, q.Value) + ") AS TEXT)"
		}
		return fn + "(" + nonEmptyStringSQL(b, "x", q.Value) + ")"
	case store.RollupLatest:
		by := nonEmptyStringSQL(b, "x", q.By)
		path := b.jsonPath(q.Value)
		return "max(CASE WHEN x.rk = 1 AND " + by + " IS NOT NULL THEN json_array(x.properties -> " + path +
			", CASE WHEN x.pickok THEN json('true') ELSE json('false') END) END)"
	default:
		return "NULL"
	}
}

// integralSQL is the property as an integer when it is an integral JSON
// number in the int64 range, else NULL.
func integralSQL(b *sqlBuilder, prop string) string {
	path := b.jsonPath(prop)
	t := typeExpr("x", path)
	raw := rawExpr("x", path)
	return "(CASE " + t + " WHEN 'integer' THEN " + raw +
		" WHEN 'real' THEN CASE WHEN " + raw + " = CAST(" + raw + " AS INTEGER)" +
		" AND " + raw + " >= -9223372036854775808.0 AND " + raw + " < 9223372036854775807.0" +
		" THEN CAST(" + raw + " AS INTEGER) END END)"
}

// nonEmptyStringSQL is the property's text when it is a non-empty JSON
// string, else NULL.
func nonEmptyStringSQL(b *sqlBuilder, alias, prop string) string {
	path := b.jsonPath(prop)
	raw := rawExpr(alias, path)
	return "(CASE WHEN " + typeExpr(alias, path) + " = 'text' AND " + raw + " <> '' THEN " + raw + " END)"
}
