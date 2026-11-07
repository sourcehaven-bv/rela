package sqlitestore

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/reconcile"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// StoredElementsForTest reads the stored reconciliation element of every live
// entity and relation row of s, NULL columns as zero values. The address
// comes from the index expression, so the conformance suite also pins it to
// the Go address.
func StoredElementsForTest(ctx context.Context, s *Store) ([]storetest.StoredElement, error) {
	var out []storetest.StoredElement
	for _, q := range []struct {
		kind reconcile.Kind
		sql  string
	}{
		{reconcile.KindEntity, `SELECT ` + entityAddressSQL + `, coalesce(element_hash, ''),
			coalesce(element_fp0, 0), coalesce(element_fp1, 0), coalesce(element_fp2, 0), coalesce(element_fp3, 0)
			FROM entities`},
		{reconcile.KindRelation, `SELECT ` + relationAddressSQL + `, coalesce(element_hash, ''),
			coalesce(element_fp0, 0), coalesce(element_fp1, 0), coalesce(element_fp2, 0), coalesce(element_fp3, 0)
			FROM relations`},
	} {
		els, err := collectRows(ctx, s.q(), q.sql, nil, func(sc scanner) (storetest.StoredElement, error) {
			el := storetest.StoredElement{Kind: q.kind}
			err := sc.Scan(&el.Address, &el.Hash, &el.Limbs[0], &el.Limbs[1], &el.Limbs[2], &el.Limbs[3])
			return el, err
		})
		if err != nil {
			return nil, err
		}
		out = append(out, els...)
	}
	return out, nil
}

// SortedAddressesForTest returns the entity addresses ordered by the
// database through the address expression.
func SortedAddressesForTest(ctx context.Context, s *Store) ([]string, error) {
	rows, err := s.q().QueryContext(ctx, `SELECT `+entityAddressSQL+` AS a FROM entities ORDER BY a`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// HealElementsForTest runs one element heal pass of up to batch rows per
// table.
func HealElementsForTest(ctx context.Context, s *Store, batch int) (bool, error) {
	return healElements(ctx, s, batch)
}

// ExplainEntityRangeForTest returns the query plan of a range sum over entity
// addresses, the query step 3 will run.
func ExplainEntityRangeForTest(ctx context.Context, s *Store) (string, error) {
	return s.explain(ctx, `SELECT count(*), sum(element_fp0), sum(element_fp1), sum(element_fp2), sum(element_fp3)
		FROM entities WHERE `+entityAddressSQL+` >= ? AND `+entityAddressSQL+` < ?`, []any{"a", "z"})
}

// ExplainRelationRangeForTest is [ExplainEntityRangeForTest] over relation
// addresses.
func ExplainRelationRangeForTest(ctx context.Context, s *Store) (string, error) {
	return s.explain(ctx, `SELECT count(*), sum(element_fp0), sum(element_fp1), sum(element_fp2), sum(element_fp3)
		FROM relations WHERE `+relationAddressSQL+` >= ? AND `+relationAddressSQL+` < ?`, []any{"A", "Z"})
}
