package sync

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// isLocalNotFound reports whether a local apply/delete error is a not-found,
// which deleteLocal treats as already-converged (idempotent mirror delete).
//
// It must cover every not-found shape the manager can return, because they do
// NOT share a single sentinel: DeleteEntity wraps [entitymanager.ErrEntityNotFound],
// but DeleteRelation wraps [store.ErrNotFound] directly (not the entity sentinel).
// Missing the relation case would wedge a pull on resume — a re-played relation
// tombstone for an already-absent relation would abort the whole run instead of
// being the no-op it should be.
func isLocalNotFound(err error) bool {
	return errors.Is(err, entitymanager.ErrEntityNotFound) ||
		errors.Is(err, entitymanager.ErrRelationNotFound) ||
		errors.Is(err, store.ErrNotFound)
}

// errRemoteAbsent is an internal signal that a fetch returned 404 — used by the
// apply path to convert a manifest "deleted" or a vanished record into a local
// delete.
var errRemoteAbsent = errors.New("remote record absent")

// errRemoteApplierRequired is returned when a pull (or force-pull) is attempted
// without a local applier wired — pull writes locally and cannot run read-only.
var errRemoteApplierRequired = errors.New("sync: pull requires a local applier")

// PullOutcome classifies what happened to one record during a pull.
type PullOutcome int

const (
	// OutcomePulled: a remote create/update was applied locally; index updated.
	OutcomePulled PullOutcome = iota
	// OutcomePulledDelete: a remote tombstone was mirrored as a local delete.
	OutcomePulledDelete
	// OutcomePullConflict: the record changed both remotely AND locally — HALTED.
	OutcomePullConflict
	// OutcomePullSkipped: remote hash equals the index — already in sync, no-op.
	OutcomePullSkipped
)

// PullRecordResult reports the result for a single manifest entry.
type PullRecordResult struct {
	Key     string
	Outcome PullOutcome
	Detail  string
}

// PullReport summarizes a pull run, including the cursor to persist. Cursor is
// the highest watermark whose records were ALL confirmed-applied; a halted
// conflict caps the cursor at the last fully-applied point so a re-run revisits
// the conflict rather than skipping past it.
type PullReport struct {
	Results   []PullRecordResult
	Applied   int
	Deleted   int
	Conflicts int
	Skipped   int
	Cursor    string
}

// Pull fetches the manifest since the index cursor, diffs each entry, and
// applies remote changes locally in topological order. A record that changed on
// both ends halts with a conflict entry (resolve with --force). The cursor only
// advances past records that were applied or confirmed already-in-sync, so a
// conflict (or a mid-batch transport failure, surfaced as an error) leaves the
// cursor where a re-run resumes correctly.
func (e *Engine) Pull(ctx context.Context) (*PullReport, error) {
	if e.applier == nil {
		return nil, errRemoteApplierRequired
	}
	man, err := e.client.Manifest(ctx, e.idx.Cursor)
	if err != nil {
		return nil, err
	}

	snap, _, err := SnapshotLocal(ctx, e.store)
	if err != nil {
		return nil, err
	}

	// The manifest may list the same key more than once (one entry per change
	// since the cursor). Collapse to the LAST change per key — that is the record's
	// current server state, and the only one worth fetching/applying. Without this
	// a record edited twice remotely would be planned (and reported) twice.
	latest := dedupeChanges(man.Changes)

	// Decide each entry's disposition first (no I/O), then order the ones that
	// need applying. Skips and conflicts do not change local state.
	type plan struct {
		change   ManifestChange
		key      string
		kind     Kind
		conflict bool
		skip     bool
	}
	plans := make([]plan, 0, len(latest))
	for _, ch := range latest {
		p := plan{change: ch, key: ch.ID, kind: manifestKind(ch.Kind)}
		p.conflict, p.skip = e.classifyPull(ch, snap)
		plans = append(plans, p)
	}

	// Apply order: entities before relations (creates/updates), relation-deletes
	// before entity-deletes. Skips/conflicts are pass-through (no apply), but we
	// keep them in the ordered stream so the report reads in a sensible order.
	ordered := orderForApply(plans,
		func(p plan) (Kind, bool) { return p.kind, p.change.Deleted },
		func(p plan) string { return p.key })

	report := &PullReport{Cursor: e.idx.Cursor}
	for _, p := range ordered {
		switch {
		case p.skip:
			report.Results = append(report.Results, PullRecordResult{Key: p.key, Outcome: OutcomePullSkipped})
			report.Skipped++
		case p.conflict:
			report.Results = append(report.Results, PullRecordResult{
				Key: p.key, Outcome: OutcomePullConflict,
				Detail: "changed both locally and remotely; resolve with `rela sync pull --force " + p.key + "` (remote wins) or `rela sync push --force " + p.key + "` (local wins)",
			})
			report.Conflicts++
		default:
			res, aerr := e.applyOne(ctx, p.change, p.kind)
			if aerr != nil {
				// Mid-batch failure: do NOT advance the cursor past the failure.
				// The per-record index updates already committed let a re-run skip
				// completed records; the unchanged cursor makes it re-fetch and
				// resume from the last durably-applied watermark.
				report.Cursor = e.idx.Cursor
				return report, aerr
			}
			report.Results = append(report.Results, *res)
			countPull(report, res.Outcome)
		}
	}

	// Advance the cursor only if NO conflict halted a record — a conflict means
	// the operator must act before we can claim convergence up to the new
	// watermark. With no conflicts, every entry was applied or already in sync,
	// so the server's cursor is safe to adopt.
	if report.Conflicts == 0 {
		report.Cursor = man.Cursor
		e.idx.Cursor = man.Cursor
	} else {
		report.Cursor = e.idx.Cursor // unchanged; re-run revisits the conflict
	}
	return report, nil
}

// classifyPull decides whether a manifest entry is a skip (remote == index) or a
// conflict (remote differs AND local also dirty). Returns (conflict, skip);
// both false means "apply it".
func (e *Engine) classifyPull(ch ManifestChange, snap *LocalSnapshot) (conflict, skip bool) {
	base, indexed := e.idx.Hash(ch.ID)
	local, localPresent := snap.Records[ch.ID]
	localDirty := (localPresent && (!indexed || local.Hash != base)) || (!localPresent && indexed)

	if ch.Deleted {
		// Remote tombstone. If the index already has no record, it's a no-op.
		if !indexed {
			return false, true
		}
		// If the local copy diverged from the index, deleting it would lose local
		// work → conflict.
		if localDirty {
			return true, false
		}
		return false, false // clean local delete-mirror
	}

	// Remote upsert. We cannot know the remote hash without fetching, so the
	// manifest alone can't prove "remote == index". We use the index vs local to
	// gate conflicts: if local is dirty, applying the remote would clobber local
	// edits → conflict. If local is clean, fetch + apply (applyOne re-checks the
	// fetched hash against the index to skip a true no-op).
	if localDirty {
		return true, false
	}
	return false, false
}

// applyOne fetches and applies a single remote upsert, or mirrors a delete. It
// re-checks the fetched hash against the index so an unchanged record (remote
// moved then moved back, or a manifest replay) is a cheap skip, not a rewrite.
func (e *Engine) applyOne(ctx context.Context, ch ManifestChange, kind Kind) (*PullRecordResult, error) {
	if ch.Deleted {
		if err := e.deleteLocal(ctx, kind, ch.ID); err != nil {
			return nil, err
		}
		e.idx.Delete(ch.ID)
		return &PullRecordResult{Key: ch.ID, Outcome: OutcomePulledDelete}, nil
	}
	return e.applyRemote(ctx, kind, ch.ID)
}

// applyRemote fetches a remote record and applies it locally via the
// id-preserving applier, then re-baselines the index to the remote hash. A 404
// surfaces as errRemoteAbsent so the caller can mirror a delete.
func (e *Engine) applyRemote(ctx context.Context, kind Kind, key string) (*PullRecordResult, error) {
	switch kind {
	case KindEntity:
		fe, err := e.client.GetEntity(ctx, key)
		if err != nil {
			if isNotFound(err) {
				return nil, errRemoteAbsent
			}
			return nil, err
		}
		if cur, ok := e.idx.Hash(key); ok && cur == fe.Hash {
			return &PullRecordResult{Key: key, Outcome: OutcomePullSkipped}, nil
		}
		ent := &entity.Entity{
			ID: fe.Body.ID, Type: fe.Body.Type, Properties: fe.Body.Properties, Content: fe.Body.Content,
		}
		if ent.ID == "" {
			ent.ID = key
		}
		if _, err := e.applier.ApplyEntity(ctx, ent); err != nil {
			return nil, fmt.Errorf("apply entity %s: %w", key, err)
		}
		e.idx.Set(key, fe.Hash)
		return &PullRecordResult{Key: key, Outcome: OutcomePulled}, nil
	default:
		from, relType, to, ok := splitRelationKey(key)
		if !ok {
			return nil, fmt.Errorf("internal: malformed relation key %q", key)
		}
		fr, err := e.client.GetRelation(ctx, from, relType, to)
		if err != nil {
			if isNotFound(err) {
				return nil, errRemoteAbsent
			}
			return nil, err
		}
		if cur, ok := e.idx.Hash(key); ok && cur == fr.Hash {
			return &PullRecordResult{Key: key, Outcome: OutcomePullSkipped}, nil
		}
		rel := &entity.Relation{
			From: from, Type: relType, To: to, Properties: fr.Body.Properties, Content: fr.Body.Content,
		}
		if _, err := e.applier.ApplyRelation(ctx, rel); err != nil {
			return nil, fmt.Errorf("apply relation %s: %w", key, err)
		}
		e.idx.Set(key, fr.Hash)
		return &PullRecordResult{Key: key, Outcome: OutcomePulled}, nil
	}
}

// deleteLocal mirrors a remote delete in the working copy. A missing local
// record is not an error — the end state (absent) already holds (idempotent).
func (e *Engine) deleteLocal(ctx context.Context, kind Kind, key string) error {
	switch kind {
	case KindEntity:
		if _, err := e.applier.DeleteEntity(ctx, key, true); err != nil {
			if isLocalNotFound(err) {
				return nil
			}
			return fmt.Errorf("delete entity %s: %w", key, err)
		}
		return nil
	default:
		from, relType, to, ok := splitRelationKey(key)
		if !ok {
			return fmt.Errorf("internal: malformed relation key %q", key)
		}
		if err := e.applier.DeleteRelation(ctx, from, relType, to); err != nil {
			if isLocalNotFound(err) {
				return nil
			}
			return fmt.Errorf("delete relation %s: %w", key, err)
		}
		return nil
	}
}

func countPull(r *PullReport, o PullOutcome) {
	switch o {
	case OutcomePulled:
		r.Applied++
	case OutcomePulledDelete:
		r.Deleted++
	case OutcomePullSkipped:
		r.Skipped++
	case OutcomePullConflict:
		r.Conflicts++
	}
}

// manifestKind maps the wire kind ("e"/"r") to the engine's Kind.
func manifestKind(k string) Kind {
	if k == "r" {
		return KindRelation
	}
	return KindEntity
}

// dedupeChanges collapses repeated keys to their LAST occurrence, preserving the
// order of each key's final appearance. The manifest is ordered by seq, so the
// last entry for a key is its newest state (e.g. an edit followed by a delete
// collapses to the delete). Keyed by kind+id so an entity and a relation that
// happen to share an id string never collide.
//
// INVARIANT this relies on (pgstore manifest contract): manifest seq strictly
// increases, and a record recreated after deletion always out-seqs its prior
// tombstone — so for delete-then-recreate the last entry is the live upsert
// (correct), and for recreate-then-delete it is the tombstone (correct). If the
// server ever emitted a tombstone with a higher seq than a still-live row, this
// would mirror a delete of a live record. The pgstore manifest guarantees it
// does not (deletions and live rows share the monotonic seq space).
func dedupeChanges(changes []ManifestChange) []ManifestChange {
	lastIdx := make(map[string]int, len(changes))
	for i, ch := range changes {
		lastIdx[ch.Kind+":"+ch.ID] = i
	}
	out := make([]ManifestChange, 0, len(lastIdx))
	for i, ch := range changes {
		if lastIdx[ch.Kind+":"+ch.ID] == i {
			out = append(out, ch)
		}
	}
	return out
}
