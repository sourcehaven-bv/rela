package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// Soft delete (store.SoftDeleter) moves a family's rows from entities and
// relations into marked_entities and marked_relations (migration 0018), and
// back. Each method runs in one transaction under the family lock, the same
// discipline DeleteEntity uses; on a Tx view that transaction is a savepoint of
// the open one.
//
// Marking writes tombstones and NOTIFYs exactly like a delete, and restoring
// gives the rows a fresh seq, so the change feed and the manifest see a delete
// followed by a create and need no knowledge of marks.

// dropMarkedEdgesSQL removes the hidden edges that touch $1, for a hard delete
// or a rename of $1. Otherwise a restore of the marked end would bring back an
// edge to an entity that is gone, or to a new entity that took its id.
const dropMarkedEdgesSQL = `DELETE FROM marked_relations WHERE from_id = $1 OR to_id = $1`

// The marked rows read back as live-shaped rows.
const (
	markedEntitySelect = `SELECT e.id, e.type, e.face, e.properties, e.content, e.updated_at
		FROM marked_entities m, jsonb_populate_record(NULL::entities, m.row) e`
	markedRelationSelect = `SELECT r.from_id, r.from_face, r.rel_type, r.to_id, r.properties, r.content, r.updated_at
		FROM marked_relations m, jsonb_populate_record(NULL::relations, m.row) r`
	relationOrder = ` ORDER BY from_id, from_face, rel_type, to_id`
)

// SoftDelete implements [store.SoftDeleteProvider].
func (s *Store) SoftDelete() store.SoftDeleter { return softDeleter{s: s} }

type softDeleter struct{ s *Store }

func (d softDeleter) MarkDeleted(ctx context.Context, id, by string) (*store.DeleteResult, error) {
	s := d.s
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	if lockErr := lockFamily(ctx, tx, id); lockErr != nil {
		return nil, lockErr
	}
	family, err := scanEntities(ctx, tx,
		`SELECT id, type, face, properties, content, updated_at
		 FROM entities WHERE id = $1 ORDER BY face ASC`, id)
	if err != nil {
		return nil, err
	}
	if len(family) == 0 {
		return nil, store.ErrNotFound
	}
	related, err := scanRelations(ctx, tx,
		`SELECT from_id, from_face, rel_type, to_id, properties, content, updated_at
		 FROM relations WHERE from_id = $1 OR to_id = $1`+relationOrder, id)
	if err != nil {
		return nil, err
	}

	for _, st := range []struct {
		q    string
		args []any
	}{
		{`INSERT INTO marked_entities (id, face, deleted_by, row)
		  SELECT id, face, $2, to_jsonb(e) FROM entities e WHERE id = $1`, []any{id, by}},
		// Delete first and copy what the DELETE returns. A DELETE waits for
		// and re-checks a row that a concurrent hard delete of the other end
		// holds, so an edge that delete removed is not copied here, where a
		// plain SELECT could still see it and hide a copy.
		{`WITH gone AS (DELETE FROM relations r WHERE from_id = $1 OR to_id = $1 RETURNING r.*)
		  INSERT INTO marked_relations (owner_id, from_id, from_face, rel_type, to_id, row)
		  SELECT $1, from_id, from_face, rel_type, to_id, to_jsonb(gone) FROM gone
		  ON CONFLICT DO NOTHING`, []any{id}},
		{`DELETE FROM entities WHERE id = $1`, []any{id}},
	} {
		if _, err := tx.Exec(ctx, st.q, st.args...); err != nil {
			return nil, err
		}
	}

	evs := familyEvents(store.EventEntityDeleted, store.EventRelationDeleted, family, related)
	if err := s.writeTombstonesForEvents(ctx, tx, evs); err != nil {
		return nil, err
	}
	for _, ev := range evs {
		s.notify(ctx, tx, ev)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	for _, fe := range family {
		notifyFaceDelete(s, id, fe.Face)
	}
	notifyLastFaceDelete(s, id)
	s.emitAll(evs)
	return &store.DeleteResult{DeletedEntities: family, DeletedRelations: related}, nil
}

func (d softDeleter) Unmark(ctx context.Context, id string) (*store.DeleteResult, error) {
	s := d.s
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	if lockErr := lockFamily(ctx, tx, id); lockErr != nil {
		return nil, lockErr
	}
	family, err := scanEntities(ctx, tx, markedEntitySelect+` WHERE m.id = $1 ORDER BY m.face`, id)
	if err != nil {
		return nil, err
	}
	if len(family) == 0 {
		return nil, store.ErrNotFound
	}
	// An edge whose other end is still marked stays hidden and moves to that
	// entity, so it comes back when that one does.
	const otherEnd = `CASE WHEN from_id = $1 THEN to_id ELSE from_id END`
	if _, err = tx.Exec(ctx, `UPDATE marked_relations SET owner_id = `+otherEnd+`
		WHERE owner_id = $1 AND `+otherEnd+` IN (SELECT id FROM marked_entities WHERE id <> $1)`, id); err != nil {
		return nil, err
	}
	back, err := scanRelations(ctx, tx, markedRelationSelect+` WHERE m.owner_id = $1`+relationOrder, id)
	if err != nil {
		return nil, err
	}

	for _, q := range []string{
		`INSERT INTO entities SELECT e.* FROM marked_entities m,
		   jsonb_populate_record(NULL::entities, m.row) e WHERE m.id = $1`,
		`UPDATE entities SET seq = nextval('rela_seq') WHERE id = $1`,
		`DELETE FROM marked_entities WHERE id = $1`,
		// A live edge created on the same key meanwhile wins.
		`INSERT INTO relations SELECT r.* FROM marked_relations m,
		   jsonb_populate_record(NULL::relations, m.row) r WHERE m.owner_id = $1
		 ON CONFLICT DO NOTHING`,
		`UPDATE relations SET seq = nextval('rela_seq') WHERE from_id = $1 OR to_id = $1`,
		`DELETE FROM marked_relations WHERE owner_id = $1`,
	} {
		if _, err := tx.Exec(ctx, q, id); err != nil {
			// A derived unique index (rela_derived_uniq__*) may refuse the
			// row if another entity took its value meanwhile.
			return nil, s.mapConflict(err)
		}
	}

	evs := familyEvents(store.EventEntityCreated, store.EventRelationCreated, family, back)
	for _, ev := range evs {
		s.notify(ctx, tx, ev)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	for _, e := range family {
		s.notifyPut(e)
	}
	s.emitAll(evs)
	return &store.DeleteResult{DeletedEntities: family, DeletedRelations: back}, nil
}

func (d softDeleter) ListMarked(ctx context.Context) ([]store.MarkedEntity, error) {
	rows, err := d.s.db.Query(ctx, `SELECT m.deleted_at, m.deleted_by, e.id, e.type, e.face,
		  e.properties, e.content, e.updated_at
		FROM marked_entities m, jsonb_populate_record(NULL::entities, m.row) e
		ORDER BY m.deleted_at, m.id, m.face`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []store.MarkedEntity
	for rows.Next() {
		var (
			at time.Time
			by string
		)
		e, err := scanEntity(prefixScanner{rows, []any{&at, &by}})
		if err != nil {
			return nil, err
		}
		if n := len(out); n > 0 && out[n-1].ID == e.ID {
			out[n-1].Entities = append(out[n-1].Entities, e)
			continue
		}
		out = append(out, store.MarkedEntity{ID: e.ID, DeletedAt: at, DeletedBy: by, Entities: []*entity.Entity{e}})
	}
	return out, rows.Err()
}

func (d softDeleter) PurgeMarked(ctx context.Context, id string) (*store.DeleteResult, error) {
	tx, err := d.s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	if lockErr := lockFamily(ctx, tx, id); lockErr != nil {
		return nil, lockErr
	}
	family, err := scanEntities(ctx, tx, markedEntitySelect+` WHERE m.id = $1 ORDER BY m.face`, id)
	if err != nil {
		return nil, err
	}
	if len(family) == 0 {
		return nil, store.ErrNotFound
	}
	// Hidden edges to id held by another marked entity go too; they must not
	// come back pointing at an entity that no longer exists.
	const match = ` WHERE m.owner_id = $1 OR m.from_id = $1 OR m.to_id = $1`
	rels, err := scanRelations(ctx, tx, markedRelationSelect+match+relationOrder, id)
	if err != nil {
		return nil, err
	}
	for _, q := range []string{
		`DELETE FROM marked_relations m` + match,
		`DELETE FROM attachments WHERE entity_id = $1`,
		`DELETE FROM marked_entities WHERE id = $1`,
	} {
		if _, err := tx.Exec(ctx, q, id); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &store.DeleteResult{DeletedEntities: family, DeletedRelations: rels}, nil
}

// markedIDTaken reports whether a marked entity holds id, case-folded.
// except skips one id, for a rename that only changes casing.
func markedIDTaken(ctx context.Context, q DBTX, id, except string) (bool, error) {
	var held bool
	err := q.QueryRow(ctx, `SELECT true FROM marked_entities
		WHERE lower(id) = lower($1) AND id <> $2 LIMIT 1`, id, except).Scan(&held)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return held, err
}

// revealedRelation returns the hidden edge k when ctx reveals one of its
// endpoints (see [store.WithRevealed]).
func revealedRelation(ctx context.Context, s *Store, k entity.RelationKey) (*entity.Relation, error) {
	id, ok := store.RevealedFor(ctx, k.From, k.To)
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return scanRelation(s.db.QueryRow(ctx, markedRelationSelect+` WHERE m.owner_id = $1
		AND m.from_id = $2 AND m.rel_type = $3 AND m.to_id = $4 AND m.from_face = $5`,
		id, k.From, k.Type, k.To, string(k.FromFace)))
}

// revealedRelations returns the hidden relations of the entity ctx reveals
// that satisfy q, for ListRelations.
func revealedRelations(ctx context.Context, s *Store, q store.RelationQuery) ([]*entity.Relation, error) {
	id := store.RevealedID(ctx)
	if id == "" || q.EntityID != id {
		return nil, nil
	}
	all, err := scanRelations(ctx, s.db, markedRelationSelect+` WHERE m.owner_id = $1`+relationOrder, id)
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

// familyEvents builds one event per face and per relation.
func familyEvents(
	entityOp, relationOp store.EventOp, family []*entity.Entity, rels []*entity.Relation,
) []store.Event {
	evs := make([]store.Event, 0, len(family)+len(rels))
	for _, e := range family {
		evs = append(evs, store.Event{Op: entityOp, EntityType: e.Type, EntityID: e.ID, Face: e.Face})
	}
	for _, r := range rels {
		evs = append(evs, store.Event{
			Op: relationOp, RelationType: r.Type, From: r.From, To: r.To, Face: r.FromFace,
		})
	}
	return evs
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
