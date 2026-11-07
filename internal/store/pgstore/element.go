package pgstore

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/reconcile"
)

// The reconciliation element of each live row (TKT-1G2XR1R, migration 0042):
// element_hash and the four fingerprint limbs element_fp0..3. Migration 0042
// explains the columns, the address expression and the clearing trigger.
//
// Single-row writes set the columns in the write statement, hashed from the
// row as it will read back ([reconcile.EntityRowColumns]). Bulk writes let the trigger
// clear the rows they change and then fill them in the same transaction
// ([fillEntityElements], [fillRelationElements]). The version sweep heals rows
// left NULL by anything else.

// entityAddressSQL is the SQL form of [reconcile.EntityAddress] over the
// entities columns. It must match the expression of entities_element_idx
// exactly, or a range query cannot use the index.
const entityAddressSQL = `(type || '/' || id || CASE WHEN face = '' THEN '' ELSE '@' || face END) COLLATE "C"`

// relationAddressSQL is the SQL form of [entity.RelationKey.String] over the
// relations columns, matching relations_element_idx.
const relationAddressSQL = `(from_id || CASE WHEN from_face = '' THEN '' ELSE '@' || from_face END` +
	` || '--' || rel_type || '--' || to_id) COLLATE "C"`

// elementKeysSQL removes the element columns from a row saved as jsonb (the
// soft-delete tables), so a restored row gets them recomputed.
const elementKeysSQL = `'{element_hash,element_fp0,element_fp1,element_fp2,element_fp3}'::text[]`

// healElements fills up to batch unhashed entity rows and up to batch
// unhashed relation rows. It reports whether either table filled a whole
// batch with progress, so the caller runs again at once. A full batch with
// no progress is logged: those rows stay first in key order and hold back
// the rows behind them.
func healElements(ctx context.Context, db DBTX, batch int) (more bool, err error) {
	for _, kind := range []struct {
		name string
		fill func(context.Context, DBTX, elementScope, int) (int, int, error)
	}{{"entity", fillEntityElements}, {"relation", fillRelationElements}} {
		read, stored, fillErr := kind.fill(ctx, db, anyRowScope(), batch)
		if fillErr != nil {
			return more, fmt.Errorf("pgstore: heal %s elements: %w", kind.name, fillErr)
		}
		if read < batch {
			continue
		}
		if stored > 0 {
			more = true
			continue
		}
		slog.WarnContext(ctx, "pgstore: element heal stalled; no row in a full batch could be stored",
			"kind", kind.name, "batch", read)
	}
	return more, nil
}

// elementScope selects the rows a fill looks at. Build one with the
// functions below; the SQL comes only from package constants.
type elementScope struct {
	where string
	args  []any
}

// entityRowScope is the one entity row ref.
func entityRowScope(ref entity.Ref) elementScope {
	return elementScope{where: `id = $1 AND face = $2`, args: []any{ref.ID, string(ref.Face)}}
}

// entityFamilyScope is every face row of id.
func entityFamilyScope(id string) elementScope {
	return elementScope{where: `id = $1`, args: []any{id}}
}

// relationRowScope is the one relation k.
func relationRowScope(k entity.RelationKey) elementScope {
	return elementScope{
		where: `from_id = $1 AND from_face = $2 AND rel_type = $3 AND to_id = $4`,
		args:  []any{k.From, string(k.FromFace), k.Type, k.To},
	}
}

// relationsOfScope is every relation with id at either end.
func relationsOfScope(id string) elementScope {
	return elementScope{where: `(from_id = $1 OR to_id = $1)`, args: []any{id}}
}

// relationTypeScope is every relation of type relType.
func relationTypeScope(relType string) elementScope {
	return elementScope{where: `rel_type = $1`, args: []any{relType}}
}

// anyRowScope is every row of the table; the heal bounds it with a limit.
func anyRowScope() elementScope { return elementScope{where: `TRUE`} }

// fillEntityElements computes and stores the element of every row in scope
// whose element_hash is NULL, up to limit rows (0 means no limit), in key
// order. It returns how many rows it read and how many it stored.
//
// The rows are read with the same columns and decode as every entity read,
// so the stored hash is the hash of what a reader gets. The write is guarded
// by xmin and by element_hash still being NULL: a row that changed after the
// read keeps NULL, and the next fill or heal takes it. Inside a write
// transaction the rows in scope are already locked by that transaction, so
// the guard only matters for the heal.
func fillEntityElements(ctx context.Context, db DBTX, scope elementScope, limit int) (read, stored int, err error) {
	return fillElements(ctx, db, entityElementTable, scope, limit)
}

// fillRelationElements is [fillEntityElements] for relations.
func fillRelationElements(ctx context.Context, db DBTX, scope elementScope, limit int) (read, stored int, err error) {
	return fillElements(ctx, db, relationElementTable, scope, limit)
}

// elementTable describes one table for [fillElements]. Every string is a
// package literal.
type elementTable struct {
	name    string
	columns string // the columns scan reads, in order
	keyCols []string
	scan    func(scanner) (key []string, el reconcile.Element, err error)
}

var entityElementTable = elementTable{
	name:    `entities`,
	columns: `id, type, face, properties, content, updated_at`,
	keyCols: []string{`id`, `face`},
	scan: func(sc scanner) ([]string, reconcile.Element, error) {
		e, err := scanEntity(sc)
		if err != nil {
			return nil, reconcile.Element{}, err
		}
		return []string{e.ID, string(e.Face)}, reconcile.EntityElement(*e), nil
	},
}

var relationElementTable = elementTable{
	name:    `relations`,
	columns: `from_id, from_face, rel_type, to_id, properties, content, updated_at`,
	keyCols: []string{`from_id`, `from_face`, `rel_type`, `to_id`},
	scan: func(sc scanner) ([]string, reconcile.Element, error) {
		r, err := scanRelation(sc)
		if err != nil {
			return nil, reconcile.Element{}, err
		}
		return []string{r.From, string(r.FromFace), r.Type, r.To}, reconcile.RelationElement(*r), nil
	},
}

func fillElements(
	ctx context.Context, db DBTX, t elementTable, scope elementScope, limit int,
) (read, stored int, err error) {
	q := `SELECT ` + t.columns + `, xmin::text FROM ` + t.name + `
		WHERE element_hash IS NULL AND ` + scope.where + `
		ORDER BY ` + strings.Join(t.keyCols, `, `)
	if limit > 0 {
		q += fmt.Sprintf(` LIMIT %d`, limit)
	}
	batch, read, err := readElementBatch(ctx, db, t, q, scope.args)
	if err != nil {
		return 0, 0, err
	}
	stored, err = batch.write(ctx, db, t)
	return read, stored, err
}

// readElementBatch runs q and computes the element of every row it returns.
// It returns the batch and how many rows q returned. A row that does not
// decode is logged and left NULL rather than failing the pass: it would
// otherwise sort first on every later pass and hold back every row behind
// it. It still counts as read, so a batch of only such rows is reported as a
// stall.
func readElementBatch(
	ctx context.Context, db DBTX, t elementTable, q string, args []any,
) (*elementBatch, int, error) {
	rows, err := db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	batch := &elementBatch{}
	read := 0
	for rows.Next() {
		read++
		var xmin string
		key, el, scanErr := t.scan(rowPlus{rows, []any{&xmin}})
		if scanErr != nil {
			slog.WarnContext(ctx, "pgstore: cannot compute a row's element; it stays NULL",
				"table", t.name, "error", scanErr)
			continue
		}
		batch.add(key, xmin, el.Columns())
	}
	return batch, read, rows.Err()
}

// rowPlus scans a row that has more columns after the ones a scan function
// such as [scanEntity] reads; they land in extra.
type rowPlus struct {
	row   scanner
	extra []any
}

func (r rowPlus) Scan(dest ...any) error {
	return r.row.Scan(append(dest, r.extra...)...)
}

// elementBatch collects computed elements for one UPDATE ... FROM unnest.
type elementBatch struct {
	keys  [][]string // per key column, the values
	xmins []string
	hash  []string
	limbs [4][]int64
}

func (b *elementBatch) len() int { return len(b.xmins) }

func (b *elementBatch) add(key []string, xmin string, c reconcile.Columns) {
	if b.keys == nil {
		b.keys = make([][]string, len(key))
	}
	for i, v := range key {
		b.keys[i] = append(b.keys[i], v)
	}
	b.xmins = append(b.xmins, xmin)
	b.hash = append(b.hash, c.Hash)
	for i := range b.limbs {
		b.limbs[i] = append(b.limbs[i], c.Limbs[i])
	}
}

// write stores the batch in one statement and returns how many rows it
// changed.
//
// It locks the rows in key order and skips a row another transaction holds.
// The heal runs beside writers that lock many rows (a cascading delete, a
// rename, a restore), and taking a batch of row locks in join order could
// deadlock with them. A skipped row keeps NULL for the next pass. Inside a
// write transaction the rows in scope are that transaction's own, so nothing
// is skipped there.
func (b *elementBatch) write(ctx context.Context, db DBTX, t elementTable) (int, error) {
	if b.len() == 0 {
		return 0, nil
	}
	var args []any
	var unnest, names, match, order strings.Builder
	for i, col := range t.keyCols {
		args = append(args, b.keys[i])
		fmt.Fprintf(&unnest, "$%d::text[], ", i+1)
		fmt.Fprintf(&names, "k%d, ", i)
		fmt.Fprintf(&match, "t.%s = v.k%d AND ", col, i)
		if i > 0 {
			order.WriteString(", ")
		}
		fmt.Fprintf(&order, "t.%s", col)
	}
	args = append(args, b.xmins, b.hash)
	fmt.Fprintf(&unnest, "$%d::text[], $%d::text[]", len(args)-1, len(args))
	for i := range b.limbs {
		args = append(args, b.limbs[i])
		fmt.Fprintf(&unnest, ", $%d::bigint[]", len(args))
	}
	q := `WITH v AS (
			SELECT * FROM unnest(` + unnest.String() + `) AS v(` + names.String() + `x, h, f0, f1, f2, f3)
		), l AS (
			SELECT t.ctid AS c, v.h, v.f0, v.f1, v.f2, v.f3
			FROM ` + t.name + ` t JOIN v ON ` + match.String() + `t.xmin::text = v.x
			WHERE t.element_hash IS NULL
			ORDER BY ` + order.String() + `
			FOR UPDATE OF t SKIP LOCKED
		)
		UPDATE ` + t.name + ` t
		SET element_hash = l.h, element_fp0 = l.f0, element_fp1 = l.f1, element_fp2 = l.f2, element_fp3 = l.f3
		FROM l WHERE t.ctid = l.c`
	tag, err := db.Exec(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
