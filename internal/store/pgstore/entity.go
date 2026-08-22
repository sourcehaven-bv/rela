package pgstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// checkQueryScope rejects a query this backend cannot answer: the shared
// AllStates+World contradiction rule (storeutil.ValidateEntityQuery).
//
// The transitional refusal of a non-default World is GONE as of PR-C —
// world scoping is pushed into SQL (see worldSQL / buildEntitySelectSQL),
// so there is nothing left to refuse.
func checkQueryScope(q store.EntityQuery) error {
	return storeutil.ValidateEntityQuery(q)
}

// --- EntityReader ---

// GetEntity returns a single entity by ID, or store.ErrNotFound. The
// bare id addresses the DEFAULT state (TKT-DOFYR1).
func (s *Store) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	return s.GetEntityState(ctx, id, "")
}

// GetEntityState returns the content state addressed by (id, p); the
// zero pointer is the default state. ErrNotFound covers a missing state
// even when sibling states exist.
func (s *Store) GetEntityState(ctx context.Context, id string, p entity.Pointer) (*entity.Entity, error) {
	const q = `SELECT id, type, pointer, properties, content, updated_at
	           FROM entities WHERE id = $1 AND pointer = $2`
	e, err := scanEntity(s.db.QueryRow(ctx, q, id, p))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return e, nil
}

// ListEntities streams entities matching q in ascending-ID order. Cursor and
// Limit are ignored (per the EntityReader contract).
func (s *Store) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	if err := checkQueryScope(q); err != nil {
		return func(yield func(*entity.Entity, error) bool) { yield(nil, err) }
	}
	sql, args := buildEntityListSQL(q, "")
	return func(yield func(*entity.Entity, error) bool) {
		rows, err := s.db.Query(ctx, sql, args...)
		if err != nil {
			yield(nil, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			e, err := scanEntity(rows)
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(e, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(nil, err)
		}
	}
}

// ListEntityHeaders implements store.HeaderReader: the same listing as
// ListEntities with the content column left OUT of the SELECT, so entity
// bodies are never read from disk, never cross the wire, and never land in
// a pgx scan buffer.
//
// This is the point of the capability — the generic fallback in
// store.ListEntityHeaders bounds only retention, whereas here a 20k-row
// scan over ~2 GB of markdown transfers a few MB of ids and properties.
func (s *Store) ListEntityHeaders(
	ctx context.Context, q store.EntityQuery,
) iter.Seq2[store.EntityHeader, error] {
	if err := checkQueryScope(q); err != nil {
		return func(yield func(store.EntityHeader, error) bool) { yield(store.EntityHeader{}, err) }
	}
	sql, args := buildEntityHeaderListSQL(q, "")
	return func(yield func(store.EntityHeader, error) bool) {
		rows, err := s.db.Query(ctx, sql, args...)
		if err != nil {
			yield(store.EntityHeader{}, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			h, err := scanEntityHeader(rows)
			if err != nil {
				yield(store.EntityHeader{}, err)
				return
			}
			if !yield(h, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(store.EntityHeader{}, err)
		}
	}
}

// ListEntitiesPage returns a page of entities. A keyset cursor on id keeps
// pages stable; see store.ListEntitiesPage for the contract.
func (s *Store) ListEntitiesPage(ctx context.Context, q store.EntityQuery) (store.Page[*entity.Entity], error) {
	if err := checkQueryScope(q); err != nil {
		return store.Page[*entity.Entity]{}, err
	}
	cursorKey, err := storeutil.DecodeCursor(q.Cursor)
	if err != nil {
		return store.Page[*entity.Entity]{}, err
	}

	// Fetch limit+1 to detect whether a further page exists.
	fetch := q.Limit
	if fetch > 0 {
		fetch++
	}
	sql, args := buildEntityListSQL(q, cursorKey)
	if fetch > 0 {
		sql += fmt.Sprintf(" LIMIT %d", fetch)
	}

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return store.Page[*entity.Entity]{}, err
	}
	defer rows.Close()

	items := make([]*entity.Entity, 0)
	for rows.Next() {
		e, err := scanEntity(rows)
		if err != nil {
			return store.Page[*entity.Entity]{}, err
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return store.Page[*entity.Entity]{}, err
	}

	var next string
	if q.Limit > 0 && len(items) > q.Limit {
		last := items[q.Limit-1]
		items = items[:q.Limit]
		// The cursor is the STATE key so AllStates pagination resumes
		// mid-family; for default-only queries it degenerates to the
		// historical bare id.
		next = storeutil.EncodeCursor(entity.FormatStateRef(last.ID, last.Pointer))
	}
	return store.Page[*entity.Entity]{Items: items, NextCursor: next}, nil
}

// CountEntities counts entities matching q.
func (s *Store) CountEntities(ctx context.Context, q store.EntityQuery) (int, error) {
	if err := checkQueryScope(q); err != nil {
		return 0, err
	}
	sql, args := buildEntityCountSQL(q)
	var n int
	if err := s.db.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// buildEntityCountSQL counts the entities in scope.
//
// Under a world it counts PRIMES, not candidate rows: an entity holding
// three faces is ONE entity in the world, so `count(*)` over the widened
// candidate set would over-count exactly those families that have several
// coordinates. `count(DISTINCT id)` is enough here — the world admits at
// most one prime per id, so distinct ids and primes are the same number,
// and it avoids materializing the DISTINCT ON.
//
// Counts must be world-scoped at all (RR-EHER1V): existence in a world is
// the publication bit, so an unscoped tally would tell a published-world
// surface how many unpublished drafts exist.
func buildEntityCountSQL(q store.EntityQuery) (sql string, args []any) {
	if q.World.IsDefaultWorld() {
		where, wargs := entityWhere(q, "")
		return "SELECT count(*) FROM entities" + where, wargs
	}
	_, candidate := worldSQL(q.World, "", &args)
	scope := entityScopeWhere(q, candidate, &args)
	return "SELECT count(DISTINCT id) FROM entities" + scope, args
}

// HighestID returns the highest numeric suffix among IDs of the form
// "<prefix>-<n>", or 0. Matching memstore/fsstore: non-numeric suffixes are
// skipped and gaps are ignored. The parse is done in Go (not SQL) to keep the
// Sscanf("%d") semantics identical across backends.
func (s *Store) HighestID(ctx context.Context, prefix string) (int, error) {
	pfx := prefix + "-"
	// pointer = '': states share their family's number (TKT-DOFYR1).
	const q = `SELECT id FROM entities WHERE id LIKE $1 AND pointer = ''`
	rows, err := s.db.Query(ctx, q, pfx+"%")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	highest := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		if !strings.HasPrefix(id, pfx) {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(id[len(pfx):], "%d", &n); err == nil && n > highest {
			highest = n
		}
	}
	return highest, rows.Err()
}

// PropertyValues returns distinct values of a top-level property, ordered by
// frequency (desc), then value (asc) for stable ties. Values are stringified
// to match memstore's fmt.Sprintf("%v") behavior; empty strings are skipped.
func (s *Store) PropertyValues(ctx context.Context, property string, limit int) ([]string, error) {
	// pointer = '': DEFAULT-WORLD aggregate, deliberately un-worlded
	// (TKT-WAV8XP PR-C). Suggestion counts are a default-world aggregate
	// (TKT-DOFYR1) — a state row must not inflate its family's values.
	const q = `SELECT properties -> $1 AS v FROM entities WHERE properties ? $1 AND pointer = ''`
	rows, err := s.db.Query(ctx, q, property)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		val := stringifyJSONValue(raw)
		if val != "" {
			counts[val]++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	type vc struct {
		value string
		count int
	}
	sorted := make([]vc, 0, len(counts))
	for v, c := range counts {
		sorted = append(sorted, vc{v, c})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].count != sorted[j].count {
			return sorted[i].count > sorted[j].count
		}
		return sorted[i].value < sorted[j].value
	})

	result := make([]string, 0, len(sorted))
	for i := 0; i < len(sorted) && (limit == 0 || i < limit); i++ {
		result = append(result, sorted[i].value)
	}
	return result, nil
}

// --- EntityWriter ---

// attributionValues returns the last_edited_by_user / last_edited_by_tool SQL
// values for the boundary-populated store.Attribution on ctx. Absent (or
// empty-component) attribution maps to NULL, never to an empty or placeholder
// string — NULL is the "unknown editor" encoding the version sweep's
// system-principal fallback keys on (TKT-ZIRMGM, RR-U964M0).
func attributionValues(ctx context.Context) (user, tool *string) {
	a := store.AttributionFrom(ctx)
	if a.User != "" {
		user = &a.User
	}
	if a.Tool != "" {
		tool = &a.Tool
	}
	return user, tool
}

// CreateEntity inserts a new entity. Returns store.ErrConflict if the ID
// exists. The created entity (with server-assigned updated_at) is delivered
// to observers and an EventEntityCreated is emitted after commit.
func (s *Store) CreateEntity(ctx context.Context, e *entity.Entity) error {
	if err := validateID(e.ID); err != nil {
		return err
	}

	props, err := marshalProps(e.Properties)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	if !e.Pointer.IsDefault() {
		// Row-family invariants (TKT-DOFYR1, design doc §6): a
		// non-default state requires its default row (no headless
		// states) and shares the family's type. FOR SHARE is the
		// load-bearing part: under READ COMMITTED a plain probe leaves a
		// check-then-act window in which a concurrent family delete
		// commits between probe and insert, materializing a headless
		// state. The share lock on the default row blocks that delete
		// until this insert commits.
		var defType string
		probeErr := tx.QueryRow(ctx,
			`SELECT type FROM entities WHERE id = $1 AND pointer = '' FOR SHARE`, e.ID).Scan(&defType)
		if errors.Is(probeErr, pgx.ErrNoRows) {
			return storeutil.HeadlessStateError(e.ID)
		}
		if probeErr != nil {
			return probeErr
		}
		if defType != e.Type {
			return storeutil.StateTypeMismatchError(e.ID, e.Pointer, e.Type, defType)
		}
	}

	editorUser, editorTool := attributionValues(ctx)
	const q = `
		INSERT INTO entities (id, pointer, type, properties, content, search_text, updated_at,
		                      last_edited_by_user, last_edited_by_tool)
		VALUES ($1, $2, $3, $4, $5, $6, now(), $7, $8)
		ON CONFLICT (id, pointer) DO NOTHING
		RETURNING updated_at`
	var updatedAt time.Time
	err = tx.QueryRow(ctx, q, e.ID, e.Pointer, e.Type, props, e.Content, entitySearchText(e),
		editorUser, editorTool).Scan(&updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ErrConflict
	}
	// ON CONFLICT (id) only swallows a clash on the byte-exact primary key. A
	// clash on the case-folded identity index (entities_id_lower_key, e.g.
	// creating "ABC" when "abc" exists) surfaces as a unique violation instead
	// — it is the same "already exists" outcome, so it maps to ErrConflict
	// rather than leaking a driver error (BUG-3RCWNS). A clash on a derived
	// unique index (rela_derived_uniq__*, TKT-3Q0GP1) instead maps to
	// store.UniquePropertyError naming the property; mapConflict discriminates by
	// constraint name and passes non-23505 errors through unchanged.
	if err != nil {
		return s.mapConflict(err)
	}

	ev := store.Event{Op: store.EventEntityCreated, EntityType: e.Type, EntityID: e.ID, Pointer: e.Pointer}
	s.notify(ctx, tx, ev) // cross-process NOTIFY, atomic with the write
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	stored := e.Clone()
	stored.UpdatedAt = updatedAt
	s.notifyPut(stored)
	s.emit(ev)
	return nil
}

// UpdateEntity overwrites an existing entity. Returns store.ErrNotFound if the
// entity does not exist.
func (s *Store) UpdateEntity(ctx context.Context, e *entity.Entity) error {
	props, err := marshalProps(e.Properties)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	if !e.Pointer.IsDefault() {
		// Row-family invariant: a non-default state cannot be re-typed
		// away from its family (TKT-DOFYR1, design doc §6).
		var curType string
		probeErr := tx.QueryRow(ctx,
			`SELECT type FROM entities WHERE id = $1 AND pointer = $2`, e.ID, e.Pointer).Scan(&curType)
		if errors.Is(probeErr, pgx.ErrNoRows) {
			return store.ErrNotFound
		}
		if probeErr != nil {
			return probeErr
		}
		if curType != e.Type {
			return storeutil.StateTypeMismatchError(e.ID, e.Pointer, e.Type, curType)
		}
	}

	editorUser, editorTool := attributionValues(ctx)
	const q = `
		UPDATE entities
		SET type = $3, properties = $4, content = $5, search_text = $6,
		    updated_at = now(), seq = nextval('rela_seq'),
		    last_edited_by_user = $7, last_edited_by_tool = $8
		WHERE id = $1 AND pointer = $2
		RETURNING updated_at`
	var updatedAt time.Time
	err = tx.QueryRow(ctx, q, e.ID, e.Pointer, e.Type, props, e.Content, entitySearchText(e),
		editorUser, editorTool).Scan(&updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ErrNotFound
	}
	if err != nil {
		// An update can violate a derived unique index too — e.g. an automation
		// sets a unique property to a value another entity already holds
		// (TKT-3Q0GP1). mapConflict names the property for a rela_derived_uniq__*
		// clash and passes other errors through unchanged.
		return s.mapConflict(err)
	}

	ev := store.Event{Op: store.EventEntityUpdated, EntityType: e.Type, EntityID: e.ID, Pointer: e.Pointer}
	s.notify(ctx, tx, ev)
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	stored := e.Clone()
	stored.UpdatedAt = updatedAt
	s.notifyPut(stored)
	s.emit(ev)
	return nil
}

// DeleteEntity removes an entity. Without cascade, returns store.ErrHasRelations
// if any relation references it. With cascade, deletes referencing relations
// and the entity's attachments in one transaction, returning the removed rows.
func (s *Store) DeleteEntity(ctx context.Context, id string, cascade bool) (*store.DeleteResult, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	// Delete addresses the whole state FAMILY of the bare id
	// (TKT-DOFYR1): the WHERE id = $1 statements below sweep every
	// state row, and the bare From/To match sweeps every relation tail.
	family, err := scanEntities(ctx, tx,
		`SELECT id, type, pointer, properties, content, updated_at
		 FROM entities WHERE id = $1 ORDER BY pointer ASC`, id)
	if err != nil {
		return nil, err
	}
	if len(family) == 0 {
		return nil, store.ErrNotFound
	}

	related, err := scanRelations(ctx, tx,
		`SELECT from_id, from_pointer, rel_type, to_id, properties, content, updated_at
		 FROM relations WHERE from_id = $1 OR to_id = $1
		 ORDER BY from_id, from_pointer, rel_type, to_id`, id)
	if err != nil {
		return nil, err
	}
	if !cascade && len(related) > 0 {
		return nil, fmt.Errorf("%w: entity %s has %d relation(s)", store.ErrHasRelations, id, len(related))
	}

	if _, err := tx.Exec(ctx, `DELETE FROM relations WHERE from_id = $1 OR to_id = $1`, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM attachments WHERE entity_id = $1`, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM entities WHERE id = $1`, id); err != nil {
		return nil, err
	}

	// Build the events and NOTIFY for each inside the tx (atomic with the
	// deletes), then commit, then fan out to in-process subscribers.
	evs := make([]store.Event, 0, len(family)+len(related))
	for _, fe := range family {
		evs = append(evs, store.Event{
			Op: store.EventEntityDeleted, EntityType: fe.Type, EntityID: id, Pointer: fe.Pointer,
		})
	}
	for _, r := range related {
		evs = append(evs, store.Event{
			Op: store.EventRelationDeleted, RelationType: r.Type, From: r.From, To: r.To,
			Pointer: r.FromPointer,
		})
	}
	// Record a tombstone for each deletion in the same tx, so the durable
	// "what changed since cursor X" manifest can report removals that the live
	// rows no longer reflect (FEAT-NJ9FEN).
	if err := s.writeTombstonesForEvents(ctx, tx, evs); err != nil {
		return nil, err
	}
	for _, ev := range evs {
		s.notify(ctx, tx, ev)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// One notifyDelete for an N-state family: observers are bare-id
	// keyed (see notifyPut), so a single delete covers every face; the
	// N tombstones above serve the seq-keyed feed instead.
	s.notifyDelete(id)
	s.emitAll(evs)

	return &store.DeleteResult{DeletedEntities: family, DeletedRelations: related}, nil
}

// scanEntities drains a multi-row entity query.
func scanEntities(ctx context.Context, db DBTX, sql string, args ...any) ([]*entity.Entity, error) {
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
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
	return out, rows.Err()
}

// rekeyStateFamily re-keys every state row of oldID onto newID in place
// (TKT-DOFYR1): WHERE id = $1 sweeps the family, RETURNING hands back
// each renamed state, and search_text is recomputed PER STATE from that
// state's own content — one writer for the column (see
// entitySearchText); splicing the new ID over the old prefix in SQL
// risks a mixed-case prefix and assumes lower() preserves byte length,
// which is not true for all Unicode the store permits. renamed is the
// default face for observers, nil for a headless family.
func rekeyStateFamily(
	ctx context.Context, tx pgx.Tx, oldID, newID string,
) (states []*entity.Entity, renamed *entity.Entity, err error) {
	states, err = scanEntities(ctx, tx,
		`UPDATE entities SET id = $2, updated_at = now(), seq = nextval('rela_seq')
		 WHERE id = $1
		 RETURNING id, type, pointer, properties, content, updated_at`, oldID, newID)
	if err != nil {
		return nil, nil, err
	}
	for _, st := range states {
		if _, err = tx.Exec(ctx,
			`UPDATE entities SET search_text = $3 WHERE id = $1 AND pointer = $2`,
			newID, st.Pointer, entitySearchText(st)); err != nil {
			return nil, nil, err
		}
		if st.Pointer.IsDefault() {
			renamed = st
		}
	}
	return states, renamed, nil
}

// RenameEntity changes an entity's ID, rewriting every relation endpoint and
// re-keying attachments atomically. Returns store.ErrNotFound if oldID is
// absent, store.ErrConflict if newID exists.
func (s *Store) RenameEntity(ctx context.Context, oldID, newID string) (*store.RenameResult, error) {
	if err := validateID(newID); err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	var exists bool
	err = tx.QueryRow(ctx, `SELECT true FROM entities WHERE id = $1`, oldID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// lower(...) so a rename onto an existing entity's case-variant conflicts
	// (BUG-3RCWNS); `id <> $2` lets an entity change its OWN casing
	// (abc -> ABC), which is a legitimate rename and not a self-collision.
	// Matches the entities_id_lower_key index, so this uses it.
	err = tx.QueryRow(ctx,
		`SELECT true FROM entities WHERE lower(id) = lower($1) AND id <> $2`,
		newID, oldID).Scan(&exists)
	if err == nil {
		return nil, store.ErrConflict
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	renamedStates, renamed, err := rekeyStateFamily(ctx, tx, oldID, newID)
	if err != nil {
		return nil, err
	}
	if len(renamedStates) == 0 {
		// The family vanished between the existence probe and the
		// UPDATE (concurrent delete under READ COMMITTED): not-found,
		// never an index panic.
		return nil, store.ErrNotFound
	}
	newType := renamedStates[0].Type

	// Capture the relation triples that reference oldID BEFORE re-keying them.
	// A rename changes a relation's primary key (from_id/to_id), so to an
	// id-keyed sync client the OLD triple is removed and the NEW one is created.
	// We tombstone the old triples below so the manifest reports the removal —
	// otherwise the client keeps a ghost edge forever (FEAT-NJ9FEN).
	oldTriples, err := scanRelations(ctx, tx,
		`SELECT from_id, from_pointer, rel_type, to_id, properties, content, updated_at
		 FROM relations WHERE from_id = $1 OR to_id = $1
		 ORDER BY from_id, from_pointer, rel_type, to_id`, oldID)
	if err != nil {
		return nil, err
	}

	tag, err := tx.Exec(ctx,
		`UPDATE relations SET from_id = $2, seq = nextval('rela_seq') WHERE from_id = $1`, oldID, newID)
	if err != nil {
		return nil, err
	}
	updated := tag.RowsAffected()
	tag, err = tx.Exec(ctx,
		`UPDATE relations SET to_id = $2, seq = nextval('rela_seq') WHERE to_id = $1`, oldID, newID)
	if err != nil {
		return nil, err
	}
	updated += tag.RowsAffected()

	if _, err := tx.Exec(ctx,
		`UPDATE attachments SET entity_id = $2, seq = nextval('rela_seq') WHERE entity_id = $1`,
		oldID, newID); err != nil {
		return nil, err
	}

	// Tombstone the removed identities (the old entity id and each old relation
	// triple) in the same tx, so the durable manifest reports a rename as the
	// removal of oldID that it is, from an id-keyed client's view. The re-keyed
	// rows already carry the new ids with fresh seqs (the "create" half).
	for _, st := range renamedStates {
		if err := s.writeEntityTombstone(ctx, tx, oldID, st.Pointer, newType); err != nil {
			return nil, err
		}
	}
	for _, r := range oldTriples {
		if err := s.writeRelationTombstone(ctx, tx, r.From, r.FromPointer, r.Type, r.To); err != nil {
			return nil, err
		}
	}

	// A rename presents to other processes as "the new entity changed" (they
	// re-snapshot). NOTIFY inside the tx; commit; then fan out in-process —
	// one event per state, like every write (TKT-DOFYR1).
	evs := make([]store.Event, 0, len(renamedStates))
	for _, st := range renamedStates {
		evs = append(evs, store.Event{
			Op: store.EventEntityUpdated, EntityType: newType, EntityID: newID, Pointer: st.Pointer,
		})
	}
	for _, ev := range evs {
		s.notify(ctx, tx, ev)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Notify with the value captured in-tx (RETURNING) rather than re-reading:
	// a post-commit GetEntity could miss the put if the row is concurrently
	// deleted or the query transiently fails, leaving the search index stale.
	// A headless family (renamed == nil) has no default face for observers —
	// same skip as fs/mem (TKT-DOFYR1).
	s.notifyDelete(oldID)
	if renamed != nil {
		s.notifyPut(renamed) // renamed carries updated_at from the RETURNING clause
	}
	s.emitAll(evs)

	return &store.RenameResult{RelationsUpdated: int(updated)}, nil
}

// --- observers ---

// Observers receive DEFAULT-STATE writes only in Step 1 (TKT-DOFYR1):
// the search indexers key documents by bare id, so a state event would
// overwrite the default face in the index. Same skip as fsstore and
// memstore — three backends, one rule; Subscribe() events are NOT
// filtered and fire per state.
func (s *Store) notifyPut(e *entity.Entity) {
	if !e.Pointer.IsDefault() {
		return
	}
	if s.txPending != nil { // inside Tx: deliver after the outer commit (tx.go)
		s.txPending.add(func(p *Store) { p.notifyPut(e) })
		return
	}
	for _, o := range s.observers {
		_ = o.EntityPut(e)
	}
}

func (s *Store) notifyDelete(id string) {
	if s.txPending != nil { // inside Tx: deliver after the outer commit (tx.go)
		s.txPending.add(func(p *Store) { p.notifyDelete(id) })
		return
	}
	for _, o := range s.observers {
		_ = o.EntityDelete(id)
	}
}

// --- row scanning + helpers ---

// scanner abstracts pgx.Row and pgx.Rows for shared scan helpers.
type scanner interface {
	Scan(dest ...any) error
}

func scanEntity(row scanner) (*entity.Entity, error) {
	var (
		id, typ, content, ptr string
		props                 []byte
		updatedAt             time.Time
	)
	if err := row.Scan(&id, &typ, &ptr, &props, &content, &updatedAt); err != nil {
		return nil, err
	}
	e := entity.New(id, typ)
	e.Pointer = entity.Pointer(ptr)
	e.Content = content
	e.UpdatedAt = updatedAt
	var err error
	if e.Properties, err = unmarshalProps(props); err != nil {
		return nil, err
	}
	return e, nil
}

func scanEntityHeader(row scanner) (store.EntityHeader, error) {
	var (
		id, typ, ptr string
		props        []byte
		updatedAt    time.Time
	)
	if err := row.Scan(&id, &typ, &ptr, &props, &updatedAt); err != nil {
		return store.EntityHeader{}, err
	}
	h := store.EntityHeader{ID: id, Type: typ, Pointer: entity.Pointer(ptr), UpdatedAt: updatedAt}
	var err error
	if h.Properties, err = unmarshalProps(props); err != nil {
		return store.EntityHeader{}, err
	}
	return h, nil
}

// buildEntityListSQL builds the SELECT + WHERE + ORDER BY for entity
// listings. keysetAfter, when non-empty, resumes pagination after a
// cursor. Ordering is ascending (id, pointer): for the default-only
// zero-value query that is exactly the contract's historical
// ascending-id order (the pointer column is constant ”); under
// AllStates the states of an id sort immediately after its default row.
//
// That contiguity is a SHARED contract, not a pgstore detail: fs/mem
// match it via storeutil.CompareStateKeys, which orders their index by
// the same (bare id, pointer) tuple. It is load-bearing for world
// resolution, which buffers one family and decides at end-of-family
// (TKT-WAV8XP). Note fs/mem need an explicit comparator to get here —
// they key on the JOINED "id@pointer" string, and '@' (0x40) sorts after
// the digits (0x30-0x39), so plain string order puts PAGE-10's family
// inside PAGE-1's. Changing either side's ordering breaks the other.
func buildEntityListSQL(q store.EntityQuery, keysetAfter string) (sql string, args []any) {
	return buildEntitySelectSQL(q, keysetAfter, "id, type, pointer, properties, content, updated_at")
}

// buildEntityHeaderListSQL mirrors buildEntityListSQL WITHOUT the content
// column. Column order must stay in sync with scanEntityHeader.
func buildEntityHeaderListSQL(q store.EntityQuery, keysetAfter string) (sql string, args []any) {
	return buildEntitySelectSQL(q, keysetAfter, "id, type, pointer, properties, updated_at")
}

// buildEntitySelectSQL is the shared body of the two list builders: the
// same scope and ordering over a different column list.
//
// For the DEFAULT world it is the historical flat SELECT, allocating and
// costing exactly what it did before worlds existed — a project that
// never declares a pointer must pay nothing (store.WorldScope.IsDefaultWorld).
//
// For a real world it becomes DISTINCT ON (id) over the candidate rows,
// ordered by (id, rank), which picks each family's prime in one pass.
// Resolution cannot be a row predicate — see worldSQL — so the shape has
// to change, not just the WHERE clause.
//
// The keyset condition stays OUTSIDE the DISTINCT ON: it must page over
// PRIMES, not over candidate rows. Applying it inside would let a cursor
// land mid-family and resolve a prime from a partial view, which is the
// wrong-prime hazard storeutil.PaginateWorldPrimes exists to avoid.
func buildEntitySelectSQL(q store.EntityQuery, keysetAfter, columns string) (sql string, args []any) {
	if q.World.IsDefaultWorld() {
		where, wargs := entityWhere(q, keysetAfter)
		return `SELECT ` + columns + ` FROM entities` + where + ` ORDER BY id ASC, pointer ASC`, wargs
	}

	// ONE worldSQL call produces both expressions, so the coordinate
	// parameters are bound exactly once and the rank's placeholders are
	// the same ones the candidate predicate uses.
	rank, candidate := worldSQL(q.World, "", &args)
	scope := entityScopeWhere(q, candidate, &args)
	inner := `SELECT DISTINCT ON (id) ` + columns + ` FROM entities` + scope +
		` ORDER BY id ASC, (` + rank + `) ASC, pointer ASC`

	outer := `SELECT ` + columns + ` FROM (` + inner + `) p`
	if keysetAfter != "" {
		// Cursor semantics match the default-world path: an unparseable
		// cursor RESTARTS rather than comparing against garbage.
		if cursorID, cursorPtr, err := entity.ParseStateRef(keysetAfter); err == nil {
			args = append(args, cursorID)
			idArg := len(args)
			args = append(args, string(cursorPtr))
			outer += fmt.Sprintf(" WHERE (p.id, p.pointer) > ($%d, $%d)", idArg, len(args))
		}
	}
	return outer + ` ORDER BY id ASC, pointer ASC`, args
}

// entityScopeWhere builds the WHERE clause for a WORLD-scoped listing:
// the world's candidate predicate (already built, so its coordinate
// parameters are bound once and shared with the rank expression) plus
// the query's own type/IDs filters.
//
// It carries no keyset condition on purpose. Paging a world-scoped query
// must page over PRIMES, not candidate rows, so the cursor is applied
// OUTSIDE the DISTINCT ON — see buildEntitySelectSQL.
func entityScopeWhere(q store.EntityQuery, candidate string, args *[]any) string {
	conds := []string{candidate}
	if q.Type != "" {
		*args = append(*args, q.Type)
		conds = append(conds, fmt.Sprintf("type = $%d", len(*args)))
	}
	if len(q.IDs) > 0 {
		*args = append(*args, q.IDs)
		conds = append(conds, fmt.Sprintf("id = ANY($%d)", len(*args)))
	}
	return " WHERE " + strings.Join(conds, " AND ")
}

func entityWhere(q store.EntityQuery, keysetAfter string) (where string, args []any) {
	var conds []string
	// Default-world scope: the zero-value query returns default states
	// only — byte-identical behavior for pointerless projects. AllStates
	// is the raw storage-truth escape hatch (see store.EntityQuery).
	//
	// A non-default World does NOT come through here: it needs the
	// widened candidate predicate plus a rank, which only
	// buildEntitySelectSQL can pair up — see entityScopeWhere.
	if !q.AllStates {
		conds = append(conds, "pointer = ''")
	}
	if q.Type != "" {
		args = append(args, q.Type)
		conds = append(conds, fmt.Sprintf("type = $%d", len(args)))
	}
	if len(q.IDs) > 0 {
		args = append(args, q.IDs)
		conds = append(conds, fmt.Sprintf("id = ANY($%d)", len(args)))
	}
	if keysetAfter != "" {
		// The cursor encodes the state key ("id" or "id@pointer"); a
		// pre-upgrade cursor parses as a bare id with the zero pointer,
		// so it keeps resuming correctly. Row-wise comparison matches
		// the (id, pointer) ordering. An unparseable cursor genuinely
		// RESTARTS (the keyset condition is omitted) — comparing against
		// the garbage string would silently skip every row sorting below
		// it, which a paging caller cannot tell from end-of-results.
		if cursorID, cursorPtr, err := entity.ParseStateRef(keysetAfter); err == nil {
			args = append(args, cursorID)
			idArg := len(args)
			args = append(args, string(cursorPtr))
			conds = append(conds, fmt.Sprintf("(id, pointer) > ($%d, $%d)", idArg, len(args)))
		}
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func marshalProps(p map[string]any) ([]byte, error) {
	if len(p) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(p)
}

// unmarshalProps decodes a JSONB properties blob into a Go map, normalizing
// numbers so whole values are int (see normalizeJSONNumbers). An empty blob
// yields an empty (non-nil) map.
func unmarshalProps(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	if m == nil {
		return map[string]any{}, nil
	}
	return normalizeJSONMap(m), nil
}

// normalizeJSONNumbers walks a decoded JSON value (decoded with UseNumber) and
// converts json.Number to int when it has no fractional part, else float64.
// Without this, every numeric property round-trips as float64 — but callers
// (and the conformance suite) store and expect plain int for whole numbers,
// matching the in-memory backends. Strings, bools, and nested
// maps/slices are preserved structurally.
func normalizeJSONNumbers(v any) any {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return int(i)
		}
		if f, err := t.Float64(); err == nil {
			return f
		}
		return t.String()
	case map[string]any:
		return normalizeJSONMap(t)
	case []any:
		for i := range t {
			t[i] = normalizeJSONNumbers(t[i])
		}
		return t
	default:
		return v
	}
}

func normalizeJSONMap(m map[string]any) map[string]any {
	for k, v := range m {
		m[k] = normalizeJSONNumbers(v)
	}
	return m
}

// stringifyJSONValue renders a raw JSONB value the way memstore's
// fmt.Sprintf("%v", v) would, so PropertyValues output matches across backends.
// JSON strings render without quotes; numbers without scientific notation where
// possible; everything else falls back to its JSON text.
func stringifyJSONValue(raw []byte) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return strings.TrimSpace(string(raw))
	}
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		// Match fmt %v for whole numbers (e.g. 5 not 5e+00).
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return fmt.Sprintf("%v", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// entitySearchText builds the lowercased text the search Backend matches
// against. It mirrors search.MatchText's field selection exactly: entity ID,
// content, and STRING-valued properties only (non-string props are excluded).
func entitySearchText(e *entity.Entity) string {
	var b strings.Builder
	b.WriteString(strings.ToLower(e.ID))
	b.WriteByte('\n')
	b.WriteString(strings.ToLower(e.Content))
	for _, v := range e.Properties {
		if str, ok := v.(string); ok {
			b.WriteByte('\n')
			b.WriteString(strings.ToLower(str))
		}
	}
	return b.String()
}
