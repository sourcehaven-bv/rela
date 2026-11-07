package pgstore

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/elementnaive"
)

// ElementRanges is the element range reader of a pgstore (store.ElementScope,
// TKT-IQU106C). Each call is one statement over the stored element columns:
// the scope's row gate and the per-row hidden-field test run in its WHERE
// clause, under the aggregate, so a row the reader may not see is never
// summed or counted.
//
// The gate of a type with a relation predicate walks its closure from every
// entity of the type, as the search and list gates do, so a small range of a
// large gated type costs the type. Seeding the closure from the range is an
// optimisation for when real range sizes are known.
type ElementRanges struct {
	s *Store
}

// NewElementRanges returns the element range reader over s.
// Nil: rejected, as every required collaborator is.
func NewElementRanges(s *Store) (*ElementRanges, error) {
	if s == nil {
		return nil, errors.New("pgstore: nil store")
	}
	return &ElementRanges{s: s}, nil
}

// RangeFingerprint returns the count and fingerprint sum of the elements
// scope admits in r.
func (g *ElementRanges) RangeFingerprint(
	ctx context.Context, scope store.ElementScope, r store.ElementRange,
) (store.RangeSum, error) {
	plan, err := elementnaive.Plan(scope, r)
	if err != nil || plan.Empty() {
		return store.RangeSum{}, err
	}
	sqlText, args, err := buildRangeSumSQL(plan)
	if err != nil {
		return store.RangeSum{}, err
	}
	var count, pending int64
	var limbs [4]int64
	err = g.s.db.QueryRow(ctx, sqlText, args...).Scan(
		&count, &pending, &limbs[0], &limbs[1], &limbs[2], &limbs[3],
	)
	if err != nil {
		return store.RangeSum{}, fmt.Errorf("pgstore: range fingerprint: %w", err)
	}
	sum, err := store.RangeSumFromAggregates(count, pending, limbs)
	if err != nil {
		return store.RangeSum{}, fmt.Errorf("pgstore: %w", err)
	}
	return sum, nil
}

// ListRange lists the elements scope admits in r, in address order, up to
// l.Limit.
func (g *ElementRanges) ListRange(
	ctx context.Context, scope store.ElementScope, r store.ElementRange, l store.RangeList,
) ([]store.RangeEntry, error) {
	if err := store.ValidateRangeList(l); err != nil {
		return nil, err
	}
	plan, err := elementnaive.Plan(scope, r)
	if err != nil || plan.Empty() {
		return nil, err
	}
	sqlText, args, err := buildRangeListSQL(plan, l)
	if err != nil {
		return nil, err
	}
	rows, err := g.s.db.Query(ctx, sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("pgstore: list range: %w", err)
	}
	defer rows.Close()
	var out []store.RangeEntry
	for rows.Next() {
		var (
			e              store.RangeEntry
			hash           *string
			k1, k2, k3, k4 string
		)
		if err := rows.Scan(&e.Address, &hash, &e.Pending, &k1, &k2, &k3, &k4); err != nil {
			return nil, fmt.Errorf("pgstore: list range scan: %w", err)
		}
		e.SetRowKey(plan.Range.Relations, k1, k2, k3, k4)
		if hash != nil {
			e.Hash = *hash
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgstore: list range: %w", err)
	}
	return out, nil
}

func buildRangeSumSQL(plan store.ElementRangePlan) (sqlText string, args []any, err error) {
	b := &sqlBuilder{}
	with, rows := rangeRowsSQL(b, plan)
	sqlText = withClause(with) + "SELECT count(*), count(*) FILTER (WHERE NOT x.ok)" +
		", COALESCE(sum(x.f0) FILTER (WHERE x.ok), 0)::bigint" +
		", COALESCE(sum(x.f1) FILTER (WHERE x.ok), 0)::bigint" +
		", COALESCE(sum(x.f2) FILTER (WHERE x.ok), 0)::bigint" +
		", COALESCE(sum(x.f3) FILTER (WHERE x.ok), 0)::bigint FROM (" + rows + ") x"
	if b.err != nil {
		return "", nil, b.err
	}
	return sqlText, b.args, nil
}

func buildRangeListSQL(plan store.ElementRangePlan, l store.RangeList) (sqlText string, args []any, err error) {
	b := &sqlBuilder{}
	with, rows := rangeRowsSQL(b, plan)
	where := ""
	if l.PendingOnly {
		where = " WHERE NOT x.ok"
	}
	sqlText = withClause(with) + "SELECT x.addr, CASE WHEN x.ok THEN x.hash END, NOT x.ok, x.k1, x.k2, x.k3, x.k4" +
		" FROM (" + rows + ") x" + where + ` ORDER BY x.addr COLLATE "C" LIMIT ` + b.arg(l.Limit)
	if b.err != nil {
		return "", nil, b.err
	}
	return sqlText, b.args, nil
}

// rangeRowsSQL renders the rows plan admits as a derived table with the
// columns addr, ok (the row may fold its stored element), hash, f0..f3 and
// the key columns k1..k4. ok is never NULL: a NULL would drop the row from
// both the folded and the pending side of a sum.
func rangeRowsSQL(b *sqlBuilder, plan store.ElementRangePlan) (with []string, rows string) {
	if plan.Range.Relations {
		return relationRangeRowsSQL(b, plan)
	}
	arms := make([]string, 0, len(plan.Types))
	for i, tp := range plan.Types {
		typeArg := b.arg(tp.Type)
		gw, gate := typeGateSQL(b, "g"+strconv.Itoa(i)+"_", tp, typeArg)
		hw, ok := foldSQL(b, "h"+strconv.Itoa(i)+"_", tp, typeArg)
		with = append(append(with, gw...), hw...)
		where := append([]string{
			entityAddressSQL + " >= " + b.arg(tp.Lo),
			entityAddressSQL + " < " + b.arg(tp.Hi),
			"e.type = " + typeArg,
		}, gate...)
		arms = append(arms, "SELECT "+entityAddressSQL+" AS addr, "+ok+" AS ok, e.element_hash AS hash"+
			", e.element_fp0 AS f0, e.element_fp1 AS f1, e.element_fp2 AS f2, e.element_fp3 AS f3"+
			", e.type AS k1, e.id AS k2, e.face AS k3, ''::text AS k4"+
			" FROM entities e WHERE "+strings.Join(where, " AND "))
	}
	return with, strings.Join(arms, " UNION ALL ")
}

// typeGateSQL renders tp's row gate as conjuncts over e.
func typeGateSQL(b *sqlBuilder, prefix string, tp store.TypePlan, typeArg string) (with, conds []string) {
	if tp.Gate != nil {
		with, conds = fieldGrantCond(b, prefix, *tp.Gate, typeArg)
	}
	if tp.Faces != nil {
		faces := make([]string, len(tp.Faces))
		for i, f := range tp.Faces {
			faces[i] = f.String()
		}
		conds = append(conds, "e.face = ANY("+b.arg(faces)+")")
	}
	return with, conds
}

// foldSQL renders whether a row of tp may fold its stored element.
func foldSQL(b *sqlBuilder, prefix string, tp store.TypePlan, typeArg string) (with []string, ok string) {
	switch tp.Fold {
	case store.FoldStored:
		return nil, "(e.element_hash IS NOT NULL)"
	case store.FoldPerRow:
	default:
		return nil, "FALSE"
	}
	clean := []string{"e.element_hash IS NOT NULL"}
	for i, h := range tp.Hidden {
		w, visible := rowConditionsSQL(b, prefix+strconv.Itoa(i)+"_", []store.RowCondition{h.Visible}, typeArg)
		with = append(with, w...)
		clean = append(clean, "(NOT (e.properties ? "+b.arg(h.Property)+") OR "+visible+")")
	}
	return with, "COALESCE(" + strings.Join(clean, " AND ") + ", FALSE)"
}

// relationRangeRowsSQL is [rangeRowsSQL] for a relation range: the relations
// of the planned types in range whose head has an admitted row and whose
// tail row is admitted (its FromFace row, or any row for an identity tail).
func relationRangeRowsSQL(b *sqlBuilder, plan store.ElementRangePlan) (with []string, rows string) {
	gates := make([]string, 0, 1+len(plan.Types))
	gates = append(gates, "FALSE")
	for i, tp := range plan.Types {
		typeArg := b.arg(tp.Type)
		gw, gate := typeGateSQL(b, "g"+strconv.Itoa(i)+"_", tp, typeArg)
		with = append(with, gw...)
		gates = append(gates, "("+strings.Join(append([]string{"e.type = " + typeArg}, gate...), " AND ")+")")
	}
	admitted := "(" + strings.Join(gates, " OR ") + ")"
	where := []string{relationAddressSQL + " >= " + b.arg(plan.Range.Lo)}
	if !plan.Range.Unbounded {
		where = append(where, relationAddressSQL+" < "+b.arg(plan.Range.Hi))
	}
	where = append(where,
		"rel.rel_type = ANY("+b.arg(plan.RelationTypes())+")",
		"EXISTS (SELECT 1 FROM entities e WHERE e.id = rel.to_id AND "+admitted+")",
		"EXISTS (SELECT 1 FROM entities e WHERE e.id = rel.from_id"+
			" AND (rel.from_face = '' OR e.face = rel.from_face) AND "+admitted+")")
	ok := "COALESCE(rel.element_hash IS NOT NULL AND rel.rel_type = ANY(" + b.arg(plan.FoldTypes()) + "), FALSE)"
	return with, "SELECT " + relationAddressSQL + " AS addr, " + ok + " AS ok, rel.element_hash AS hash" +
		", rel.element_fp0 AS f0, rel.element_fp1 AS f1, rel.element_fp2 AS f2, rel.element_fp3 AS f3" +
		", rel.from_id AS k1, rel.from_face AS k2, rel.rel_type AS k3, rel.to_id AS k4" +
		" FROM relations rel WHERE " + strings.Join(where, " AND ")
}
