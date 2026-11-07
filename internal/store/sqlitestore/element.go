package sqlitestore

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/reconcile"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The reconciliation element of each live row (TKT-1G2XR1R): element_hash and
// the four fingerprint limbs element_fp0..3. sqlitedb's elementDDL explains
// the columns, the address expression and the clearing trigger.
//
// Single-row writes set the columns in the write statement, hashed from the
// row as it will read back ([entityElementCols]), and then refill a row the
// trigger cleared because its canonical hash did not change
// ([refillEntityElement]). Bulk writes let the trigger clear the rows they
// change and fill them afterwards ([fillEntityElements]). The version sweep
// heals rows left NULL by anything else.

// entityAddressSQL is the SQL form of [reconcile.EntityAddress] over the
// entities columns. It must match the expression of entities_element_idx
// exactly, or a range query cannot use the index.
const entityAddressSQL = `(type || '/' || id || CASE WHEN face = '' THEN '' ELSE '@' || face END)`

// relationAddressSQL is the SQL form of [entity.RelationKey.String] over the
// relations columns, matching relations_element_idx.
const relationAddressSQL = `(from_id || CASE WHEN from_face = '' THEN '' ELSE '@' || from_face END` +
	` || '--' || rel_type || '--' || to_id)`

// elementSetSQL assigns the five element columns from five arguments.
const elementSetSQL = `element_hash = ?, element_fp0 = ?, element_fp1 = ?, element_fp2 = ?, element_fp3 = ?`

// elementCols are the stored element columns of one row, in column order:
// element_hash, element_fp0, element_fp1, element_fp2, element_fp3.
type elementCols struct {
	hash  string
	limbs [4]int64
}

// args returns the columns as statement arguments.
func (c elementCols) args() []any {
	return []any{c.hash, c.limbs[0], c.limbs[1], c.limbs[2], c.limbs[3]}
}

func colsOf(el reconcile.Element) elementCols {
	c := elementCols{hash: el.Hash.String()}
	for i, l := range el.Fingerprint().Limbs() {
		c.limbs[i] = int64(l)
	}
	return c
}

// entityElementCols returns the element columns for an entity row written
// with these values. props is the marshaled properties the statement stores;
// they are decoded as a read decodes them, so the hash equals the hash of the
// row a later read returns.
func entityElementCols(id string, face entity.Face, typ, props, content string) (elementCols, error) {
	p, err := unmarshalProps(props)
	if err != nil {
		return elementCols{}, fmt.Errorf("sqlitestore: element of %s: %w", id, err)
	}
	e := entity.New(id, typ)
	e.Face = face
	e.Properties = p
	e.Content = content
	return colsOf(reconcile.EntityElement(*e)), nil
}

// relationElementCols is [entityElementCols] for a relation row.
func relationElementCols(k entity.RelationKey, props, content string) (elementCols, error) {
	p, err := unmarshalProps(props)
	if err != nil {
		return elementCols{}, fmt.Errorf("sqlitestore: element of %s: %w", k, err)
	}
	r := entity.NewRelation(k.From, k.Type, k.To)
	r.FromFace = k.FromFace
	r.Properties = p
	r.Content = content
	return colsOf(reconcile.RelationElement(*r)), nil
}

// refillEntityElement stores c on the entity row ref after an update that
// set it, when the trigger cleared it because the canonical hash did not
// change. The row must still hold exactly the type, properties and content
// the update wrote; otherwise another write landed in between and this one
// stores nothing. On a row whose element is set it is a primary-key probe that
// writes nothing.
func refillEntityElement(
	ctx context.Context, q querier, ref entity.Ref, typ, props, content string, c elementCols,
) error {
	args := append(c.args(), ref.ID, string(ref.Face), typ, props, content)
	if _, err := q.ExecContext(ctx, `UPDATE entities SET `+elementSetSQL+`
		WHERE id = ? AND face = ? AND element_hash IS NULL
		  AND type = ? AND properties = ? AND content = ?`, args...); err != nil {
		return fmt.Errorf("sqlitestore: element of %s: %w", ref.ID, err)
	}
	return nil
}

// refillRelationElement is [refillEntityElement] for a relation row.
func refillRelationElement(
	ctx context.Context, q querier, k entity.RelationKey, props, content string, c elementCols,
) error {
	args := append(c.args(), k.From, string(k.FromFace), k.Type, k.To, props, content)
	if _, err := q.ExecContext(ctx, `UPDATE relations SET `+elementSetSQL+`
		WHERE from_id = ? AND from_face = ? AND rel_type = ? AND to_id = ? AND element_hash IS NULL
		  AND properties = ? AND content = ?`, args...); err != nil {
		return fmt.Errorf("sqlitestore: element of %s: %w", k, err)
	}
	return nil
}

// warnElementLeftNull logs a failed element write that follows a data write.
// On the root store the data write has already committed, so returning the
// error would report a write that happened as failed. The row's element is
// NULL, never stale (the trigger cleared it), and the sweep's heal fills it.
// Nil err does nothing.
func warnElementLeftNull(ctx context.Context, err error) {
	if err != nil {
		slog.WarnContext(ctx, "sqlitestore: element left NULL after a write; the sweep will fill it",
			"error", err)
	}
}

// elementScope selects the rows a fill looks at. Build one with the
// functions below; the SQL comes only from package constants.
type elementScope struct {
	where string
	args  []any
}

// entityFamilyScope is every face row of id.
func entityFamilyScope(id string) elementScope {
	return elementScope{where: `id = ?`, args: []any{id}}
}

// relationsOfScope is every relation with id at either end.
func relationsOfScope(id string) elementScope {
	return elementScope{where: `(from_id = ? OR to_id = ?)`, args: []any{id, id}}
}

// relationTypeScope is every relation of type relType.
func relationTypeScope(relType string) elementScope {
	return elementScope{where: `rel_type = ?`, args: []any{relType}}
}

// anyRowScope is every row of the table; the heal bounds it with a limit.
func anyRowScope() elementScope { return elementScope{where: `1`} }

// fillEntityElements computes and stores the element of every entity row in
// scope whose element_hash is NULL, up to limit rows (0 means no limit), in
// key order. It returns how many rows it read and how many it stored.
//
// The read and the writes run in one transaction (joining the caller's), so
// no other write lands between them. The rows are read with the same columns
// and decode as every entity read, so the stored hash is the hash of what a
// reader gets.
func fillEntityElements(ctx context.Context, s *Store, scope elementScope, limit int) (read, stored int, err error) {
	return fillElements(ctx, s, entityElementTable, scope, limit)
}

// fillRelationElements is [fillEntityElements] for relations.
func fillRelationElements(ctx context.Context, s *Store, scope elementScope, limit int) (read, stored int, err error) {
	return fillElements(ctx, s, relationElementTable, scope, limit)
}

// elementTable describes one table for [fillElements]. Every string is a
// package literal.
type elementTable struct {
	name    string
	columns string // the columns scan reads, in order
	order   string
	scan    func(scanner) (reconcile.Element, error)
}

var entityElementTable = elementTable{
	name:    `entities`,
	columns: entityColumns,
	order:   `id, face`,
	scan: func(sc scanner) (reconcile.Element, error) {
		e, err := scanEntity(sc)
		if err != nil {
			return reconcile.Element{}, err
		}
		return reconcile.EntityElement(*e), nil
	},
}

var relationElementTable = elementTable{
	name:    `relations`,
	columns: relationColumns,
	order:   `from_id, from_face, rel_type, to_id`,
	scan: func(sc scanner) (reconcile.Element, error) {
		r, err := scanRelation(sc)
		if err != nil {
			return reconcile.Element{}, err
		}
		return reconcile.RelationElement(*r), nil
	},
}

func fillElements(
	ctx context.Context, s *Store, t elementTable, scope elementScope, limit int,
) (read, stored int, err error) {
	q := `SELECT ` + t.columns + `, rowid FROM ` + t.name + `
		WHERE element_hash IS NULL AND ` + scope.where + ` ORDER BY ` + t.order
	if limit > 0 {
		q += fmt.Sprintf(` LIMIT %d`, limit)
	}
	err = s.Tx(ctx, func(tx store.Store) error {
		v, ok := tx.(*Store)
		if !ok { // unreachable: Tx always hands back our own view type
			return fmt.Errorf("sqlitestore: unexpected transaction view type %T", tx)
		}
		txCtx := store.ContextInTx(ctx)
		// collectRows reads every row before the writes start, so no cursor
		// is open while the same connection writes. A row that does not decode is logged and left NULL rather than
		// failing the pass: it would sort first on every later pass and hold
		// back every row behind it. It still counts as read, so a batch of
		// only such rows is reported as a stall.
		rows, rerr := collectRows(txCtx, v.q(), q, scope.args, func(sc scanner) (rowElement, error) {
			var rowid int64
			el, scanErr := t.scan(rowPlus{sc, []any{&rowid}})
			if scanErr != nil {
				slog.WarnContext(txCtx, "sqlitestore: cannot compute a row's element; it stays NULL",
					"table", t.name, "error", scanErr)
				return rowElement{}, nil
			}
			return rowElement{rowid: rowid, cols: colsOf(el), ok: true}, nil
		})
		if rerr != nil {
			return rerr
		}
		read = len(rows)
		stored, rerr = writeRowElements(ctx, v.q(), t.name, rows)
		return rerr
	})
	return read, stored, err
}

// rowElement is a computed element waiting to be stored on a row. ok is
// false for a row whose element could not be computed.
type rowElement struct {
	rowid int64
	cols  elementCols
	ok    bool
}

// writeRowElements stores each element on its row, if the row's element is
// still NULL. table is a package literal.
func writeRowElements(ctx context.Context, q querier, table string, pending []rowElement) (int, error) {
	stored := 0
	for _, p := range pending {
		if !p.ok {
			continue
		}
		res, err := q.ExecContext(ctx, `UPDATE `+table+` SET `+elementSetSQL+`
			WHERE rowid = ? AND element_hash IS NULL`, append(p.cols.args(), p.rowid)...)
		if err != nil {
			return stored, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return stored, err
		}
		stored += int(n)
	}
	return stored, nil
}

// healElements fills up to batch unhashed entity rows and up to batch
// unhashed relation rows. It reports whether either table filled a whole
// batch with progress, so the caller runs again at once. A full batch with
// no progress is logged: those rows stay first in key order and hold back
// the rows behind them.
func healElements(ctx context.Context, s *Store, batch int) (more bool, err error) {
	for _, kind := range []struct {
		name, table string
		fill        func(context.Context, *Store, elementScope, int) (int, int, error)
	}{{"entity", "entities", fillEntityElements}, {"relation", "relations", fillRelationElements}} {
		// The fill takes the write lock; probe first so a tick with nothing to
		// heal (the usual case) does not. The probe reads the partial
		// elementless index.
		var unhashed bool
		if err := s.q().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM `+kind.table+
			` WHERE element_hash IS NULL)`).Scan(&unhashed); err != nil {
			return more, fmt.Errorf("sqlitestore: heal %s elements: %w", kind.name, err)
		}
		if !unhashed {
			continue
		}
		read, stored, fillErr := kind.fill(ctx, s, anyRowScope(), batch)
		if fillErr != nil {
			return more, fmt.Errorf("sqlitestore: heal %s elements: %w", kind.name, fillErr)
		}
		if read < batch {
			continue
		}
		if stored > 0 {
			more = true
			continue
		}
		slog.WarnContext(ctx, "sqlitestore: element heal stalled; no row in a full batch could be stored",
			"kind", kind.name, "batch", read)
	}
	return more, nil
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
