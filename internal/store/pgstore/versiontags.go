package pgstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// Version tags (TKT-VO6VG9). The contract is on store.VersionTagger; this file
// is the PostgreSQL mechanism.
//
// Every write runs in one transaction on one acquired connection that holds
// the session advisory lock on sweepAdvisoryLockKey, the lock the sweep and
// purge take. So a tag cannot race a sweep capture (the capture-now would
// duplicate it) or a purge (which could delete the row being tagged). Unlike
// the sweep, a tag write WAITS for the lock: it is a caller's request, not a
// periodic job that can try again next tick. The wait is bounded by
// lock_timeout and by ctx.

// versionLockTimeout bounds how long a tag write or an entity purge waits for
// the version lock. A sweep tick over a large backlog can hold it for a while;
// past this, the caller gets an error rather than a hung request.
const versionLockTimeout = "30s"

// versionTagger is the [store.VersionTagger] built by
// [VersionStore.VersionTagger].
type versionTagger struct {
	v          *VersionStore
	projection store.ProjectionProvider
}

// VersionTagger implements [store.VersionTaggerProvider]. The projection
// stamps a version that tagging the current state captures.
//
// Nil: projection is rejected, since a capture without one could not be
// rendered.
func (v *VersionStore) VersionTagger(projection store.ProjectionProvider) (store.VersionTagger, error) {
	if projection == nil {
		return nil, errors.New("pgstore: version tagger needs a projection provider")
	}
	if _, ok := v.db.(*pgxpool.Pool); !ok {
		return nil, errors.New("pgstore: version tagger requires a *pgxpool.Pool (session-scoped advisory lock)")
	}
	return &versionTagger{v: v, projection: projection}, nil
}

// withVersionLock runs fn in one transaction on one acquired connection that
// holds the version lock, waiting for it up to versionLockTimeout.
//
// The lock is released AFTER the transaction ends, and that order matters: a
// session-level advisory lock survives COMMIT and ROLLBACK, and an unlock
// issued inside an aborted transaction would itself fail and leave the lock
// riding the pooled connection.
func (v *VersionStore) withVersionLock(ctx context.Context, fn func(tx pgx.Tx) error) error {
	pool, ok := v.db.(*pgxpool.Pool)
	if !ok {
		return errors.New("pgstore: the version lock requires a *pgxpool.Pool")
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	state, err := lockedTx(ctx, conn, fn)
	switch state {
	case lockNotTaken:
	case lockHeld:
		advisoryUnlock(context.WithoutCancel(ctx), conn, sweepAdvisoryLockKey)
	case lockUnknown:
		// The lock call failed, and a failure seen by the client (a
		// cancelled ctx, a broken read) does not prove the server did not
		// grant it. Closing the connection ends its session and so releases
		// any lock it holds; Release then discards the closed connection
		// instead of pooling it with a lock riding on it.
		_ = conn.Conn().Close(context.WithoutCancel(ctx))
	}
	return err
}

// lockState is what lockedTx knows about the session lock when it returns.
type lockState int

const (
	lockNotTaken lockState = iota // the lock was never requested
	lockHeld                      // the lock was granted
	lockUnknown                   // the request failed; it may have been granted
)

// lockedTx is withVersionLock's transaction: it begins on conn, waits for the
// version lock, runs fn and commits. The state tells the caller whether it
// must release the session lock after the transaction ends.
func lockedTx(ctx context.Context, conn *pgxpool.Conn, fn func(tx pgx.Tx) error) (lockState, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return lockNotTaken, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op
	// SET LOCAL ends with the transaction, so the timeout cannot leak into
	// the next user of this pooled connection.
	if _, err = tx.Exec(ctx, "SET LOCAL lock_timeout = '"+versionLockTimeout+"'"); err != nil {
		return lockNotTaken, err
	}
	// Two-key form, schema-scoped: must match tryAdvisoryLock (sweep.go) or
	// the sweep and this would stop excluding each other.
	if _, err = tx.Exec(ctx,
		`SELECT pg_advisory_lock($1::int, hashtext(current_schema()))`, sweepAdvisoryLockKey); err != nil {
		return lockUnknown, fmt.Errorf("pgstore: wait for the version lock: %w", err)
	}
	if fnErr := fn(tx); fnErr != nil {
		return lockHeld, fnErr
	}
	return lockHeld, tx.Commit(ctx)
}

// TagCurrent implements [store.VersionTagger].
func (t *versionTagger) TagCurrent(ctx context.Context, req store.TagRequest) (store.VersionMeta, error) {
	if err := store.ValidateTagRequest(req, true); err != nil {
		return store.VersionMeta{}, err
	}
	if store.InTx(ctx) {
		return store.VersionMeta{}, store.ErrTagInTx
	}
	var out store.VersionMeta
	err := t.v.withVersionLock(ctx, func(tx pgx.Tx) error {
		// FOR SHARE OF e holds the live row until the tag commits. A
		// delete, rename or update of it waits, and so does the version row
		// entitymanager captures synchronously after that write: it cannot
		// land between this read and the tag. A write that committed first
		// is seen here (the row is gone, or its new content is captured).
		c, err := scanCandidate(tx.QueryRow(ctx,
			candidateSelect+` WHERE e.id = $1 AND e.face = $2 FOR SHARE OF e`,
			req.Ref.ID, string(req.Ref.Face)))
		if errors.Is(err, pgx.ErrNoRows) {
			return store.ErrNotFound
		}
		if err != nil {
			return err
		}
		if expErr := checkExpect(c, req.Expect); expErr != nil {
			return expErr
		}
		rows, err := listLineage(ctx, tx, req.Ref)
		if err != nil {
			return err
		}
		vseq, captured, err := t.currentVersion(ctx, tx, c, rows)
		if err != nil {
			return err
		}
		if captured {
			if rows, err = listLineage(ctx, tx, req.Ref); err != nil {
				return err
			}
		}
		out, err = setTag(ctx, tx, req.Ref, req.Name, vseq, rows, req.PrincipalUser, req.PrincipalTool)
		return err
	})
	return out, err
}

// checkExpect applies TagCurrent's compare-and-set precondition to the live
// row.
func checkExpect(c sweepCandidate, expect store.EntityVersion) error {
	if expect == "" {
		return nil
	}
	props, err := unmarshalProps(c.props)
	if err != nil {
		return err
	}
	actual := store.VersionOf(&entity.Entity{ID: c.id, Type: c.typ, Content: c.content, Properties: props})
	if actual != expect {
		return &store.VersionConflictError{ID: c.id, Expected: expect, Actual: actual}
	}
	return nil
}

// currentVersion returns the vseq of the version that holds the live row's
// content, capturing one first when the current lifecycle has none. captured
// reports whether it wrote a row.
func (t *versionTagger) currentVersion(
	ctx context.Context, tx pgx.Tx, c sweepCandidate, rows []storeutil.LineageRow,
) (vseq int64, captured bool, err error) {
	_, liveHash, err := c.versionInput("", nil)
	if err != nil {
		return 0, false, err
	}
	current := storeutil.CurrentLifecycle(rows)
	if c.latestHash == liveHash && current[c.latestVseq] {
		// A purge tombstone carries the live hash precisely so the sweep does
		// not re-capture purged content. Tagging must not re-capture it either.
		if store.VersionOp(c.latestOp) == store.VersionOpPurge {
			return 0, false, fmt.Errorf("%w: the current state of %s was purged from history",
				store.ErrVersionNotTaggable, c.id)
		}
		return c.latestVseq, false, nil
	}
	hash, projJSON := t.projection.Projection()
	if hash == "" {
		return 0, false, errors.New("pgstore: no render projection available to capture the current state")
	}
	if c.latestHash == liveHash {
		// The matching row lies outside this face's lineage (an earlier
		// occupant of a reused id). The sweep would dedup against it, so
		// capture explicitly; the sweep then dedups against this row instead.
		in, contentHash, inErr := c.versionInput(hash, projJSON)
		if inErr != nil {
			return 0, false, inErr
		}
		vseq, err = insertVersion(ctx, tx, in, contentHash)
		return vseq, err == nil, err
	}
	vseq, _, err = captureLive(ctx, tx, c, hash, projJSON)
	return vseq, err == nil, err
}

// TagVersion implements [store.VersionTagger].
func (t *versionTagger) TagVersion(ctx context.Context, req store.TagRequest) (store.VersionMeta, error) {
	if err := store.ValidateTagRequest(req, false); err != nil {
		return store.VersionMeta{}, err
	}
	if store.InTx(ctx) {
		return store.VersionMeta{}, store.ErrTagInTx
	}
	var out store.VersionMeta
	err := t.v.withVersionLock(ctx, func(tx pgx.Tx) error {
		rows, err := listLineage(ctx, tx, req.Ref)
		if err != nil {
			return err
		}
		if req.Version > len(rows) {
			return store.ErrNotFound
		}
		row := rows[req.Version-1]
		if !storeutil.Taggable(row, storeutil.CurrentLifecycle(rows)) {
			return fmt.Errorf("%w: version %d of %s is a %s row or predates the current lifecycle",
				store.ErrVersionNotTaggable, req.Version, req.Ref.ID, row.Meta.Op)
		}
		out, err = setTag(ctx, tx, req.Ref, req.Name, row.Vseq, rows, req.PrincipalUser, req.PrincipalTool)
		return err
	})
	return out, err
}

// UntagVersion implements [store.VersionTagger].
func (t *versionTagger) UntagVersion(ctx context.Context, req store.UntagRequest) error {
	if err := store.ValidateUntagRequest(req); err != nil {
		return err
	}
	if store.InTx(ctx) {
		return store.ErrTagInTx
	}
	return t.v.withVersionLock(ctx, func(tx pgx.Tx) error {
		rows, err := listLineage(ctx, tx, req.Ref)
		if err != nil {
			return err
		}
		vseqs := storeutil.LineageVseqs(rows)
		tags, err := readTags(ctx, tx, vseqs)
		if err != nil {
			return err
		}
		if _, ok := storeutil.TaggedRow(rows, tags, req.Name.String()); !ok {
			return store.ErrNotFound
		}
		_, err = tx.Exec(ctx,
			`DELETE FROM version_tags WHERE name = $1 AND vseq = ANY($2)`, req.Name.String(), vseqs)
		return err
	})
}

// VersionByTag implements [store.VersionTagLookup]. The lineage, the tags and
// the snapshot are read in one repeatable-read snapshot, so a concurrent move
// or purge cannot pair a tag with a lineage it no longer belongs to, nor
// shift the ordinal between the lookup and the content read.
func (v *VersionStore) VersionByTag(
	ctx context.Context, ref entity.Ref, name store.VersionTagName,
) (*store.VersionSnapshot, error) {
	if name.IsZero() {
		return nil, fmt.Errorf("%w: missing tag name", store.ErrInvalidVersionTag)
	}
	tx, err := v.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // read-only; nothing to commit
	if _, err = tx.Exec(ctx, `SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY`); err != nil {
		return nil, err
	}
	rows, err := listLineage(ctx, tx, ref)
	if err != nil {
		return nil, err
	}
	tags, err := readTags(ctx, tx, storeutil.LineageVseqs(rows))
	if err != nil {
		return nil, err
	}
	row, ok := storeutil.TaggedRow(rows, tags, name.String())
	if !ok {
		return nil, store.ErrNotFound
	}
	storeutil.ApplyTags(rows, tags)
	row, _ = storeutil.RowByVseq(rows, row.Vseq)
	snap := store.VersionSnapshot{VersionMeta: row.Meta}
	var props []byte
	err = tx.QueryRow(ctx, `
		SELECT ev.content, ev.properties, sv.projection
		FROM entity_versions ev
		JOIN schema_versions sv ON sv.hash = ev.schema_hash
		WHERE ev.vseq = $1`, row.Vseq).Scan(&snap.Content, &props, &snap.Projection)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if snap.Properties, err = unmarshalProps(props); err != nil {
		return nil, err
	}
	return &snap, nil
}

// VersionByTag implements [store.VersionTagger] through the store's own
// lookup, which needs no projection.
func (t *versionTagger) VersionByTag(
	ctx context.Context, ref entity.Ref, name store.VersionTagName,
) (*store.VersionSnapshot, error) {
	return t.v.VersionByTag(ctx, ref, name)
}

// setTag points name at vseq: it deletes every same-name tag in the lineage,
// across all lifecycles, so at most one row of that name exists, then inserts
// the new one. Returns the tagged version's metadata with its tags.
func setTag(
	ctx context.Context, q DBTX, ref entity.Ref, name store.VersionTagName, vseq int64,
	rows []storeutil.LineageRow, user, tool string,
) (store.VersionMeta, error) {
	if _, ok := storeutil.RowByVseq(rows, vseq); !ok {
		return store.VersionMeta{}, store.ErrNotFound
	}
	vseqs := storeutil.LineageVseqs(rows)
	if _, err := q.Exec(ctx,
		`DELETE FROM version_tags WHERE name = $1 AND vseq = ANY($2)`, name.String(), vseqs); err != nil {
		return store.VersionMeta{}, err
	}
	if _, err := q.Exec(ctx, `
		INSERT INTO version_tags (vseq, face, name, tagged_by_user, tagged_by_tool)
		VALUES ($1, $2, $3, $4, $5)`,
		vseq, string(ref.Face), name.String(), user, tool); err != nil {
		return store.VersionMeta{}, err
	}
	tags, err := readTags(ctx, q, vseqs)
	if err != nil {
		return store.VersionMeta{}, err
	}
	storeutil.ApplyTags(rows, tags)
	row, _ := storeutil.RowByVseq(rows, vseq)
	return row.Meta, nil
}

// purgeTaggedTargets returns the tags on the purge target rows.
func purgeTaggedTargets(ctx context.Context, q DBTX, targets []store.PurgeTarget) ([]store.PurgeTag, error) {
	vseqs := make([]int64, len(targets))
	for i, t := range targets {
		vseqs[i] = t.Vseq
	}
	tags, err := readTags(ctx, q, vseqs)
	if err != nil {
		return nil, err
	}
	out := make([]store.PurgeTag, 0, len(tags))
	for _, t := range tags {
		out = append(out, store.PurgeTag{Vseq: t.Vseq, Name: t.Name})
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// deletePurgeTags drops the tags on the purge targets, ahead of the rows the
// foreign key would otherwise refuse to delete.
func deletePurgeTags(ctx context.Context, q DBTX, tags []store.PurgeTag) error {
	if len(tags) == 0 {
		return nil
	}
	vseqs := make([]int64, len(tags))
	for i, t := range tags {
		vseqs[i] = t.Vseq
	}
	_, err := q.Exec(ctx, `DELETE FROM version_tags WHERE vseq = ANY($1)`, vseqs)
	return err
}

var (
	_ store.VersionTaggerProvider = (*VersionStore)(nil)
	_ store.VersionTagLookup      = (*VersionStore)(nil)
)
