// Package pgpiles is the PostgreSQL-backed [piles.Store].
//
// It exists because the KV backend rewrites one document per change, so
// several rela-server processes against one database would lose each other's
// writes. Here a pile is a row and an item is a row (migration 0021), in the
// tenant's schema like every other table.
//
// # Relationship to the store
//
// This package does not depend on internal/store; a pile is not graph
// content. It shares the store's connection pool, injected by the composition
// root, as pgcomments does.
//
// # Concurrency
//
// Create and add run in one transaction that first takes a per-owner advisory
// lock, so the pile cap and the eviction see a serialized view of the owner's
// piles. The lock is scoped by schema as well as owner, because advisory locks
// are database-global and two tenants must not serialize behind each other.
package pgpiles

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/piles"
)

// DBTX is the database handle this store runs on: a *pgxpool.Pool in
// production. Restated here rather than imported from pgstore, because this
// package must not depend on the store layer.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Store is the PostgreSQL piles backend.
type Store struct {
	db DBTX
}

var _ piles.Store = (*Store)(nil)

// New constructs a Store over db.
//
// Nil: rejected, so a wiring mistake fails at construction rather than at the
// first pile someone creates.
func New(db DBTX) (*Store, error) {
	if db == nil {
		return nil, errors.New("pgpiles: New requires a database handle")
	}
	return &Store{db: db}, nil
}

// nameIndex is the unique index that refuses a duplicate name.
const nameIndex = "piles_owner_name_idx"

// idKey is the primary key that refuses a duplicate pile id.
const idKey = "piles_pkey"

const pileColumns = `id, owner, name, icon, created_at, updated_at`

// lockOwner serializes the owner's creates and adds for the rest of tx.
func lockOwner(ctx context.Context, tx pgx.Tx, owner string) error {
	_, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtext('piles:' || current_schema()), hashtext($1))`, owner)
	if err != nil {
		return fmt.Errorf("pgpiles: lock owner: %w", err)
	}
	return nil
}

// isNameClash reports whether err is the duplicate-name refusal.
func isNameClash(err error) bool { return isUniqueViolation(err, nameIndex) }

// isUniqueViolation reports whether err is a unique violation of constraint.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}

// ListPiles returns the owner's piles with their items, oldest pile first.
func (s *Store) ListPiles(ctx context.Context, owner string) ([]piles.Pile, error) {
	rows, err := s.db.Query(ctx, `
		SELECT `+pileColumns+` FROM piles WHERE owner = $1 ORDER BY created_at, id`, owner)
	if err != nil {
		return nil, fmt.Errorf("pgpiles: list piles: %w", err)
	}
	out, err := pgx.CollectRows(rows, scanPile)
	if err != nil {
		return nil, fmt.Errorf("pgpiles: list piles: %w", err)
	}
	if len(out) == 0 {
		return []piles.Pile{}, nil
	}

	// One query for every pile's items, in stack order.
	itemRows, err := s.db.Query(ctx, `
		SELECT i.pile_id, i.entity_id, i.face, i.added_at
		FROM pile_items i JOIN piles p ON p.id = i.pile_id
		WHERE p.owner = $1
		ORDER BY i.pile_id, i.seq DESC`, owner)
	if err != nil {
		return nil, fmt.Errorf("pgpiles: list items: %w", err)
	}
	defer itemRows.Close()
	byPile := make(map[string][]piles.Item, len(out))
	for itemRows.Next() {
		var pileID string
		it, err := scanItem(itemRows, &pileID)
		if err != nil {
			return nil, fmt.Errorf("pgpiles: list items: %w", err)
		}
		byPile[pileID] = append(byPile[pileID], it)
	}
	if err := itemRows.Err(); err != nil {
		return nil, fmt.Errorf("pgpiles: list items: %w", err)
	}
	for i := range out {
		if items, ok := byPile[out[i].ID]; ok {
			out[i].Items = items
		}
	}
	return out, nil
}

// GetPile returns one of the owner's piles with its items.
func (s *Store) GetPile(ctx context.Context, owner, id string) (piles.Pile, error) {
	return s.onePile(ctx, `SELECT `+pileColumns+` FROM piles WHERE owner = $1 AND id = $2`, owner, id)
}

// PileByName finds the owner's pile by case-insensitive name.
func (s *Store) PileByName(ctx context.Context, owner, name string) (piles.Pile, error) {
	return s.onePile(ctx,
		`SELECT `+pileColumns+` FROM piles WHERE owner = $1 AND lower(name) = lower($2)`, owner, name)
}

func (s *Store) onePile(ctx context.Context, query string, args ...any) (piles.Pile, error) {
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return piles.Pile{}, fmt.Errorf("pgpiles: get pile: %w", err)
	}
	p, err := pgx.CollectExactlyOneRow(rows, scanPile)
	if errors.Is(err, pgx.ErrNoRows) {
		return piles.Pile{}, piles.ErrNotFound
	}
	if err != nil {
		return piles.Pile{}, fmt.Errorf("pgpiles: get pile: %w", err)
	}
	items, err := s.items(ctx, p.ID)
	if err != nil {
		return piles.Pile{}, err
	}
	p.Items = items
	return p, nil
}

// items returns one pile's items, newest first.
func (s *Store) items(ctx context.Context, pileID string) ([]piles.Item, error) {
	rows, err := s.db.Query(ctx, `
		SELECT pile_id, entity_id, face, added_at FROM pile_items
		WHERE pile_id = $1 ORDER BY seq DESC`, pileID)
	if err != nil {
		return nil, fmt.Errorf("pgpiles: read items: %w", err)
	}
	defer rows.Close()
	out := []piles.Item{}
	for rows.Next() {
		var ignored string
		it, err := scanItem(rows, &ignored)
		if err != nil {
			return nil, fmt.Errorf("pgpiles: read items: %w", err)
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgpiles: read items: %w", err)
	}
	return out, nil
}

func scanPile(row pgx.CollectableRow) (piles.Pile, error) {
	var p piles.Pile
	err := row.Scan(&p.ID, &p.Owner, &p.Name, &p.Icon, &p.Created, &p.Updated)
	p.Created, p.Updated = p.Created.UTC(), p.Updated.UTC()
	p.Items = []piles.Item{}
	return p, err
}

func scanItem(row pgx.Row, pileID *string) (piles.Item, error) {
	var id, face string
	var added time.Time
	if err := row.Scan(pileID, &id, &face, &added); err != nil {
		return piles.Item{}, err
	}
	return piles.Item{Ref: entity.Ref{ID: id, Face: entity.Face(face)}, Added: added.UTC()}, nil
}

// CreatePile stores p with refs as its first items, under the owner lock.
func (s *Store) CreatePile(ctx context.Context, p piles.Pile, refs []entity.Ref, maxPiles, maxItems int) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := lockOwner(ctx, tx, p.Owner); err != nil {
			return err
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM piles WHERE owner = $1`, p.Owner).Scan(&count); err != nil {
			return fmt.Errorf("pgpiles: count piles: %w", err)
		}
		if count >= maxPiles {
			return piles.ErrLimit
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO piles (`+pileColumns+`) VALUES ($1, $2, $3, $4, $5, $6)`,
			p.ID, p.Owner, p.Name, p.Icon, p.Created, p.Updated)
		if isNameClash(err) {
			return piles.ErrNameTaken
		}
		if isUniqueViolation(err, idKey) {
			return piles.ErrIDTaken
		}
		if err != nil {
			return fmt.Errorf("pgpiles: create pile: %w", err)
		}
		_, err = insertItems(ctx, tx, p.ID, firstUnique(refs, maxItems), p.Created)
		return err
	})
}

// UpdatePile renames or re-icons a pile.
func (s *Store) UpdatePile(ctx context.Context, owner, id, name, icon string, now time.Time) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE piles SET name = $3, icon = $4, updated_at = $5 WHERE owner = $1 AND id = $2`,
		owner, id, name, icon, now)
	if isNameClash(err) {
		return piles.ErrNameTaken
	}
	if err != nil {
		return fmt.Errorf("pgpiles: update pile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return piles.ErrNotFound
	}
	return nil
}

// DeletePile removes a pile; its items go with it through ON DELETE CASCADE.
func (s *Store) DeletePile(ctx context.Context, owner, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM piles WHERE owner = $1 AND id = $2`, owner, id)
	if err != nil {
		return fmt.Errorf("pgpiles: delete pile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return piles.ErrNotFound
	}
	return nil
}

// AddItems puts refs on top of the pile in one transaction under the owner
// lock: past maxItems it evicts the oldest or, with [piles.KeepExisting],
// inserts only what fits.
func (s *Store) AddItems(
	ctx context.Context, owner, id string, refs []entity.Ref, now time.Time, maxItems int, overflow piles.Overflow,
) (int, error) {
	added := 0
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if err := lockOwner(ctx, tx, owner); err != nil {
			return err
		}
		if err := ownedBy(ctx, tx, owner, id); err != nil {
			return err
		}
		fresh := firstUnique(refs, maxItems)
		if overflow != piles.EvictOldest {
			var err error
			if fresh, err = fitting(ctx, tx, id, fresh, maxItems); err != nil {
				return err
			}
		}
		n, err := insertItems(ctx, tx, id, fresh, now)
		if err != nil {
			return err
		}
		added = n
		if n == 0 {
			return nil
		}
		_, err = tx.Exec(ctx, `
			DELETE FROM pile_items WHERE pile_id = $1 AND seq NOT IN (
				SELECT seq FROM pile_items WHERE pile_id = $1 ORDER BY seq DESC LIMIT $2)`, id, maxItems)
		if err != nil {
			return fmt.Errorf("pgpiles: evict: %w", err)
		}
		return nil
	})
	return added, err
}

// fitting returns the refs not yet on the pile, cut to the room left under
// maxItems. Every add to a pile holds the owner lock, so the count cannot
// change before the insert.
func fitting(ctx context.Context, tx pgx.Tx, pileID string, refs []entity.Ref, maxItems int) ([]entity.Ref, error) {
	rows, err := tx.Query(ctx, `SELECT entity_id, face FROM pile_items WHERE pile_id = $1`, pileID)
	if err != nil {
		return nil, fmt.Errorf("pgpiles: read items: %w", err)
	}
	present, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (entity.Ref, error) {
		var id, face string
		scanErr := row.Scan(&id, &face)
		return entity.Ref{ID: id, Face: entity.Face(face)}, scanErr
	})
	if err != nil {
		return nil, fmt.Errorf("pgpiles: read items: %w", err)
	}
	room := max(maxItems-len(present), 0)
	fresh := make([]entity.Ref, 0, min(len(refs), room))
	for _, r := range refs {
		if len(fresh) == room {
			break
		}
		if !slices.Contains(present, r) {
			fresh = append(fresh, r)
		}
	}
	return fresh, nil
}

// insertItems adds refs as the newest items, the first ref newest, and
// returns how many were new. Rows go in last ref first, one statement each in
// a single batch, so the sequence hands the first ref the highest seq; a
// single INSERT ... SELECT would leave that order to the planner.
func insertItems(ctx context.Context, tx pgx.Tx, pileID string, refs []entity.Ref, added time.Time) (int, error) {
	if len(refs) == 0 {
		return 0, nil
	}
	batch := &pgx.Batch{}
	for _, r := range slices.Backward(refs) {
		batch.Queue(`
			INSERT INTO pile_items (pile_id, entity_id, face, added_at) VALUES ($1, $2, $3, $4)
			ON CONFLICT DO NOTHING`, pileID, r.ID, string(r.Face), added)
	}
	results := tx.SendBatch(ctx, batch)
	inserted := 0
	for range refs {
		tag, err := results.Exec()
		if err != nil {
			_ = results.Close()
			return 0, fmt.Errorf("pgpiles: add items: %w", err)
		}
		inserted += int(tag.RowsAffected())
	}
	if err := results.Close(); err != nil {
		return 0, fmt.Errorf("pgpiles: add items: %w", err)
	}
	return inserted, nil
}

// RemoveItems drops refs from the pile, ignoring refs not on it.
func (s *Store) RemoveItems(ctx context.Context, owner, id string, refs []entity.Ref) error {
	if err := ownedBy(ctx, s.db, owner, id); err != nil {
		return err
	}
	if len(refs) == 0 {
		return nil
	}
	ids, faces := split(refs)
	_, err := s.db.Exec(ctx, `
		DELETE FROM pile_items i
		USING unnest($2::text[], $3::text[]) AS r(entity_id, face)
		WHERE i.pile_id = $1 AND i.entity_id = r.entity_id AND i.face = r.face`, id, ids, faces)
	if err != nil {
		return fmt.Errorf("pgpiles: remove items: %w", err)
	}
	return nil
}

// RenameEntity rewrites items naming oldID, in one transaction. Where the
// new ref is already on a pile, the existing item keeps its place and the
// renamed one is dropped.
//
// It copies each item to the new id, keeping its seq and so its place, and
// then deletes the old rows. ON CONFLICT DO NOTHING makes the copy skip a
// (pile, new id, face) that exists, including one a concurrent add commits
// mid-statement; an UPDATE of the key would fail with a unique violation
// there.
func (s *Store) RenameEntity(ctx context.Context, oldID, newID string) error {
	if oldID == newID {
		return nil
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO pile_items (pile_id, entity_id, face, seq, added_at)
			SELECT pile_id, $2, face, seq, added_at FROM pile_items WHERE entity_id = $1
			ON CONFLICT DO NOTHING`, oldID, newID); err != nil {
			return fmt.Errorf("pgpiles: rename items: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM pile_items WHERE entity_id = $1`, oldID); err != nil {
			return fmt.Errorf("pgpiles: rename drop old items: %w", err)
		}
		return nil
	})
}

// RenameOwner moves oldOwner's piles to newOwner, in one transaction under
// both owner locks.
//
// It runs for a renamed person, whose new id is fresh, so newOwner normally
// holds no piles. If it does, they can only be a leftover the delete hook
// missed: where one clashes by name with a moved pile, the moved pile wins and
// the leftover is dropped, so the unique name index still holds.
func (s *Store) RenameOwner(ctx context.Context, oldOwner, newOwner string) error {
	if oldOwner == newOwner {
		return nil
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		// A fixed order, so two concurrent renames cannot deadlock.
		if err := lockOwner(ctx, tx, min(oldOwner, newOwner)); err != nil {
			return err
		}
		if err := lockOwner(ctx, tx, max(oldOwner, newOwner)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM piles t WHERE t.owner = $2 AND EXISTS (
				SELECT 1 FROM piles o WHERE o.owner = $1 AND lower(o.name) = lower(t.name))`,
			oldOwner, newOwner); err != nil {
			return fmt.Errorf("pgpiles: rename owner drop clashing piles: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE piles SET owner = $2 WHERE owner = $1`, oldOwner, newOwner); err != nil {
			return fmt.Errorf("pgpiles: rename owner: %w", err)
		}
		return nil
	})
}

// DeleteEntity drops items naming id.
func (s *Store) DeleteEntity(ctx context.Context, id string) error {
	if _, err := s.db.Exec(ctx, `DELETE FROM pile_items WHERE entity_id = $1`, id); err != nil {
		return fmt.Errorf("pgpiles: delete items: %w", err)
	}
	return nil
}

// DeleteOwner drops every pile owned by owner; items go with them through ON
// DELETE CASCADE.
func (s *Store) DeleteOwner(ctx context.Context, owner string) error {
	if _, err := s.db.Exec(ctx, `DELETE FROM piles WHERE owner = $1`, owner); err != nil {
		return fmt.Errorf("pgpiles: delete owned piles: %w", err)
	}
	return nil
}

// DeleteFace drops items naming exactly that face of id.
func (s *Store) DeleteFace(ctx context.Context, id string, face entity.Face) error {
	_, err := s.db.Exec(ctx, `DELETE FROM pile_items WHERE entity_id = $1 AND face = $2`, id, string(face))
	if err != nil {
		return fmt.Errorf("pgpiles: delete face: %w", err)
	}
	return nil
}

// querier is the read half of [DBTX], met by both the pool and a pgx.Tx.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ownedBy returns [piles.ErrNotFound] unless owner holds pile id.
func ownedBy(ctx context.Context, q querier, owner, id string) error {
	var one int
	err := q.QueryRow(ctx, `SELECT 1 FROM piles WHERE owner = $1 AND id = $2`, owner, id).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return piles.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("pgpiles: find pile: %w", err)
	}
	return nil
}

// inTx runs fn in a transaction, committing when it returns nil.
func (s *Store) inTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pgpiles: begin: %w", err)
	}
	// Rolls back unless Commit already succeeded, in which case it is a no-op.
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("pgpiles: commit: %w", err)
	}
	return nil
}

func split(refs []entity.Ref) (ids, faces []string) {
	ids, faces = make([]string, len(refs)), make([]string, len(refs))
	for i, r := range refs {
		ids[i], faces[i] = r.ID, string(r.Face)
	}
	return ids, faces
}

// firstUnique collapses duplicate refs, keeping the first, and keeps at most
// limit of them. Collapsing before the insert matters: inserted last ref
// first, a later duplicate would otherwise claim the slot and the lower seq.
func firstUnique(refs []entity.Ref, limit int) []entity.Ref {
	seen := make(map[entity.Ref]bool, len(refs))
	out := make([]entity.Ref, 0, min(len(refs), limit))
	for _, r := range refs {
		if len(out) == limit {
			break
		}
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}
