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

// --- EntityReader ---

// GetEntity returns a single entity by ID, or store.ErrNotFound.
func (s *Store) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	const q = `SELECT id, type, properties, content, updated_at FROM entities WHERE id = $1`
	e, err := scanEntity(s.db.QueryRow(ctx, q, id))
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

// ListEntitiesPage returns a page of entities. A keyset cursor on id keeps
// pages stable; see store.ListEntitiesPage for the contract.
func (s *Store) ListEntitiesPage(ctx context.Context, q store.EntityQuery) (store.Page[*entity.Entity], error) {
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
		next = storeutil.EncodeCursor(last.ID)
	}
	return store.Page[*entity.Entity]{Items: items, NextCursor: next}, nil
}

// CountEntities counts entities matching q.
func (s *Store) CountEntities(ctx context.Context, q store.EntityQuery) (int, error) {
	where, args := entityWhere(q, "")
	sql := "SELECT count(*) FROM entities" + where
	var n int
	if err := s.db.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// HighestID returns the highest numeric suffix among IDs of the form
// "<prefix>-<n>", or 0. Matching memstore/fsstore: non-numeric suffixes are
// skipped and gaps are ignored. The parse is done in Go (not SQL) to keep the
// Sscanf("%d") semantics identical across backends.
func (s *Store) HighestID(ctx context.Context, prefix string) (int, error) {
	pfx := prefix + "-"
	const q = `SELECT id FROM entities WHERE id LIKE $1`
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
	const q = `SELECT properties -> $1 AS v FROM entities WHERE properties ? $1`
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

	const q = `
		INSERT INTO entities (id, type, properties, content, search_text, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (id) DO NOTHING
		RETURNING updated_at`
	var updatedAt time.Time
	err = tx.QueryRow(ctx, q, e.ID, e.Type, props, e.Content, entitySearchText(e)).Scan(&updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ErrConflict
	}
	if err != nil {
		return err
	}

	ev := store.Event{Op: store.EventEntityCreated, EntityType: e.Type, EntityID: e.ID}
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

	const q = `
		UPDATE entities
		SET type = $2, properties = $3, content = $4, search_text = $5,
		    updated_at = now(), seq = nextval('rela_seq')
		WHERE id = $1
		RETURNING updated_at`
	var updatedAt time.Time
	err = tx.QueryRow(ctx, q, e.ID, e.Type, props, e.Content, entitySearchText(e)).Scan(&updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ErrNotFound
	}
	if err != nil {
		return err
	}

	ev := store.Event{Op: store.EventEntityUpdated, EntityType: e.Type, EntityID: e.ID}
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

	e, err := scanEntity(tx.QueryRow(ctx,
		`SELECT id, type, properties, content, updated_at FROM entities WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	related, err := scanRelations(ctx, tx,
		`SELECT from_id, rel_type, to_id, properties, content, updated_at
		 FROM relations WHERE from_id = $1 OR to_id = $1
		 ORDER BY from_id, rel_type, to_id`, id)
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
	evs := []store.Event{{Op: store.EventEntityDeleted, EntityType: e.Type, EntityID: id}}
	for _, r := range related {
		evs = append(evs, store.Event{
			Op: store.EventRelationDeleted, RelationType: r.Type, From: r.From, To: r.To,
		})
	}
	for _, ev := range evs {
		s.notify(ctx, tx, ev)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	s.notifyDelete(id)
	s.emitAll(evs)

	return &store.DeleteResult{DeletedEntities: []*entity.Entity{e}, DeletedRelations: related}, nil
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
	err = tx.QueryRow(ctx, `SELECT true FROM entities WHERE id = $1`, newID).Scan(&exists)
	if err == nil {
		return nil, store.ErrConflict
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// Update the ID and recompute search_text from the row itself. search_text
	// is the lowercased "id\ncontent\n<string props>" the search backend matches
	// against (see entitySearchText); only the ID part changes on rename.
	// Recomputing via entitySearchText keeps a SINGLE writer of that column —
	// splicing the new ID over the old prefix in SQL risks a mixed-case prefix
	// (renamed entity unfindable by its new ID) and assumes lower() preserves
	// byte length, which is not true for all Unicode the store permits.
	renamed, err := scanEntity(tx.QueryRow(ctx,
		`UPDATE entities SET id = $2, updated_at = now(), seq = nextval('rela_seq')
		 WHERE id = $1
		 RETURNING id, type, properties, content, updated_at`, oldID, newID))
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx,
		`UPDATE entities SET search_text = $2 WHERE id = $1`,
		newID, entitySearchText(renamed)); err != nil {
		return nil, err
	}
	newType := renamed.Type

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

	// A rename presents to other processes as "the new entity changed" (they
	// re-snapshot). NOTIFY inside the tx; commit; then fan out in-process.
	ev := store.Event{Op: store.EventEntityUpdated, EntityType: newType, EntityID: newID}
	s.notify(ctx, tx, ev)
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Notify with the value captured in-tx (RETURNING) rather than re-reading:
	// a post-commit GetEntity could miss the put if the row is concurrently
	// deleted or the query transiently fails, leaving the search index stale.
	s.notifyDelete(oldID)
	s.notifyPut(renamed) // renamed carries updated_at from the RETURNING clause
	s.emit(ev)

	return &store.RenameResult{RelationsUpdated: int(updated)}, nil
}

// --- observers ---

func (s *Store) notifyPut(e *entity.Entity) {
	for _, o := range s.observers {
		_ = o.EntityPut(e)
	}
}

func (s *Store) notifyDelete(id string) {
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
		id, typ, content string
		props            []byte
		updatedAt        time.Time
	)
	if err := row.Scan(&id, &typ, &props, &content, &updatedAt); err != nil {
		return nil, err
	}
	e := entity.New(id, typ)
	e.Content = content
	e.UpdatedAt = updatedAt
	var err error
	if e.Properties, err = unmarshalProps(props); err != nil {
		return nil, err
	}
	return e, nil
}

// buildEntityListSQL builds the SELECT + WHERE + ORDER BY for entity listings.
// keysetAfter, when non-empty, adds "id > $n" so pagination resumes after a
// cursor. Ordering is ascending by id (the contract's default stable order).
func buildEntityListSQL(q store.EntityQuery, keysetAfter string) (sql string, args []any) {
	where, args := entityWhere(q, keysetAfter)
	sql = `SELECT id, type, properties, content, updated_at FROM entities` + where + ` ORDER BY id ASC`
	return sql, args
}

func entityWhere(q store.EntityQuery, keysetAfter string) (where string, args []any) {
	var conds []string
	if q.Type != "" {
		args = append(args, q.Type)
		conds = append(conds, fmt.Sprintf("type = $%d", len(args)))
	}
	if len(q.IDs) > 0 {
		args = append(args, q.IDs)
		conds = append(conds, fmt.Sprintf("id = ANY($%d)", len(args)))
	}
	if keysetAfter != "" {
		args = append(args, keysetAfter)
		conds = append(conds, fmt.Sprintf("id > $%d", len(args)))
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func marshalProps(p map[string]interface{}) ([]byte, error) {
	if len(p) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(p)
}

// unmarshalProps decodes a JSONB properties blob into a Go map, normalizing
// numbers so whole values are int (see normalizeJSONNumbers). An empty blob
// yields an empty (non-nil) map.
func unmarshalProps(raw []byte) (map[string]interface{}, error) {
	if len(raw) == 0 {
		return map[string]interface{}{}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var m map[string]interface{}
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	if m == nil {
		return map[string]interface{}{}, nil
	}
	return normalizeJSONMap(m), nil
}

// normalizeJSONNumbers walks a decoded JSON value (decoded with UseNumber) and
// converts json.Number to int when it has no fractional part, else float64.
// Without this, every numeric property round-trips as float64 — but callers
// (and the conformance suite) store and expect plain int for whole numbers,
// matching the in-memory backends. Strings, bools, and nested
// maps/slices are preserved structurally.
func normalizeJSONNumbers(v interface{}) interface{} {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return int(i)
		}
		if f, err := t.Float64(); err == nil {
			return f
		}
		return t.String()
	case map[string]interface{}:
		return normalizeJSONMap(t)
	case []interface{}:
		for i := range t {
			t[i] = normalizeJSONNumbers(t[i])
		}
		return t
	default:
		return v
	}
}

func normalizeJSONMap(m map[string]interface{}) map[string]interface{} {
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
	var v interface{}
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
