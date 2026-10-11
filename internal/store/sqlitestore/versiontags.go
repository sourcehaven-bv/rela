package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// Version tags (TKT-VO6VG9). The contract is on store.VersionTagger; this file
// is the SQLite mechanism.
//
// Every write runs under versionMu, which the sweep and purge also hold, in
// one BEGIN IMMEDIATE transaction on one pinned connection. versionMu keeps a
// tag from racing a sweep capture (the capture-now would duplicate it) or a
// purge (which could delete the row being tagged). BEGIN IMMEDIATE takes the
// write lock up front, so the read-then-write cannot fail on a lock upgrade,
// and serializes the tag against ordinary entity writes for the CAS check.

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
		return nil, errors.New("sqlitestore: version tagger needs a projection provider")
	}
	return &versionTagger{v: v, projection: projection}, nil
}

// withVersionWrite runs fn in one write transaction under versionMu.
//
// Called on a ctx inside a store Tx it would wait on that transaction's
// write lock; the busy timeout bounds the wait. The tagger refuses a ctx
// marked by store.ContextInTx before getting here.
func (v *VersionStore) withVersionWrite(ctx context.Context, fn func(q querier) error) error {
	v.store.versionMu.Lock()
	defer v.store.versionMu.Unlock()

	conn, err := v.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("sqlitestore: acquire connection: %w", err)
	}
	committed := false
	defer func() {
		// Without the rollback a failed or panicking fn would return the
		// connection to the pool with the transaction open; see Store.Tx.
		if !committed {
			_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		}
		_ = conn.Close()
	}()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("sqlitestore: begin version tag write: %w", err)
	}
	if err := fn(conn); err != nil {
		return err
	}
	if _, err := conn.ExecContext(context.WithoutCancel(ctx), "COMMIT"); err != nil {
		return fmt.Errorf("sqlitestore: commit version tag write: %w", err)
	}
	committed = true
	return nil
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
	err := t.v.withVersionWrite(ctx, func(q querier) error {
		c, err := scanCandidate(q.QueryRowContext(ctx,
			candidateSelect+` WHERE c.id = ? AND c.face = ?`, req.Ref.ID, string(req.Ref.Face)))
		if errors.Is(err, sql.ErrNoRows) {
			return store.ErrNotFound
		}
		if err != nil {
			return err
		}
		if expErr := checkExpect(c, req.Expect); expErr != nil {
			return expErr
		}
		rows, err := listLineage(ctx, q, req.Ref)
		if err != nil {
			return err
		}
		vseq, captured, err := t.currentVersion(ctx, q, c, rows)
		if err != nil {
			return err
		}
		if captured {
			if rows, err = listLineage(ctx, q, req.Ref); err != nil {
				return err
			}
		}
		out, err = setTag(ctx, q, req.Ref, req.Name, vseq, rows, req.PrincipalUser, req.PrincipalTool)
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
	ctx context.Context, q querier, c sweepCandidate, rows []storeutil.LineageRow,
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
		return 0, false, errors.New("sqlitestore: no render projection available to capture the current state")
	}
	if c.latestHash == liveHash {
		// The matching row lies outside this face's lineage (an earlier
		// occupant of a reused id). The sweep would dedup against it, so
		// capture explicitly; the sweep then dedups against this row instead.
		in, contentHash, inErr := c.versionInput(hash, projJSON)
		if inErr != nil {
			return 0, false, inErr
		}
		vseq, err = insertVersion(ctx, q, in, contentHash)
		return vseq, err == nil, err
	}
	vseq, _, err = captureLive(ctx, q, c, hash, projJSON)
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
	err := t.v.withVersionWrite(ctx, func(q querier) error {
		rows, err := listLineage(ctx, q, req.Ref)
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
		out, err = setTag(ctx, q, req.Ref, req.Name, row.Vseq, rows, req.PrincipalUser, req.PrincipalTool)
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
	return t.v.withVersionWrite(ctx, func(q querier) error {
		rows, err := listLineage(ctx, q, req.Ref)
		if err != nil {
			return err
		}
		vseqs := storeutil.LineageVseqs(rows)
		tags, err := readTags(ctx, q, vseqs)
		if err != nil {
			return err
		}
		if _, ok := storeutil.TaggedRow(rows, tags, req.Name.String()); !ok {
			return store.ErrNotFound
		}
		return deleteNamedTags(ctx, q, req.Name, vseqs)
	})
}

// VersionByTag implements [store.VersionTagLookup]. The lineage, the tags and
// the snapshot are read in one transaction, so under WAL all three reads see
// one snapshot and a purge cannot shift the ordinal between them.
func (v *VersionStore) VersionByTag(
	ctx context.Context, ref entity.Ref, name store.VersionTagName,
) (*store.VersionSnapshot, error) {
	if name.IsZero() {
		return nil, fmt.Errorf("%w: missing tag name", store.ErrInvalidVersionTag)
	}
	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: begin tag read: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
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
	var props string
	err = tx.QueryRowContext(ctx, `
		SELECT ev.content, ev.properties, sv.projection
		FROM entity_versions ev
		JOIN schema_versions sv ON sv.hash = ev.schema_hash
		WHERE ev.vseq = ?`, row.Vseq).Scan(&snap.Content, &props, &snap.Projection)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: read tagged version of %s: %w", ref.ID, err)
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
	ctx context.Context, q querier, ref entity.Ref, name store.VersionTagName, vseq int64,
	rows []storeutil.LineageRow, user, tool string,
) (store.VersionMeta, error) {
	if _, ok := storeutil.RowByVseq(rows, vseq); !ok {
		return store.VersionMeta{}, store.ErrNotFound
	}
	vseqs := storeutil.LineageVseqs(rows)
	if err := deleteNamedTags(ctx, q, name, vseqs); err != nil {
		return store.VersionMeta{}, err
	}
	if _, err := q.ExecContext(ctx, `
		INSERT INTO version_tags (vseq, face, name, tagged_by_user, tagged_by_tool, tagged_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		vseq, string(ref.Face), name.String(), user, tool, timestampNow()); err != nil {
		return store.VersionMeta{}, fmt.Errorf("sqlitestore: insert version tag: %w", err)
	}
	tags, err := readTags(ctx, q, vseqs)
	if err != nil {
		return store.VersionMeta{}, err
	}
	storeutil.ApplyTags(rows, tags)
	row, _ := storeutil.RowByVseq(rows, vseq)
	return row.Meta, nil
}

// deleteNamedTags deletes the tags called name on any of vseqs.
func deleteNamedTags(ctx context.Context, q querier, name store.VersionTagName, vseqs []int64) error {
	if len(vseqs) == 0 {
		return nil
	}
	ph, args := idPlaceholders(vseqs)
	args = append([]any{name.String()}, args...)
	if _, err := q.ExecContext(ctx,
		`DELETE FROM version_tags WHERE name = ? AND vseq IN (`+ph+`)`, args...); err != nil {
		return fmt.Errorf("sqlitestore: delete version tags: %w", err)
	}
	return nil
}

// purgeTaggedTargets returns the tags on the purge target rows.
func purgeTaggedTargets(ctx context.Context, q querier, targets []store.PurgeTarget) ([]store.PurgeTag, error) {
	vseqs := make([]int64, len(targets))
	for i, t := range targets {
		vseqs[i] = t.Vseq
	}
	tags, err := readTags(ctx, q, vseqs)
	if err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		return nil, nil
	}
	out := make([]store.PurgeTag, len(tags))
	for i, t := range tags {
		out[i] = store.PurgeTag{Vseq: t.Vseq, Name: t.Name}
	}
	return out, nil
}

// deletePurgeTags drops the tags on the purge targets, ahead of the rows the
// foreign key would otherwise refuse to delete.
func deletePurgeTags(ctx context.Context, q querier, tags []store.PurgeTag) error {
	if len(tags) == 0 {
		return nil
	}
	vseqs := make([]int64, len(tags))
	for i, t := range tags {
		vseqs[i] = t.Vseq
	}
	ph, args := idPlaceholders(vseqs)
	if _, err := q.ExecContext(ctx, `DELETE FROM version_tags WHERE vseq IN (`+ph+`)`, args...); err != nil {
		return fmt.Errorf("sqlitestore: delete purged version tags: %w", err)
	}
	return nil
}

var (
	_ store.VersionTaggerProvider = (*VersionStore)(nil)
	_ store.VersionTagLookup      = (*VersionStore)(nil)
)
