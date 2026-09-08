package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// --- EntityReader ---------------------------------------------------------

// GetEntity returns a single entity by ID, or store.ErrNotFound. The bare id
// addresses the DEFAULT state (TKT-DOFYR1).
func (s *Store) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	return s.GetEntityState(ctx, id, "")
}

// GetEntityState returns the content state addressed by (id, p); the zero face
// is the default state. ErrNotFound covers a missing state even when sibling
// states of the same id exist.
func (s *Store) GetEntityState(ctx context.Context, id string, p entity.Face) (*entity.Entity, error) {
	row := s.q().QueryRowContext(ctx,
		`SELECT `+entityColumns+` FROM entities WHERE id = ? AND face = ?`, id, string(p))
	e, err := scanEntity(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("sqlitestore: get %s: %w", entity.FormatStateRef(id, p), store.ErrNotFound)
	}
	return e, err
}

func (s *Store) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	if err := storeutil.ValidateEntityQuery(q); err != nil {
		return func(yield func(*entity.Entity, error) bool) { yield(nil, err) }
	}
	sqlText, args := buildEntitySelectSQL(q, "", entityColumns)
	return func(yield func(*entity.Entity, error) bool) {
		rows, err := s.q().QueryContext(ctx, sqlText, args...)
		if err != nil {
			yield(nil, fmt.Errorf("sqlitestore: list entities: %w", err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			e, err := scanEntity(rows)
			if !yield(e, err) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(nil, fmt.Errorf("sqlitestore: list entities: %w", err))
		}
	}
}

func (s *Store) CountEntities(ctx context.Context, q store.EntityQuery) (int, error) {
	if err := storeutil.ValidateEntityQuery(q); err != nil {
		return 0, err
	}
	sqlText, args := buildEntityCountSQL(q)
	var n int
	if err := s.q().QueryRowContext(ctx, sqlText, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("sqlitestore: count entities: %w", err)
	}
	return n, nil
}

// --- EntityWriter ---------------------------------------------------------

// CreateEntity returns store.ErrConflict when the (id, face) slot is taken.
// The stress test's plain writer tolerates ONLY ErrConflict here, so the
// mapping from SQLite's UNIQUE violation must be exact.
//
// A non-default face additionally has to satisfy the row-family invariants
// (TKT-DOFYR1, design doc §6): no headless states, one type per family. Those
// are a check-then-act pair against the default row, so the whole method runs
// in a transaction — without one a concurrent family delete could commit
// between probe and insert and materialize a headless state. A default-face
// create needs no probe and takes the plain path.
func (s *Store) CreateEntity(ctx context.Context, e *entity.Entity) error {
	if err := storeutil.ValidateID(e.ID); err != nil {
		return fmt.Errorf("sqlitestore: create: %w", err)
	}
	if e.Face.IsDefault() {
		return s.createEntityLocked(ctx, e)
	}
	return s.Tx(ctx, func(tx store.Store) error {
		view, ok := tx.(*Store)
		if !ok { // unreachable: Tx always hands back our own view type
			return errors.New("sqlitestore: unexpected transaction view type")
		}
		return view.createEntityLocked(ctx, e)
	})
}

func (s *Store) createEntityLocked(ctx context.Context, e *entity.Entity) error {
	if !e.Face.IsDefault() {
		// Row-family invariants: a non-default state requires its default row
		// and shares the family's type. One choke point for every direct
		// writer, matching fs/mem/pg.
		var defType string
		err := s.q().QueryRowContext(ctx,
			`SELECT type FROM entities WHERE id = ? AND face = ''`, e.ID).Scan(&defType)
		if errors.Is(err, sql.ErrNoRows) {
			return storeutil.HeadlessStateError(e.ID)
		}
		if err != nil {
			return fmt.Errorf("sqlitestore: create %s: %w", e.ID, err)
		}
		if defType != e.Type {
			return storeutil.StateTypeMismatchError(e.ID, e.Face, e.Type, defType)
		}
	}

	props, err := marshalProps(e.Properties)
	if err != nil {
		return err
	}
	updated := e.UpdatedAt
	if updated.IsZero() {
		updated = time.Now().UTC()
	}

	_, err = s.write(ctx, `INSERT INTO entities (id, face, type, properties, content, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		e.ID, string(e.Face), e.Type, props, e.Content, updated.Format(timeFmt))
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("sqlitestore: create %s: %w",
				entity.FormatStateRef(e.ID, e.Face), store.ErrConflict)
		}
		return fmt.Errorf("sqlitestore: create %s: %w", e.ID, err)
	}

	s.notifyPut(e)
	s.emit(store.Event{
		Op: store.EventEntityCreated, EntityID: e.ID, EntityType: e.Type, Face: e.Face,
	})
	return nil
}

func (s *Store) UpdateEntity(ctx context.Context, e *entity.Entity) error {
	// Row-family invariant: a non-default state cannot be re-typed away from
	// its family (TKT-DOFYR1, design doc §6). The default face carries the
	// family's type, so re-typing IT is the legitimate whole-family retype the
	// storetest UpdateChangesType case covers.
	if !e.Face.IsDefault() {
		var curType string
		err := s.q().QueryRowContext(ctx,
			`SELECT type FROM entities WHERE id = ? AND face = ?`, e.ID, string(e.Face)).Scan(&curType)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("sqlitestore: update %s: %w",
				entity.FormatStateRef(e.ID, e.Face), store.ErrNotFound)
		}
		if err != nil {
			return fmt.Errorf("sqlitestore: update %s: %w", e.ID, err)
		}
		if curType != e.Type {
			return storeutil.StateTypeMismatchError(e.ID, e.Face, e.Type, curType)
		}
	}

	props, err := marshalProps(e.Properties)
	if err != nil {
		return err
	}
	updated := e.UpdatedAt
	if updated.IsZero() {
		updated = time.Now().UTC()
	}

	res, err := s.write(ctx, `UPDATE entities SET type = ?, properties = ?, content = ?, updated_at = ?
		WHERE id = ? AND face = ?`,
		e.Type, props, e.Content, updated.Format(timeFmt), e.ID, string(e.Face))
	if err != nil {
		return fmt.Errorf("sqlitestore: update %s: %w", e.ID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlitestore: update %s: %w", e.ID, err)
	}
	if n == 0 {
		return fmt.Errorf("sqlitestore: update %s: %w",
			entity.FormatStateRef(e.ID, e.Face), store.ErrNotFound)
	}

	s.notifyPut(e)
	s.emit(store.Event{
		Op: store.EventEntityUpdated, EntityID: e.ID, EntityType: e.Type, Face: e.Face,
	})
	return nil
}

// DeleteEntity refuses an entity with relations unless cascade is set. The
// stress test tolerates ErrNotFound and ErrHasRelations here and nothing
// else, so both must be reported precisely.
// DeleteEntity removes an entity, its attachments, and (with cascade) its
// relations.
//
// Wrapped in a transaction like RenameEntity: it issues four statements, so
// without one a failure between them leaves relations gone but the entity
// present, or the entity gone with orphaned attachment rows. The Tx also closes
// a check-then-act race — relCount is read and then acted on, and a plain
// CreateRelation could otherwise slip in between, so a non-cascade delete would
// silently remove an entity that had just gained an edge.
//
// Nesting is safe: a call from inside an existing Tx joins it rather than
// opening a second one.
func (s *Store) DeleteEntity(ctx context.Context, id string, cascade bool) (*store.DeleteResult, error) {
	var result *store.DeleteResult
	err := s.Tx(ctx, func(tx store.Store) error {
		view, ok := tx.(*Store)
		if !ok { // unreachable: Tx always hands back our own view type
			return errors.New("sqlitestore: unexpected transaction view type")
		}
		var derr error
		result, derr = view.deleteEntityLocked(ctx, id, cascade)
		return derr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) deleteEntityLocked(
	ctx context.Context, id string, cascade bool,
) (*store.DeleteResult, error) {
	// Delete addresses the whole state FAMILY of the bare id (TKT-DOFYR1).
	// The scan is defensive so a headless family — which the load path
	// tolerates even though no write path can create one — still deletes
	// cleanly.
	family, err := s.stateFamily(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(family) == 0 {
		return nil, fmt.Errorf("sqlitestore: delete %s: %w", id, store.ErrNotFound)
	}

	var relCount int
	if err := s.q().QueryRowContext(ctx,
		`SELECT count(*) FROM relations WHERE from_id = ? OR to_id = ?`, id, id).Scan(&relCount); err != nil {
		return nil, fmt.Errorf("sqlitestore: delete %s: %w", id, err)
	}
	if relCount > 0 && !cascade {
		return nil, fmt.Errorf("sqlitestore: delete %s: %w", id, store.ErrHasRelations)
	}

	// The deleted entities are part of the result contract: callers use them to
	// report what went away, and a version-capturing backend needs the
	// pre-delete state, which no longer exists once the rows are gone.
	result := &store.DeleteResult{DeletedEntities: family}
	if cascade && relCount > 0 {
		// Collect BEFORE deleting: the result reports which relations went, and
		// after the DELETE there is nothing left to enumerate. The bare
		// from_id/to_id match sweeps EVERY tail face.
		incident, err := s.incidentRelations(ctx, id)
		if err != nil {
			return nil, err
		}
		result.DeletedRelations = incident
		if _, err := s.write(ctx, `DELETE FROM relations WHERE from_id = ? OR to_id = ?`, id, id); err != nil {
			return nil, fmt.Errorf("sqlitestore: delete %s relations: %w", id, err)
		}
	}

	// Attachments are owned by the entity, so they go with it regardless of
	// cascade — that flag governs RELATIONS, which have another endpoint and so
	// need the caller's consent to remove.
	if _, err := s.write(ctx, `DELETE FROM attachments WHERE entity_id = ?`, id); err != nil {
		return nil, fmt.Errorf("sqlitestore: delete %s attachments: %w", id, err)
	}
	if _, err := s.write(ctx, `DELETE FROM entities WHERE id = ?`, id); err != nil {
		return nil, fmt.Errorf("sqlitestore: delete %s: %w", id, err)
	}

	for _, r := range result.DeletedRelations {
		s.emit(store.Event{
			Op: store.EventRelationDeleted, RelationType: r.Type,
			From: r.From, To: r.To, Face: r.FromFace,
		})
	}
	for _, fe := range family {
		s.notifyFaceDelete(id, fe.Face)
	}
	// The whole family went, so the bare-id observers hear one delete.
	s.notifyLastFaceDelete(id)
	for _, fe := range family {
		// Per-face type: the load path tolerates a mistyped state, so one
		// family-wide type would misreport it.
		s.emit(store.Event{
			Op: store.EventEntityDeleted, EntityID: id, EntityType: fe.Type, Face: fe.Face,
		})
	}
	return result, nil
}

// DeleteEntityState removes ONE content state (face) and only the edges that
// belong to it (TKT-C1XUA8).
//
// Contrast DeleteEntity above, which sweeps the whole family and every incident
// edge on BOTH sides. Reusing that here would make discarding a draft destroy
// the published face and cut every inbound link unrelated entities hold on it —
// so this deletes by (id, face), and among relations only the OUTGOING edges
// whose tail is this face. Incoming edges survive: heads are entity-level
// (design doc §2.3), so an inbound edge points at the ENTITY, not at one of its
// faces.
//
// Transacted for the same reason DeleteEntity is: it issues several statements
// plus a check-then-act on the sibling count.
func (s *Store) DeleteEntityState(
	ctx context.Context, id string, p entity.Face,
) (*store.DeleteResult, error) {
	var result *store.DeleteResult
	err := s.Tx(ctx, func(tx store.Store) error {
		view, ok := tx.(*Store)
		if !ok { // unreachable: Tx always hands back our own view type
			return errors.New("sqlitestore: unexpected transaction view type")
		}
		var derr error
		result, derr = view.deleteEntityStateLocked(ctx, id, p)
		return derr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) deleteEntityStateLocked(
	ctx context.Context, id string, p entity.Face,
) (*store.DeleteResult, error) {
	target, err := s.GetEntityState(ctx, id, p)
	if err != nil {
		return nil, err
	}

	// Refuse to orphan the family: a family with no default row has no defined
	// meaning, and world fallback (`otherwise: default`) resolves against it.
	// Deleting the LAST face is fine — nothing is left to orphan.
	if p.IsDefault() {
		var others int
		if cErr := s.q().QueryRowContext(ctx,
			`SELECT count(*) FROM entities WHERE id = ? AND face <> ''`, id).Scan(&others); cErr != nil {
			return nil, fmt.Errorf("sqlitestore: delete state %s: %w", id, cErr)
		}
		if others > 0 {
			return nil, fmt.Errorf(
				"%w: cannot delete the default face of %s while %d other state(s) remain",
				store.ErrInvalidQuery, id, others)
		}
	}

	owned, err := s.ownedRelations(ctx, id, p)
	if err != nil {
		return nil, err
	}
	if _, err := s.write(ctx,
		`DELETE FROM relations WHERE from_id = ? AND from_face = ?`, id, string(p)); err != nil {
		return nil, fmt.Errorf("sqlitestore: delete state %s relations: %w", id, err)
	}
	if _, err := s.write(ctx,
		`DELETE FROM entities WHERE id = ? AND face = ?`, id, string(p)); err != nil {
		return nil, fmt.Errorf("sqlitestore: delete state %s: %w", id, err)
	}

	// Attachments are keyed to the BARE id, so they belong to the entity rather
	// than to a face: only sweep them once the last face is gone. A discarded
	// draft must not destroy attachments the surviving faces serve.
	var remaining int
	if cErr := s.q().QueryRowContext(ctx,
		`SELECT count(*) FROM entities WHERE id = ?`, id).Scan(&remaining); cErr != nil {
		return nil, fmt.Errorf("sqlitestore: delete state %s: %w", id, cErr)
	}
	s.notifyFaceDelete(id, p)
	if remaining == 0 {
		if _, err := s.write(ctx, `DELETE FROM attachments WHERE entity_id = ?`, id); err != nil {
			return nil, fmt.Errorf("sqlitestore: delete state %s attachments: %w", id, err)
		}
		// Observers keyed on the bare id must NOT hear a delete while other
		// faces remain — that would de-index an entity the store still holds.
		s.notifyLastFaceDelete(id)
	}

	s.emit(store.Event{
		Op: store.EventEntityDeleted, EntityID: id, EntityType: target.Type, Face: p,
	})
	for _, r := range owned {
		s.emit(store.Event{
			Op: store.EventRelationDeleted, RelationType: r.Type,
			From: r.From, To: r.To, Face: r.FromFace,
		})
	}
	return &store.DeleteResult{
		DeletedEntities: []*entity.Entity{target}, DeletedRelations: owned,
	}, nil
}

// stateFamily loads every content state of the bare id, ordered by face so the
// default row leads. Its own function so the rows handle can be closed with
// defer rather than by hand on each return path.
func (s *Store) stateFamily(ctx context.Context, id string) ([]*entity.Entity, error) {
	rows, err := s.q().QueryContext(ctx,
		`SELECT `+entityColumns+` FROM entities WHERE id = ? ORDER BY face`, id)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: state family for %s: %w", id, err)
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
		return nil, fmt.Errorf("sqlitestore: state family for %s: %w", id, err)
	}
	return out, nil
}

// incidentRelations lists every relation with id at either endpoint, across
// every tail face. Its own function so the rows handle can be closed with defer
// rather than by hand on each return path.
func (s *Store) incidentRelations(ctx context.Context, id string) ([]*entity.Relation, error) {
	return s.scanRelationKeys(ctx,
		`SELECT from_id, from_face, rel_type, to_id FROM relations
		 WHERE from_id = ? OR to_id = ? ORDER BY from_id, from_face, rel_type, to_id`, id, id)
}

// ownedRelations lists the OUTGOING edges whose tail is exactly p — the edges
// that belong to one face and go with it. Incoming edges are deliberately not
// included; see DeleteEntityState.
func (s *Store) ownedRelations(
	ctx context.Context, id string, p entity.Face,
) ([]*entity.Relation, error) {
	return s.scanRelationKeys(ctx,
		`SELECT from_id, from_face, rel_type, to_id FROM relations
		 WHERE from_id = ? AND from_face = ? ORDER BY rel_type, to_id`, id, string(p))
}

// scanRelationKeys runs a query selecting the four identity columns of a
// relation. Only the key is read: these results feed a DeleteResult and the
// deletion events, neither of which carries properties or content.
func (s *Store) scanRelationKeys(
	ctx context.Context, query string, args ...any,
) ([]*entity.Relation, error) {
	rows, err := s.q().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: scan relation keys: %w", err)
	}
	defer rows.Close()

	var out []*entity.Relation
	for rows.Next() {
		var (
			r    entity.Relation
			face string
		)
		if err := rows.Scan(&r.From, &face, &r.Type, &r.To); err != nil {
			return nil, fmt.Errorf("sqlitestore: scan relation keys: %w", err)
		}
		r.FromFace = entity.Face(face)
		out = append(out, &r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlitestore: scan relation keys: %w", err)
	}
	return out, nil
}

// --- Freshness / Lifecycle ------------------------------------------------

func (s *Store) LastModified(ctx context.Context) (time.Time, error) {
	var raw sql.NullString
	if err := s.q().QueryRowContext(ctx,
		`SELECT max(updated_at) FROM (
			SELECT updated_at FROM entities UNION ALL SELECT updated_at FROM relations)`).Scan(&raw); err != nil {
		return time.Time{}, fmt.Errorf("sqlitestore: last modified: %w", err)
	}
	if !raw.Valid || raw.String == "" {
		return time.Time{}, nil // empty store: zero time, per contract
	}
	t, err := time.Parse(timeFmt, raw.String)
	if err != nil {
		return time.Time{}, fmt.Errorf("sqlitestore: last modified: %w", err)
	}
	return t, nil
}

// --- helpers --------------------------------------------------------------

type scanner interface {
	Scan(dest ...any) error
}

func scanEntity(sc scanner) (*entity.Entity, error) {
	var (
		e       entity.Entity
		face    string
		props   string
		updated string
	)
	if err := sc.Scan(&e.ID, &e.Type, &face, &props, &e.Content, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("sqlitestore: scan entity: %w", err)
	}
	// The stored column is the canonical serialized coordinate; the store only
	// equality-matches it, never parses it (see entity.Face).
	e.Face = entity.Face(face)
	props2, err := unmarshalProps(props)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: entity %s: %w", e.ID, err)
	}
	e.Properties = props2
	t, err := time.Parse(timeFmt, updated)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: parse updated_at for %s: %w", e.ID, err)
	}
	e.UpdatedAt = t
	return &e, nil
}
