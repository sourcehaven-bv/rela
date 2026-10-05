package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// Soft delete (store.SoftDeleter) moves a family's rows from entities and
// relations into marked_entities and marked_relations (see sqlitedb's
// softDeleteDDL), and back. Every statement runs in one transaction, so a
// failure leaves the family wholly live or wholly marked.

// The live column lists, in the order the side tables repeat them.
// dropMarkedEdgesSQL removes the hidden edges that touch an id, for a hard
// delete or a rename of that id. Otherwise a restore of the marked end would
// bring back an edge to an entity that is gone, or to a new entity that took
// its id. Arguments: the id, twice.
const dropMarkedEdgesSQL = `DELETE FROM marked_relations WHERE from_id = ? OR to_id = ?`

const (
	entityRowColumns = "id, face, type, properties, content, updated_at, " +
		"last_edited_by_user, last_edited_by_tool, origin_kind, origin_source, origin_source_face, " +
		"origin_source_type, origin_definition"
	relationRowColumns = "from_id, from_face, rel_type, to_id, properties, content, updated_at, " +
		"rel_record_id, last_edited_by_user, last_edited_by_tool"
)

// SoftDelete implements [store.SoftDeleteProvider]. On a transaction view the
// methods join that transaction.
func (s *Store) SoftDelete() store.SoftDeleter { return softDeleter{s: s} }

type softDeleter struct{ s *Store }

// inTx runs fn on a transaction view, joining an open one.
func (d softDeleter) inTx(ctx context.Context, fn func(*Store) error) error {
	return d.s.Tx(ctx, func(tx store.Store) error {
		view, ok := tx.(*Store)
		if !ok { // unreachable: Tx always hands back our own view type
			return errors.New("sqlitestore: unexpected transaction view type")
		}
		return fn(view)
	})
}

func (d softDeleter) MarkDeleted(ctx context.Context, id, by string) (*store.DeleteResult, error) {
	var result *store.DeleteResult
	err := d.inTx(ctx, func(v *Store) error {
		family, err := v.stateFamily(ctx, id)
		if err != nil {
			return err
		}
		if len(family) == 0 {
			return fmt.Errorf("sqlitestore: soft delete %s: %w", id, store.ErrNotFound)
		}
		incident, err := v.incidentRelations(ctx, id)
		if err != nil {
			return err
		}
		stmts := []struct {
			q    string
			args []any
		}{
			{`INSERT INTO marked_entities (` + entityRowColumns + `, deleted_at, deleted_by)
			  SELECT ` + entityRowColumns + `, ?, ? FROM entities WHERE id = ?`,
				[]any{sqlitedb.FormatTime(time.Now()), by, id}},
			{`INSERT INTO marked_relations (owner_id, ` + relationRowColumns + `)
			  SELECT ?, ` + relationRowColumns + ` FROM relations WHERE from_id = ? OR to_id = ?
			  ON CONFLICT DO NOTHING`,
				[]any{id, id, id}},
			{`DELETE FROM relations WHERE from_id = ? OR to_id = ?`, []any{id, id}},
			{`DELETE FROM entities WHERE id = ?`, []any{id}},
		}
		for _, st := range stmts {
			if _, err := v.write(ctx, st.q, st.args...); err != nil {
				return fmt.Errorf("sqlitestore: soft delete %s: %w", id, err)
			}
		}
		emitRelations(v, store.EventRelationDeleted, incident)
		for _, e := range family {
			v.notifyFaceDelete(id, e.Face)
		}
		v.notifyLastFaceDelete(id)
		emitEntities(v, store.EventEntityDeleted, family)
		result = &store.DeleteResult{DeletedEntities: family, DeletedRelations: incident}
		return nil
	})
	return result, err
}

func (d softDeleter) Unmark(ctx context.Context, id string) (*store.DeleteResult, error) {
	var result *store.DeleteResult
	err := d.inTx(ctx, func(v *Store) error {
		family, err := markedFamily(ctx, v, id)
		if err != nil {
			return err
		}
		if len(family) == 0 {
			return fmt.Errorf("sqlitestore: restore %s: %w", id, store.ErrNotFound)
		}
		// An edge whose other end is still marked stays hidden and moves to
		// that entity, so it comes back when that one does.
		const otherEnd = `CASE WHEN from_id = ? THEN to_id ELSE from_id END`
		if _, err = v.write(ctx, `UPDATE marked_relations SET owner_id = `+otherEnd+`
			WHERE owner_id = ? AND `+otherEnd+` IN (SELECT id FROM marked_entities WHERE id <> ?)`,
			id, id, id, id); err != nil {
			return fmt.Errorf("sqlitestore: restore %s: %w", id, err)
		}
		back, err := v.scanRelationKeys(ctx, `SELECT from_id, from_face, rel_type, to_id
			FROM marked_relations WHERE owner_id = ? ORDER BY from_id, from_face, rel_type, to_id`, id)
		if err != nil {
			return err
		}
		stmts := []struct {
			q    string
			args []any
		}{
			{`INSERT INTO entities (` + entityRowColumns + `)
			  SELECT ` + entityRowColumns + ` FROM marked_entities WHERE id = ?`, []any{id}},
			{`DELETE FROM marked_entities WHERE id = ?`, []any{id}},
			// A live edge created on the same key meanwhile wins.
			{`INSERT INTO relations (` + relationRowColumns + `)
			  SELECT ` + relationRowColumns + ` FROM marked_relations WHERE owner_id = ?
			  ON CONFLICT DO NOTHING`, []any{id}},
			{`DELETE FROM marked_relations WHERE owner_id = ?`, []any{id}},
		}
		for _, st := range stmts {
			if _, err := v.write(ctx, st.q, st.args...); err != nil {
				return fmt.Errorf("sqlitestore: restore %s: %w", id, err)
			}
		}
		for _, e := range family {
			v.notifyPut(e)
		}
		emitEntities(v, store.EventEntityCreated, family)
		emitRelations(v, store.EventRelationCreated, back)
		result = &store.DeleteResult{DeletedEntities: family, DeletedRelations: back}
		return nil
	})
	return result, err
}

func (d softDeleter) ListMarked(ctx context.Context) ([]store.MarkedEntity, error) {
	rows, err := d.s.q().QueryContext(ctx, `SELECT deleted_at, deleted_by, `+entityColumns+`
		FROM marked_entities ORDER BY deleted_at, id, face`)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: list marked: %w", err)
	}
	defer rows.Close()

	var out []store.MarkedEntity
	for rows.Next() {
		var at, by string
		e, err := scanEntity(prefixScanner{rows, []any{&at, &by}})
		if err != nil {
			return nil, err
		}
		if n := len(out); n > 0 && out[n-1].ID == e.ID {
			out[n-1].Entities = append(out[n-1].Entities, e)
			continue
		}
		t, err := parseTime(at)
		if err != nil {
			return nil, fmt.Errorf("sqlitestore: parse deleted_at: %w", err)
		}
		out = append(out, store.MarkedEntity{ID: e.ID, DeletedAt: t, DeletedBy: by, Entities: []*entity.Entity{e}})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlitestore: list marked: %w", err)
	}
	return out, nil
}

func (d softDeleter) PurgeMarked(ctx context.Context, id string) (*store.DeleteResult, error) {
	var result *store.DeleteResult
	err := d.inTx(ctx, func(v *Store) error {
		family, err := markedFamily(ctx, v, id)
		if err != nil {
			return err
		}
		if len(family) == 0 {
			return fmt.Errorf("sqlitestore: purge %s: %w", id, store.ErrNotFound)
		}
		// Hidden edges to id held by another marked entity go too; they must
		// not come back pointing at an entity that no longer exists.
		const match = `owner_id = ? OR from_id = ? OR to_id = ?`
		// Whole rows, not keys: the caller records delete versions from them.
		rels, err := scanRelationRows(ctx, v, `SELECT `+relationColumns+`
			FROM marked_relations WHERE `+match+` ORDER BY from_id, from_face, rel_type, to_id`, id, id, id)
		if err != nil {
			return err
		}
		for _, st := range []struct {
			q    string
			args []any
		}{
			{`DELETE FROM marked_relations WHERE ` + match, []any{id, id, id}},
			{`DELETE FROM attachments WHERE entity_id = ?`, []any{id}},
			{`DELETE FROM marked_entities WHERE id = ?`, []any{id}},
		} {
			if _, err := v.write(ctx, st.q, st.args...); err != nil {
				return fmt.Errorf("sqlitestore: purge %s: %w", id, err)
			}
		}
		result = &store.DeleteResult{DeletedEntities: family, DeletedRelations: rels}
		return nil
	})
	return result, err
}

// markedFamily loads the marked faces of id, default face first.
func markedFamily(ctx context.Context, s *Store, id string) ([]*entity.Entity, error) {
	rows, err := s.q().QueryContext(ctx,
		`SELECT `+entityColumns+` FROM marked_entities WHERE id = ? ORDER BY face`, id)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: marked family %s: %w", id, err)
	}
	defer rows.Close()
	var out []*entity.Entity
	for rows.Next() {
		e, err := scanEntity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlitestore: marked family %s: %w", id, err)
	}
	return out, nil
}

// markedIDTaken reports whether a marked entity holds id, case-folded.
// except skips one id, for a rename that only changes casing.
func markedIDTaken(ctx context.Context, s *Store, id, except string) (bool, error) {
	var n int
	err := s.q().QueryRowContext(ctx, `SELECT count(*) FROM marked_entities
		WHERE lower(id) = lower(?) AND lower(id) <> lower(?)`, id, except).Scan(&n)
	return n > 0, err
}

// revealedRelation returns the hidden edge k when ctx reveals one of its
// endpoints (see [store.WithRevealed]).
func revealedRelation(ctx context.Context, s *Store, k entity.RelationKey) (*entity.Relation, error) {
	id, ok := store.RevealedFor(ctx, k.From, k.To)
	if !ok {
		return nil, sql.ErrNoRows
	}
	return scanRelation(s.q().QueryRowContext(ctx, `SELECT `+relationColumns+` FROM marked_relations
		WHERE owner_id = ? AND from_id = ? AND rel_type = ? AND to_id = ? AND from_face = ?`,
		id, k.From, k.Type, k.To, string(k.FromFace)))
}

// revealedRelations returns the hidden relations of the entity ctx reveals
// that satisfy q, for ListRelations.
func revealedRelations(ctx context.Context, s *Store, q store.RelationQuery) ([]*entity.Relation, error) {
	id := store.RevealedID(ctx)
	if id == "" || q.EntityID != id {
		return nil, nil
	}
	all, err := scanRelationRows(ctx, s, `SELECT `+relationColumns+` FROM marked_relations
		WHERE owner_id = ? ORDER BY from_id, from_face, rel_type, to_id`, id)
	if err != nil {
		return nil, err
	}
	match := storeutil.NewRelationMatcher(q)
	out := all[:0]
	for _, r := range all {
		if match(r) {
			out = append(out, r)
		}
	}
	return out, nil
}

// scanRelationRows runs a query selecting [relationColumns].
func scanRelationRows(ctx context.Context, s *Store, query string, args ...any) ([]*entity.Relation, error) {
	rows, err := s.q().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: scan relations: %w", err)
	}
	defer rows.Close()
	var out []*entity.Relation
	for rows.Next() {
		r, err := scanRelation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlitestore: scan relations: %w", err)
	}
	return out, nil
}

func emitEntities(s *Store, op store.EventOp, family []*entity.Entity) {
	for _, e := range family {
		s.emit(store.Event{Op: op, EntityID: e.ID, EntityType: e.Type, Face: e.Face})
	}
}

func emitRelations(s *Store, op store.EventOp, rels []*entity.Relation) {
	for _, r := range rels {
		s.emit(store.Event{Op: op, RelationType: r.Type, From: r.From, To: r.To, Face: r.FromFace})
	}
}

// prefixScanner scans leading extra columns before handing the rest to a
// row scanner that expects only its own columns.
type prefixScanner struct {
	sc    scanner
	extra []any
}

func (p prefixScanner) Scan(dest ...any) error {
	return p.sc.Scan(append(append([]any{}, p.extra...), dest...)...)
}
