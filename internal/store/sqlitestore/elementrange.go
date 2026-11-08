package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/elementnaive"
)

// ElementRanges is the element range reader of a sqlitestore, as pgstore's
// is: one statement per call over the stored element columns, with the gate
// and the hidden-field test inside it. A request whose names the builder
// cannot render is answered by elementnaive over the store, the reference
// both backends are tested against.
type ElementRanges struct {
	s *Store
}

// NewElementRanges returns the element range reader over s.
// Nil: rejected, as every required collaborator is.
func NewElementRanges(s *Store) (*ElementRanges, error) {
	if s == nil {
		return nil, errors.New("sqlitestore: nil store")
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
	sqlText, args, ok := buildRangeSumSQL(plan)
	if !ok {
		return elementnaive.RangeFingerprint(ctx, g.s, scope, r)
	}
	var count, pending int64
	var limbs [4]int64
	err = g.s.q().QueryRowContext(ctx, sqlText, args...).Scan(
		&count, &pending, &limbs[0], &limbs[1], &limbs[2], &limbs[3],
	)
	if err != nil {
		return store.RangeSum{}, fmt.Errorf("sqlitestore: range fingerprint: %w", err)
	}
	sum, err := store.RangeSumFromAggregates(count, pending, limbs)
	if err != nil {
		return store.RangeSum{}, fmt.Errorf("sqlitestore: %w", err)
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
	sqlText, args, ok := buildRangeListSQL(plan, l)
	if !ok {
		return elementnaive.ListRange(ctx, g.s, scope, r, l)
	}
	return collectRows(ctx, g.s.q(), sqlText, args, func(sc scanner) (store.RangeEntry, error) {
		var (
			e              store.RangeEntry
			hash           sql.NullString
			k1, k2, k3, k4 string
		)
		if err := sc.Scan(&e.Address, &hash, &e.Pending, &k1, &k2, &k3, &k4); err != nil {
			return e, err
		}
		e.SetRowKey(plan.Range.Relations, k1, k2, k3, k4)
		e.Hash = hash.String
		return e, nil
	})
}

func buildRangeSumSQL(plan store.ElementRangePlan) (sqlText string, args []any, ok bool) {
	b := &sqlBuilder{}
	with, rows := rangeRowsSQL(b, plan)
	sum := func(col string) string { return ", COALESCE(SUM(CASE WHEN x.ok THEN x." + col + " END), 0)" }
	sqlText = withClause(with) + "SELECT count(*), COALESCE(SUM(CASE WHEN x.ok THEN 0 ELSE 1 END), 0)" +
		sum("f0") + sum("f1") + sum("f2") + sum("f3") + " FROM (" + rows + ") x"
	return sqlText, b.args, !b.unsafe
}

func buildRangeListSQL(plan store.ElementRangePlan, l store.RangeList) (sqlText string, args []any, ok bool) {
	b := &sqlBuilder{}
	with, rows := rangeRowsSQL(b, plan)
	where := ""
	if l.PendingOnly {
		where = " WHERE NOT x.ok"
	}
	sqlText = withClause(with) + "SELECT x.addr, CASE WHEN x.ok THEN x.hash END, NOT x.ok, x.k1, x.k2, x.k3, x.k4" +
		" FROM (" + rows + ") x" + where + " ORDER BY x.addr LIMIT " + b.arg(l.Limit)
	return sqlText, b.args, !b.unsafe
}

// rangeRowsSQL renders the rows plan admits as a derived table, as
// pgstore's does. ok is 1 or 0, never NULL.
func rangeRowsSQL(b *sqlBuilder, plan store.ElementRangePlan) (with []string, rows string) {
	if plan.Range.Relations {
		return relationRangeRowsSQL(b, plan)
	}
	arms := make([]string, 0, len(plan.Types))
	for i, tp := range plan.Types {
		typeArg := b.arg(tp.Type)
		seed := "SELECT e0.id FROM entities e0 WHERE e0.type = " + typeArg
		gw, gate := typeGateSQL(b, "g"+strconv.Itoa(i)+"_", tp, seed)
		hw, ok := foldSQL(b, "h"+strconv.Itoa(i)+"_", tp, seed)
		with = append(append(with, gw...), hw...)
		// Unary + keeps e.type out of index selection: without statistics
		// the planner rates type=? above the address range and walks the
		// whole type. The window already holds only this type's addresses.
		where := append([]string{
			entityAddressSQL + " >= " + b.arg(tp.Lo),
			entityAddressSQL + " < " + b.arg(tp.Hi),
			"+e.type = " + typeArg,
		}, gate...)
		arms = append(arms, "SELECT "+entityAddressSQL+" AS addr, "+ok+" AS ok, e.element_hash AS hash"+
			", e.element_fp0 AS f0, e.element_fp1 AS f1, e.element_fp2 AS f2, e.element_fp3 AS f3"+
			", e.type AS k1, e.id AS k2, e.face AS k3, '' AS k4"+
			" FROM entities e WHERE "+strings.Join(where, " AND "))
	}
	return with, strings.Join(arms, " UNION ALL ")
}

// typeGateSQL renders tp's row gate as conjuncts over e. The gate query is
// a row condition: evaluated against the row at every face of its
// endpoints, with its FaceIn on the row's own face.
func typeGateSQL(b *sqlBuilder, prefix string, tp store.TypePlan, seed string) (with, conds []string) {
	if tp.Gate != nil {
		w, gate := rowConditionsSQL(b, prefix, []store.RowCondition{{Any: []*store.GraphQuery{tp.Gate}}}, seed)
		with, conds = w, []string{gate}
	}
	if tp.Faces != nil {
		faces := make([]string, len(tp.Faces))
		for i, f := range tp.Faces {
			faces[i] = f.String()
		}
		conds = append(conds, "e.face IN ("+b.jsonList(faces)+")")
	}
	return with, conds
}

// foldSQL renders whether a row of tp may fold its stored element.
func foldSQL(b *sqlBuilder, prefix string, tp store.TypePlan, seed string) (with []string, ok string) {
	switch tp.Fold {
	case store.FoldStored:
		return nil, "(e.element_hash IS NOT NULL)"
	case store.FoldPerRow:
	default:
		return nil, "0"
	}
	clean := []string{"e.element_hash IS NOT NULL"}
	for i, h := range tp.Hidden {
		w, visible := rowConditionsSQL(b, prefix+strconv.Itoa(i)+"_", []store.RowCondition{h.Visible}, seed)
		with = append(with, w...)
		clean = append(clean, "("+typeExpr("e", b.jsonPath(h.Property))+" IS NULL OR "+visible+")")
	}
	return with, "COALESCE(" + strings.Join(clean, " AND ") + ", 0)"
}

// relationRangeRowsSQL is [rangeRowsSQL] for a relation range, as pgstore's
// is.
func relationRangeRowsSQL(b *sqlBuilder, plan store.ElementRangePlan) (with []string, rows string) {
	gates := make([]string, 0, 1+len(plan.Types))
	gates = append(gates, "0")
	for i, tp := range plan.Types {
		typeArg := b.arg(tp.Type)
		seed := "SELECT e0.id FROM entities e0 WHERE e0.type = " + typeArg
		gw, gate := typeGateSQL(b, "g"+strconv.Itoa(i)+"_", tp, seed)
		with = append(with, gw...)
		gates = append(gates, "("+strings.Join(append([]string{"e.type = " + typeArg}, gate...), " AND ")+")")
	}
	admitted := "(" + strings.Join(gates, " OR ") + ")"
	where := []string{relationAddressSQL + " >= " + b.arg(plan.Range.Lo)}
	if !plan.Range.Unbounded {
		where = append(where, relationAddressSQL+" < "+b.arg(plan.Range.Hi))
	}
	where = append(where,
		"rel.rel_type IN ("+b.jsonList(plan.RelationTypes())+")",
		"EXISTS (SELECT 1 FROM entities e WHERE e.id = rel.to_id AND "+admitted+")",
		"EXISTS (SELECT 1 FROM entities e WHERE e.id = rel.from_id"+
			" AND (rel.from_face = '' OR e.face = rel.from_face) AND "+admitted+")")
	ok := "COALESCE(rel.element_hash IS NOT NULL AND rel.rel_type IN (" + b.jsonList(plan.FoldTypes()) + "), 0)"
	return with, "SELECT " + relationAddressSQL + " AS addr, " + ok + " AS ok, rel.element_hash AS hash" +
		", rel.element_fp0 AS f0, rel.element_fp1 AS f1, rel.element_fp2 AS f2, rel.element_fp3 AS f3" +
		", rel.from_id AS k1, rel.from_face AS k2, rel.rel_type AS k3, rel.to_id AS k4" +
		" FROM relations rel WHERE " + strings.Join(where, " AND ")
}
