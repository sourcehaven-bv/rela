package pgstore

import (
	"context"

	"github.com/jackc/pgx/v5/pgconn"

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
		rows, err := s.db.Query(ctx, q.sql)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			el := storetest.StoredElement{Kind: q.kind}
			if err := rows.Scan(&el.Address, &el.Hash, &el.Limbs[0], &el.Limbs[1], &el.Limbs[2], &el.Limbs[3]); err != nil {
				rows.Close()
				return nil, err
			}
			out = append(out, el)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// SortedAddressesForTest returns the entity addresses ordered by the
// database through the address expression.
func SortedAddressesForTest(ctx context.Context, s *Store) ([]string, error) {
	rows, err := s.db.Query(ctx, `SELECT `+entityAddressSQL+` AS a FROM entities ORDER BY a`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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
	return healElements(ctx, s.db, batch)
}

// HealEntitiesWithWriteForTest runs one entity heal pass and calls write after
// the pass has read its rows and before it stores their elements.
func HealEntitiesWithWriteForTest(ctx context.Context, s *Store, write func()) (read, stored int, err error) {
	return fillEntityElements(ctx, &beforeExec{DBTX: s.db, hook: write}, anyRowScope(), 0)
}

// EntityRangeSumSQLForTest is a range sum over entity addresses between $1
// and $2, the query step 3 will run.
const EntityRangeSumSQLForTest = `SELECT count(*), sum(element_fp0), sum(element_fp1), sum(element_fp2),
	sum(element_fp3) FROM entities WHERE ` + entityAddressSQL + ` >= $1 AND ` + entityAddressSQL + ` < $2`

// RelationRangeSumSQLForTest is [EntityRangeSumSQLForTest] over relation
// addresses.
const RelationRangeSumSQLForTest = `SELECT count(*), sum(element_fp0), sum(element_fp1), sum(element_fp2),
	sum(element_fp3) FROM relations WHERE ` + relationAddressSQL + ` >= $1 AND ` + relationAddressSQL + ` < $2`

// beforeExec runs hook once, before the first statement executed through it.
type beforeExec struct {
	DBTX
	hook func()
}

func (b *beforeExec) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if b.hook != nil {
		b.hook()
		b.hook = nil
	}
	return b.DBTX.Exec(ctx, sql, args...)
}
