package sync

import (
	"context"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// PushOutcome classifies what happened to one record during a push.
type PushOutcome int

const (
	// OutcomePushed: the record was applied on the server and the index updated.
	OutcomePushed PushOutcome = iota
	// OutcomeConflict: the record was HALTED, not applied, because it
	// conflicted on the server — either the server moved since the client's
	// base (412) or a create raced a concurrent first-create of the same id
	// (409). Other records still proceed; resolve with `push --force <id>`.
	OutcomeConflict
	// OutcomeInvalid: the server rejected the content as invalid (422).
	OutcomeInvalid
	// OutcomeDeleted: a local deletion was mirrored to the server.
	OutcomeDeleted
)

// PushRecordResult reports the result for a single record.
type PushRecordResult struct {
	Key     string
	Outcome PushOutcome
	Detail  string // conflict/validation explanation when relevant
	// AdoptedID is the primary-minted id a locally-created entity adopted on a
	// push-create (TKT-8P1TM7). Empty for updates/deletes/relations. The push
	// loop uses it to remap relation endpoints that referenced the temp id.
	AdoptedID string
}

// PushReport summarizes a push run.
type PushReport struct {
	Results   []PushRecordResult
	Conflicts int
	Invalid   int
	Applied   int
	Deleted   int
	Locked    int // records skipped because they were locked (git-crypt etc.)
}

// Push computes the local diff against the index and pushes every diverged
// record to the server in topological order, updating the index as each one is
// confirmed. A conflict (412) or validation failure (422) halts that single
// record with a report entry; other records still proceed (per-record idempotent
// replay — re-running resumes). The index is saved by the caller after Push
// returns so partial progress is durable.
func (e *Engine) Push(ctx context.Context) (*PushReport, error) {
	if err := e.ensureSchema(ctx); err != nil {
		return nil, err
	}
	snap, locked, err := SnapshotLocal(ctx, e.store)
	if err != nil {
		return nil, err
	}
	changes := orderForApply(DiffLocal(snap, e.idx),
		func(c LocalChange) (Kind, bool) { return c.Kind, c.Deleted },
		func(c LocalChange) string { return c.Key })

	report := &PushReport{Locked: locked}
	// A locally-created entity is pushed under a temp id and adopts the
	// primary-minted id, which RenameEntity applies to the entity AND every
	// incident relation in the LOCAL store. The pre-push snapshot still holds the
	// OLD (temp) endpoints, so relation changes captured against it are remapped
	// through the adoptions recorded this run before pushing (RR-SYNCR2) — the
	// entities-before-relations order guarantees the adoptions have landed first.
	adopted := map[string]string{} // temp id → minted id, filled as creates apply
	for _, ch := range changes {
		if ch.Kind == KindRelation && !ch.Deleted {
			if cur, ok := e.remapRelation(ctx, ch, adopted); ok {
				ch = cur
			} else {
				continue // endpoint not resolvable (missing) — skip; a re-run retries
			}
		}
		// Fail fast and locally on a key the server would reject, rather than
		// emitting a doomed request and surfacing an opaque 400. The key comes
		// from the local working copy, which never passed the server allowlist.
		if !syncableKey(ch.Key, ch.Kind) {
			report.Results = append(report.Results, PushRecordResult{
				Key: ch.Key, Outcome: OutcomeInvalid,
				Detail: "id contains characters that cannot be synced (path separators, '..', or control chars)",
			})
			report.Invalid++
			continue
		}
		res, perr := e.pushOne(ctx, ch)
		if perr != nil {
			return report, perr // transport/auth error — abort the whole run
		}
		if res.AdoptedID != "" && res.AdoptedID != ch.Key {
			adopted[ch.Key] = res.AdoptedID // temp → minted, for later relation remap
		}
		report.Results = append(report.Results, res)
		switch res.Outcome {
		case OutcomePushed:
			report.Applied++
		case OutcomeDeleted:
			report.Deleted++
		case OutcomeConflict:
			report.Conflicts++
		case OutcomeInvalid:
			report.Invalid++
		}
	}
	return report, nil
}

// remapRelation rewrites a snapshotted relation change's endpoints through the
// temp→minted-id adoptions recorded earlier in this push run, then re-reads the
// live relation under the remapped key so its properties/hash are current. This
// is what lets a relation between two locally-created entities push in the SAME
// run: the entities adopt their minted ids first (entities-before-relations),
// RenameEntity re-keys the local relation, and here we follow that re-key.
// Returns false when the remapped relation is not present locally (its endpoints
// resolved to something the store doesn't have) — skipped, a re-run retries.
func (e *Engine) remapRelation(ctx context.Context, ch LocalChange, adopted map[string]string) (LocalChange, bool) {
	rel := ch.Record.Relation
	if rel == nil {
		return ch, false
	}
	from := remapID(rel.From, adopted)
	to := remapID(rel.To, adopted)

	cur, err := e.store.GetRelation(ctx, from, rel.Type, to)
	if err != nil {
		return ch, false
	}
	ch.Key = RelationKey(from, rel.Type, to)
	ch.Record.Relation = cur
	ch.Record.Hash = canonical.HashRelation(*cur)
	// A relation between adopted endpoints has never been seen by the primary
	// under its new key, so it is a first push (no If-Match baseline).
	if _, indexed := e.idx.Baseline(ch.Key); !indexed {
		ch.Base = ""
	}
	return ch, true
}

// remapID returns the minted id a temp id was adopted to this run, or the id
// unchanged when it was not adopted.
func remapID(id string, adopted map[string]string) string {
	if minted, ok := adopted[id]; ok {
		return minted
	}
	return id
}

// syncableKey reports whether a record key is safe to put on the wire — i.e.
// every segment passes the same allowlist the server's validIDSegment enforces
// (non-empty, no path separators, no "..", no control chars). An entity key is
// one segment; a relation key is three. This is a client-side mirror of the
// server check so an unsyncable local record is reported locally, not via an
// opaque remote 400.
func syncableKey(key string, kind Kind) bool {
	if kind == KindRelation {
		from, relType, to, ok := splitRelationKey(key)
		return ok && validIDSegment(from) && validIDSegment(relType) && validIDSegment(to)
	}
	return validIDSegment(key)
}

// validIDSegment mirrors the server's allowlist (internal/dataentry/sync.go).
func validIDSegment(s string) bool {
	if s == "" || strings.ContainsAny(s, "/\\") || strings.Contains(s, "..") {
		return false
	}
	for _, c := range s {
		if c < firstPrintableASCII { // no control characters
			return false
		}
	}
	return true
}

// firstPrintableASCII is the space character; anything below it is an ASCII
// control character, disallowed in a record id segment.
const firstPrintableASCII = 0x20

// pushOne applies a single change and updates the index on success.
func (e *Engine) pushOne(ctx context.Context, ch LocalChange) (PushRecordResult, error) {
	if ch.Deleted {
		return e.pushDelete(ctx, ch)
	}
	return e.pushUpsert(ctx, ch)
}

func (e *Engine) pushUpsert(ctx context.Context, ch LocalChange) (PushRecordResult, error) {
	switch ch.Kind {
	case KindEntity:
		return e.pushEntityUpsert(ctx, ch)
	case KindRelation:
		return e.pushRelationUpsert(ctx, ch)
	default:
		return PushRecordResult{}, fmt.Errorf("internal: unknown kind for %q", ch.Key)
	}
}

// pushEntityUpsert pushes an entity through /api/v1: a record the primary has
// never seen (no baseline) is CREATED (primary mints the id, replica adopts it);
// an existing record is PATCHed with visible fields only, under If-Match.
func (e *Engine) pushEntityUpsert(ctx context.Context, ch LocalChange) (PushRecordResult, error) {
	ent := ch.Record.Entity
	plural, err := e.pluralFor(ent.Type)
	if err != nil {
		return PushRecordResult{}, err
	}

	// CREATE only for a record the primary has NEVER seen: not in the index AND
	// no known remote base. A non-empty Base means the record already exists
	// remotely (e.g. ForcePush re-read the remote hash for an unindexed record, or
	// after a lost-ack create) — creating again would mint a DUPLICATE, so take
	// the id-stable PATCH path under that If-Match instead. (An ordinary
	// first-push has both indexed=false and Base="".)
	_, indexed := e.idx.Baseline(ch.Key)
	if !indexed && ch.Base == "" {
		// First push of a locally-created record: the primary owns the id.
		res, cerr := e.client.CreateEntity(ctx, plural, v1CreateEntity{
			Properties: ent.Properties, Content: ent.Content,
		})
		if cerr != nil {
			return PushRecordResult{}, cerr
		}
		return e.recordCreate(ctx, ch, res)
	}

	// Content is a *string tri-state on the v1 PATCH; the replica owns the whole
	// body it holds, so it always sends content on an update.
	content := ent.Content
	res, perr := e.client.PatchEntity(ctx, plural, ent.ID, v1PatchEntity{
		Properties: ent.Properties, Content: &content,
	}, ch.Base)
	if perr != nil {
		return PushRecordResult{}, perr
	}
	return e.recordPush(ch.Key, ch.Record.Hash, ent.Type, res), nil
}

// pushRelationUpsert pushes a relation through the /api/v1 relation write
// endpoint (visible meta only). The route needs the FROM entity's type plural,
// resolved from the local store. A relation whose endpoints are not both present
// on the primary yet (e.g. a locally-created FROM still under a temp id) will be
// rejected by the primary's endpoint check; ordering (entities before relations)
// makes the common case succeed on the first pass, and a re-run resolves the
// rest (RR-SYNCR2).
func (e *Engine) pushRelationUpsert(ctx context.Context, ch LocalChange) (PushRecordResult, error) {
	rel := ch.Record.Relation
	fromPlural, err := e.pluralForLocalEntity(ctx, rel.From)
	if err != nil {
		return PushRecordResult{}, err
	}
	res, perr := e.client.PutRelation(ctx, fromPlural, RelationBody{
		From: rel.From, Type: rel.Type, To: rel.To, Properties: rel.Properties, Content: rel.Content,
	}, ch.Base)
	if perr != nil {
		return PushRecordResult{}, perr
	}
	return e.recordPush(ch.Key, ch.Record.Hash, rel.Type, res), nil
}

// pluralForLocalEntity resolves the /api/v1 route plural for a local entity id
// by reading its type from the local store, then mapping type→plural via the
// primary's schema. Used to build relation-write routes (keyed by the FROM
// entity's plural).
func (e *Engine) pluralForLocalEntity(ctx context.Context, id string) (string, error) {
	ent, err := e.store.GetEntity(ctx, id)
	if err != nil {
		return "", fmt.Errorf("resolve type of local entity %q for relation route: %w", id, err)
	}
	return e.pluralFor(ent.Type)
}

// recordCreate handles a push-create outcome: the primary minted an id, so the
// replica ADOPTS it and re-keys its local temp-id record to match (TKT-8P1TM7,
// "sync is a fancy browser" — the primary owns ids). RenameEntity rewrites every
// incident relation endpoint too, so relations that referenced the temp id are
// remapped in one call. The index moves from the temp key to the minted id.
//
// If the create did not apply (conflict/invalid), it is classified like any
// other non-applied push and the local temp id is left untouched for a re-run.
func (e *Engine) recordCreate(ctx context.Context, ch LocalChange, res *PushResult) (PushRecordResult, error) {
	if !res.Applied {
		return classifyConflict(ch.Key, res), nil
	}
	newID := res.CreatedID
	if newID == "" {
		// The primary applied a create but returned no id — we cannot adopt or
		// re-baseline safely. Surface it rather than guess (a silent mis-key would
		// desync the record permanently).
		return PushRecordResult{}, fmt.Errorf("create of %q applied but the server returned no id", ch.Key)
	}

	if newID != ch.Key {
		// Push DOES write locally on this path: the primary minted an id that
		// differs from the local temp id, and adopting it is a local rename.
		// So a nil applier is fatal here rather than merely limiting — guard it
		// like pull and force do, instead of dereferencing into a panic
		// (TKT-IVSJV6).
		if e.applier == nil {
			return PushRecordResult{}, fmt.Errorf(
				"adopt server id %q for local %q: %w", newID, ch.Key, errLocalApplierRequired)
		}
		if _, err := e.applier.RenameEntity(ctx, ch.Key, newID, entity.RenameOptions{}); err != nil {
			return PushRecordResult{}, fmt.Errorf("adopt server id %q for local %q: %w", newID, ch.Key, err)
		}
		e.idx.Delete(ch.Key) // move the baseline from the temp key to the minted id
	}
	// Re-read the renamed record to baseline Local against its ACTUAL post-rename
	// canonical hash — the id is part of the hash, so a create under a temp id
	// then renamed has a different canonical hash than what was pushed. Without
	// this the next push would see the record as dirty and re-push it. The create
	// already succeeded on the primary, so a read failure here must surface (not
	// baseline with an empty Local, which would force a spurious re-push forever);
	// a re-run resumes cleanly since the record is now under its minted id.
	ent, err := e.store.GetEntity(ctx, newID)
	if err != nil {
		return PushRecordResult{}, fmt.Errorf("re-baseline created %q after adopt: %w", newID, err)
	}
	e.idx.Set(newID, res.Hash, canonical.HashEntity(*ent), ch.Record.Entity.Type)
	return PushRecordResult{
		Key: ch.Key, Outcome: OutcomePushed, Detail: "created as " + newID, AdoptedID: newID,
	}, nil
}

func (e *Engine) pushDelete(ctx context.Context, ch LocalChange) (PushRecordResult, error) {
	// The record is already gone locally, so resolve its route plural from the
	// baseline's recorded Type (stored precisely for this case).
	base, _ := e.idx.Baseline(ch.Key)

	var (
		res *PushResult
		err error
	)
	switch ch.Kind {
	case KindEntity:
		plural, perr := e.pluralFor(base.Type)
		if perr != nil {
			return PushRecordResult{}, perr
		}
		res, err = e.client.DeleteEntity(ctx, plural, ch.Key, ch.Base)
	case KindRelation:
		from, relType, to, ok := splitRelationKey(ch.Key)
		if !ok {
			return PushRecordResult{}, fmt.Errorf("internal: malformed relation key %q", ch.Key)
		}
		fromPlural, perr := e.pluralForRelationFrom(ctx, from)
		if perr != nil {
			return PushRecordResult{}, perr
		}
		res, err = e.client.DeleteRelation(ctx, fromPlural, from, relType, to, ch.Base)
	}
	if err != nil {
		return PushRecordResult{}, err
	}
	if res.Applied {
		e.idx.Delete(ch.Key) // converged: gone on both ends
		return PushRecordResult{Key: ch.Key, Outcome: OutcomeDeleted}, nil
	}
	return classifyConflict(ch.Key, res), nil
}

// pluralForRelationFrom resolves the FROM entity's route plural for a relation
// whose own local record is gone (a delete). It uses the FROM entity's baseline
// Type when present (the entity may still be indexed), falling back to a local
// store read.
func (e *Engine) pluralForRelationFrom(ctx context.Context, from string) (string, error) {
	if base, ok := e.idx.Baseline(from); ok && base.Type != "" {
		return e.pluralFor(base.Type)
	}
	return e.pluralForLocalEntity(ctx, from)
}

// recordPush updates the index on a successful upsert and classifies the result.
// On Applied it records BOTH baseline tokens: server = the primary's returned
// opaque ETag (the conflict token echoed as the next If-Match), local = the
// canonical hash of the working record we just pushed (the replica's own
// change-detector). A push means the working copy is now the agreed state, so
// its canonical hash is the Local baseline; the primary's ETag is whatever it
// returned. typ is the record's type, stored so a later delete can resolve its
// route.
func (e *Engine) recordPush(key, localHash, typ string, res *PushResult) PushRecordResult {
	if res.Applied {
		e.idx.Set(key, res.Hash, localHash, typ)
		return PushRecordResult{Key: key, Outcome: OutcomePushed}
	}
	return classifyConflict(key, res)
}

// classifyConflict turns a non-applied PushResult into a halt report entry.
// A conflict (412 or 409) halts ONLY this record — the push loop continues to
// the next — and the index is left untouched so a re-run replays it after the
// operator resolves it. A 409 (concurrent first-create) gets a create-specific
// message; a 412 (base moved) gets the base-changed message.
func classifyConflict(key string, res *PushResult) PushRecordResult {
	switch {
	case res.CreatedConcurrently:
		return PushRecordResult{
			Key: key, Outcome: OutcomeConflict,
			Detail: "created concurrently by a peer since your last sync; resolve with `rela sync push --force " + key + "` (local wins) or `rela sync pull --force " + key + "` (remote wins)",
		}
	case res.Conflict:
		return PushRecordResult{
			Key: key, Outcome: OutcomeConflict,
			Detail: "remote changed since your last sync; resolve with `rela sync push --force " + key + "` (local wins) or `rela sync pull --force " + key + "` (remote wins)",
		}
	case res.Invalid:
		return PushRecordResult{Key: key, Outcome: OutcomeInvalid, Detail: res.Detail}
	default:
		return PushRecordResult{Key: key, Outcome: OutcomeConflict, Detail: "push not applied"}
	}
}
