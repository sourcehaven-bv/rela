package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// --- EntityReader ---------------------------------------------------------

// getEntitySQL reads one face row by its primary key (id, face).
const getEntitySQL = `SELECT ` + entityColumns + ` FROM entities WHERE id = ? AND face = ?`

// GetEntity returns the face row ref addresses, or store.ErrNotFound. A
// missing face is ErrNotFound even when sibling faces of the same id exist.
func (s *Store) GetEntity(ctx context.Context, ref entity.Ref) (*entity.Entity, error) {
	if !storeutil.Addressable(ref) {
		return nil, fmt.Errorf("sqlitestore: get %s: %w", ref, store.ErrNotFound)
	}
	e, err := scanEntity(s.q().QueryRowContext(ctx, getEntitySQL, ref.ID, string(ref.Face)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("sqlitestore: get %s: %w", ref, store.ErrNotFound)
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
	if e.Face.IsImplicit() {
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
	// Row-family invariant: a state shares the family's type. One choke point
	// for every direct writer, matching fs/mem/pg. ANY sibling answers — no
	// face heads a family (BUG-HC6I2T), so a first state is legal with no
	// siblings at all and only a DIVERGENT type is refused.
	//
	// Runs for EVERY face including the zero coordinate. Gating it on
	// `!e.Face.IsImplicit()` was complete only while every family necessarily
	// had a zero-coordinate row: a family created named-face-first would then
	// take a zero-coordinate write with no type check at all.
	var famType string
	err := s.q().QueryRowContext(ctx,
		`SELECT type FROM entities WHERE id = ? LIMIT 1`, e.ID).Scan(&famType)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// First state of a new family: nothing to disagree with.
	case err != nil:
		return fmt.Errorf("sqlitestore: create %s: %w", e.ID, err)
	case famType != e.Type:
		return storeutil.StateTypeMismatchError(e.ID, e.Face, e.Type, famType)
	}

	props, err := marshalProps(e.Properties)
	if err != nil {
		return err
	}
	// The store stamps the write time itself, as every backend does: a
	// caller's UpdatedAt is usually the stored value carried through a
	// read-modify-write, and keeping it would stop the sweep's settle window
	// and store.Freshness from ever seeing the edit.
	updated := time.Now()

	editorUser, editorTool := store.AttributionColumns(ctx)
	o := originColumns(store.OriginFrom(ctx))
	// The NOT EXISTS keeps a soft-deleted id held until it is purged. It sits
	// in the INSERT itself so no mark can land between probe and write.
	res, err := s.write(ctx, `INSERT INTO entities (id, face, type, properties, content, updated_at,
		                      last_edited_by_user, last_edited_by_tool,
		                      origin_kind, origin_source, origin_source_face,
		                      origin_source_type, origin_definition)
		SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		WHERE NOT EXISTS (SELECT 1 FROM marked_entities WHERE lower(id) = lower(?))`,
		e.ID, string(e.Face), e.Type, props, e.Content, sqlitedb.FormatTime(updated), editorUser, editorTool,
		o.kind, o.source, o.sourceFace, o.sourceType, o.definition, e.ID)
	var inserted int64
	if err == nil {
		inserted, err = res.RowsAffected()
	}
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("sqlitestore: create %s: %w",
				entity.FormatStateRef(e.ID, e.Face), store.ErrConflict)
		}
		return fmt.Errorf("sqlitestore: create %s: %w", e.ID, err)
	}
	if inserted == 0 {
		return fmt.Errorf("sqlitestore: create %s: soft-deleted id: %w",
			entity.FormatStateRef(e.ID, e.Face), store.ErrConflict)
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
	if !e.Face.IsImplicit() {
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
	updated := time.Now() // store-stamped; see CreateEntity

	// Every update restamps the origin, so an unmarked write clears a copy
	// marker: the columns describe the most recent write (see store.Origin).
	editorUser, editorTool := store.AttributionColumns(ctx)
	o := originColumns(store.OriginFrom(ctx))
	res, err := s.write(ctx, `UPDATE entities SET type = ?, properties = ?, content = ?, updated_at = ?,
		    last_edited_by_user = ?, last_edited_by_tool = ?,
		    origin_kind = ?, origin_source = ?, origin_source_face = ?,
		    origin_source_type = ?, origin_definition = ?
		WHERE id = ? AND face = ?`,
		e.Type, props, e.Content, sqlitedb.FormatTime(updated), editorUser, editorTool,
		o.kind, o.source, o.sourceFace, o.sourceType, o.definition, e.ID, string(e.Face))
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

// UpdateEntityIf implements the compare-and-swap write (TKT-34XS2R).
//
// The compare cannot be a SQL predicate: the version is a hash over DECODED
// properties (see [store.VersionOf]), and the stored `properties` column is
// JSON whose byte encoding is not canonical — two encodings of the same map
// would compare unequal, so a `WHERE version = ?` would reject writes that
// ought to succeed. The read and the write therefore run inside one
// transaction instead.
//
// That transaction is what makes this atomic rather than a check-then-write:
// Tx opens with BEGIN IMMEDIATE, taking SQLite's write lock up front, so no
// other writer can land between the read and the UPDATE. Nesting is safe — a
// call from inside an existing Tx joins it (see [Store.Tx]).
//
// Note sqlitestore is single-process by construction (DEC-LFSYNY takes an
// exclusive sidecar lock on open), so the cross-process case pgstore must
// handle cannot arise here. The CAS is still load-bearing for concurrent
// goroutines within the one permitted process.
func (s *Store) UpdateEntityIf(
	ctx context.Context, e *entity.Entity, cond store.UpdateCondition,
) (store.EntityVersion, error) {
	if cond.IsZero() {
		return store.VersionOf(e), s.UpdateEntity(ctx, e)
	}

	var version store.EntityVersion
	err := s.Tx(ctx, func(tx store.Store) error {
		view, ok := tx.(*Store)
		if !ok { // unreachable: Tx always hands back our own view type
			return errors.New("sqlitestore: unexpected transaction view type")
		}
		current, gErr := view.GetEntity(ctx, e.Ref())
		if gErr != nil {
			return gErr // already ErrNotFound-wrapped by GetEntity
		}
		if actual := store.VersionOf(current); actual != cond.ExpectedVersion {
			return &store.VersionConflictError{
				ID: e.ID, Expected: cond.ExpectedVersion, Actual: actual,
			}
		}
		if uErr := view.UpdateEntity(ctx, e); uErr != nil {
			return uErr
		}
		version = store.VersionOf(e)
		return nil
	})
	if err != nil {
		return "", err
	}
	return version, nil
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
func (s *Store) DeleteFamily(ctx context.Context, id string, cascade bool) (*store.DeleteResult, error) {
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

	if _, err := s.write(ctx, dropMarkedEdgesSQL, id, id); err != nil {
		return nil, fmt.Errorf("sqlitestore: delete %s hidden relations: %w", id, err)
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

// DeleteFace removes ONE face row and the edges [store.EntityWriter.DeleteFace]
// says belong to it (TKT-C1XUA8, RR-2466U1): the outgoing edges tailed at the
// face, or every incident edge when it is the family's last face.
//
// Contrast DeleteFamily above, which sweeps the whole family. Reusing that here
// would make discarding a draft destroy the published face and cut every
// inbound link unrelated entities hold on it.
//
// Transacted for the same reason DeleteFamily is: it issues several statements
// plus a check-then-act on the sibling count.
func (s *Store) DeleteFace(ctx context.Context, ref entity.Ref) (*store.DeleteResult, error) {
	if !storeutil.Addressable(ref) {
		return nil, fmt.Errorf("sqlitestore: delete %s: %w", ref, store.ErrNotFound)
	}
	var result *store.DeleteResult
	err := s.Tx(ctx, func(tx store.Store) error {
		view, ok := tx.(*Store)
		if !ok { // unreachable: Tx always hands back our own view type
			return errors.New("sqlitestore: unexpected transaction view type")
		}
		var derr error
		result, derr = view.deleteFaceLocked(ctx, ref.ID, ref.Face)
		return derr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) deleteFaceLocked(
	ctx context.Context, id string, p entity.Face,
) (*store.DeleteResult, error) {
	target, err := s.GetEntity(ctx, entity.Ref{ID: id, Face: p})
	if err != nil {
		return nil, err
	}
	var size int
	if cErr := s.q().QueryRowContext(ctx,
		`SELECT count(*) FROM entities WHERE id = ?`, id).Scan(&size); cErr != nil {
		return nil, fmt.Errorf("sqlitestore: delete state %s: %w", id, cErr)
	}
	last := size == 1

	// The last face takes every incident edge, as DeleteFamily does
	// (RR-2466U1); otherwise only the outgoing edges tailed at this face.
	var owned []*entity.Relation
	if last {
		owned, err = s.incidentRelations(ctx, id)
	} else {
		owned, err = s.ownedRelations(ctx, id, p)
	}
	if err != nil {
		return nil, err
	}
	ownedDelete, ownedArgs := `DELETE FROM relations WHERE from_id = ? AND from_face = ?`, []any{id, string(p)}
	if last {
		ownedDelete, ownedArgs = `DELETE FROM relations WHERE from_id = ? OR to_id = ?`, []any{id, id}
	}
	if _, err := s.write(ctx, ownedDelete, ownedArgs...); err != nil {
		return nil, fmt.Errorf("sqlitestore: delete state %s relations: %w", id, err)
	}
	if _, err := s.write(ctx,
		`DELETE FROM entities WHERE id = ? AND face = ?`, id, string(p)); err != nil {
		return nil, fmt.Errorf("sqlitestore: delete state %s: %w", id, err)
	}

	// Attachments are keyed to the BARE id, so they belong to the entity rather
	// than to a face: only sweep them once the last face is gone. A discarded
	// draft must not destroy attachments the surviving faces serve.
	s.notifyFaceDelete(id, p)
	if last {
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
// included; see DeleteFace.
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
	t, err := parseTime(raw.String)
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
	t, err := parseTime(updated)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: parse updated_at for %s: %w", e.ID, err)
	}
	e.UpdatedAt = t
	return &e, nil
}
