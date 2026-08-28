package pgstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

// UserStateStore is the PostgreSQL [userstate.Store]: next-action snoozes,
// mutes and cooldown records for the multi-process deployment.
//
// # Why this backend exists
//
// The filesystem backend keeps all of a deployment's state in one JSON
// document, which makes every write a read-modify-write of the whole thing.
// Within one process a mutex serializes that; across processes it is
// last-writer-wins, so two servers sharing a project directory silently lose
// each other's snoozes. Here each row is written independently, so concurrent
// writers need no coordination at all.
//
// # Not graph content
//
// These tables sit outside the entity/relation model on purpose: a snooze is
// a fact about one person's relationship to a suggestion, not about the
// entity. Keeping it out of the graph is what stops the audit log and the
// version-capture sweep filling with render-time noise. See
// migrations/0008_next_action_state.sql for the full rationale.
type UserStateStore struct {
	db DBTX
	// closed is set by Close. Reads and writes after that return ErrClosed
	// rather than hitting a pool the owner believes it has released.
	closed bool
}

// NewUserStateStore returns a PostgreSQL-backed [userstate.Store] over an
// injected pool.
//
// Takes a DBTX rather than a DSN for the same reason [New] does: appbuild
// builds ONE pool and shares it between the store, the search backend and
// this. It does not own the pool and does not close it — Close only marks
// this handle unusable.
func NewUserStateStore(db DBTX) (*UserStateStore, error) {
	if db == nil {
		return nil, errors.New("pgstore: NewUserStateStore: db must be non-nil")
	}
	return &UserStateStore{db: db}, nil
}

// compile-time check
var _ userstate.Store = (*UserStateStore)(nil)

func (s *UserStateStore) SnoozedUntil(
	ctx context.Context, key userstate.Key, now time.Time,
) (time.Time, bool, error) {
	if s.closed {
		return time.Time{}, false, userstate.ErrClosed
	}
	// Expiry is judged in the QUERY (`until > now`) rather than by comparing
	// after the read, so correctness never depends on Prune having run and a
	// stale row is simply invisible. `>` not `>=`: a snooze "until T" is over
	// at T, matching how a deadline reads to a human.
	const q = `
		SELECT until FROM next_action_snoozes
		WHERE user_id = $1 AND source = $2 AND entity_id = $3 AND variant = $4
		  AND until > $5`
	var until time.Time
	err := s.db.QueryRow(ctx, q, key.User, key.Source, key.EntityID, key.Variant, now).Scan(&until)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("pgstore: snooze lookup: %w", err)
	}
	return until, true, nil
}

func (s *UserStateStore) SetSnooze(ctx context.Context, key userstate.Key, until time.Time) error {
	if s.closed {
		return userstate.ErrClosed
	}
	// Upsert, not insert: re-snoozing REPLACES rather than stacking, and a
	// shorter re-snooze must win too — the user said "remind me sooner".
	const q = `
		INSERT INTO next_action_snoozes (user_id, source, entity_id, variant, until)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, source, entity_id, variant)
		DO UPDATE SET until = EXCLUDED.until`
	if _, err := s.db.Exec(ctx, q, key.User, key.Source, key.EntityID, key.Variant, until); err != nil {
		return fmt.Errorf("pgstore: set snooze: %w", err)
	}
	return nil
}

func (s *UserStateStore) Muted(ctx context.Context, user, source string) (bool, error) {
	if s.closed {
		return false, userstate.ErrClosed
	}
	const q = `SELECT EXISTS (SELECT 1 FROM next_action_mutes WHERE user_id = $1 AND source = $2)`
	var muted bool
	if err := s.db.QueryRow(ctx, q, user, source).Scan(&muted); err != nil {
		return false, fmt.Errorf("pgstore: mute lookup: %w", err)
	}
	return muted, nil
}

func (s *UserStateStore) SetMuted(ctx context.Context, user, source string, muted bool) error {
	if s.closed {
		return userstate.ErrClosed
	}
	if !muted {
		const del = `DELETE FROM next_action_mutes WHERE user_id = $1 AND source = $2`
		if _, err := s.db.Exec(ctx, del, user, source); err != nil {
			return fmt.Errorf("pgstore: unmute: %w", err)
		}
		return nil
	}
	// DO NOTHING keeps muting idempotent: the mute list is rendered to the
	// user as "what have I turned off?", so a duplicate row would show twice.
	const ins = `
		INSERT INTO next_action_mutes (user_id, source) VALUES ($1, $2)
		ON CONFLICT (user_id, source) DO NOTHING`
	if _, err := s.db.Exec(ctx, ins, user, source); err != nil {
		return fmt.Errorf("pgstore: mute: %w", err)
	}
	return nil
}

func (s *UserStateStore) MutedSources(ctx context.Context, user string) ([]string, error) {
	if s.closed {
		return nil, userstate.ErrClosed
	}
	const q = `SELECT source FROM next_action_mutes WHERE user_id = $1 ORDER BY source`
	rows, err := s.db.Query(ctx, q, user)
	if err != nil {
		return nil, fmt.Errorf("pgstore: muted sources: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var src string
		if err := rows.Scan(&src); err != nil {
			return nil, fmt.Errorf("pgstore: muted sources scan: %w", err)
		}
		out = append(out, src)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgstore: muted sources: %w", err)
	}
	return out, nil
}

func (s *UserStateStore) LastShown(ctx context.Context, key userstate.Key) (time.Time, bool, error) {
	if s.closed {
		return time.Time{}, false, userstate.ErrClosed
	}
	const q = `
		SELECT shown_at FROM next_action_shown
		WHERE user_id = $1 AND source = $2 AND entity_id = $3 AND variant = $4`
	var at time.Time
	err := s.db.QueryRow(ctx, q, key.User, key.Source, key.EntityID, key.Variant).Scan(&at)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("pgstore: last-shown lookup: %w", err)
	}
	return at, true, nil
}

func (s *UserStateStore) MarkShown(ctx context.Context, key userstate.Key, at time.Time) error {
	if s.closed {
		return userstate.ErrClosed
	}
	const q = `
		INSERT INTO next_action_shown (user_id, source, entity_id, variant, shown_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, source, entity_id, variant)
		DO UPDATE SET shown_at = EXCLUDED.shown_at`
	if _, err := s.db.Exec(ctx, q, key.User, key.Source, key.EntityID, key.Variant, at); err != nil {
		return fmt.Errorf("pgstore: mark shown: %w", err)
	}
	return nil
}

func (s *UserStateStore) Prune(
	ctx context.Context, now time.Time, keepShown time.Duration,
) (int, error) {
	if s.closed {
		return 0, userstate.ErrClosed
	}
	// Two statements rather than one CTE: they touch different tables with no
	// shared predicate, so combining them would only obscure which delete
	// contributed what. Mutes are deliberately untouched — they are a standing
	// choice with no expiry, and silently un-muting a source would surface as
	// suggestions the user switched off coming back.
	const delSnoozes = `DELETE FROM next_action_snoozes WHERE until <= $1`
	tag, err := s.db.Exec(ctx, delSnoozes, now)
	if err != nil {
		return 0, fmt.Errorf("pgstore: prune snoozes: %w", err)
	}
	removed := int(tag.RowsAffected())

	const delShown = `DELETE FROM next_action_shown WHERE shown_at < $1`
	tag, err = s.db.Exec(ctx, delShown, now.Add(-keepShown))
	if err != nil {
		return removed, fmt.Errorf("pgstore: prune shown: %w", err)
	}
	return removed + int(tag.RowsAffected()), nil
}

// Close marks this handle unusable. It does NOT close the pool: appbuild owns
// that and shares it with the store and the search backend, so closing it here
// would tear down two unrelated subsystems.
func (s *UserStateStore) Close() error {
	s.closed = true
	return nil
}

// UserState returns a next-action state backend sharing this store's
// connection handle.
//
// Mirrors [Store.VersionStore]: the composition root calls this in the
// postgres recipe to obtain a service it injects, and it reads the same pool
// the store queries, so a second pool is never opened for three small tables.
// Returns a fresh lightweight wrapper each call. The returned store does not
// own the pool — see [UserStateStore.Close].
func (s *Store) UserState() (userstate.Store, error) {
	return NewUserStateStore(s.db)
}
