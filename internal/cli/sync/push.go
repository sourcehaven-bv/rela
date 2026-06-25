package sync

import (
	"context"
	"fmt"
	"strings"
)

// PushOutcome classifies what happened to one record during a push.
type PushOutcome int

const (
	// OutcomePushed: the record was applied on the server and the index updated.
	OutcomePushed PushOutcome = iota
	// OutcomeConflict: the server moved since the client's base (412) — the
	// record was HALTED, not applied. Resolve with `push --force <id>`.
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
	snap, locked, err := SnapshotLocal(ctx, e.store)
	if err != nil {
		return nil, err
	}
	changes := orderForApply(DiffLocal(snap, e.idx),
		func(c LocalChange) (Kind, bool) { return c.Kind, c.Deleted },
		func(c LocalChange) string { return c.Key })

	report := &PushReport{Locked: locked}
	for _, ch := range changes {
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
	var (
		res *PushResult
		err error
	)
	switch ch.Kind {
	case KindEntity:
		ent := ch.Record.Entity
		res, err = e.client.PutEntity(ctx, EntityBody{
			ID: ent.ID, Type: ent.Type, Properties: ent.Properties, Content: ent.Content,
		}, ch.Base)
	case KindRelation:
		rel := ch.Record.Relation
		res, err = e.client.PutRelation(ctx, RelationBody{
			From: rel.From, Type: rel.Type, To: rel.To, Properties: rel.Properties, Content: rel.Content,
		}, ch.Base)
	}
	if err != nil {
		return PushRecordResult{}, err
	}
	return e.recordPush(ch.Key, ch.Record.Hash, res), nil
}

func (e *Engine) pushDelete(ctx context.Context, ch LocalChange) (PushRecordResult, error) {
	var (
		res *PushResult
		err error
	)
	switch ch.Kind {
	case KindEntity:
		res, err = e.client.DeleteEntity(ctx, ch.Key, ch.Base)
	case KindRelation:
		from, relType, to, ok := splitRelationKey(ch.Key)
		if !ok {
			return PushRecordResult{}, fmt.Errorf("internal: malformed relation key %q", ch.Key)
		}
		res, err = e.client.DeleteRelation(ctx, from, relType, to, ch.Base)
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

// recordPush updates the index on a successful upsert and classifies the result.
// On Applied, the index records the new hash; we prefer the server's returned
// hash (ETag) but fall back to the locally computed hash, which must match
// (canonical hashing is shared) — if the server returned nothing, the local hash
// is the agreed baseline.
func (e *Engine) recordPush(key, localHash string, res *PushResult) PushRecordResult {
	if res.Applied {
		agreed := res.Hash
		if agreed == "" {
			agreed = localHash
		}
		e.idx.Set(key, agreed)
		return PushRecordResult{Key: key, Outcome: OutcomePushed}
	}
	return classifyConflict(key, res)
}

// classifyConflict turns a non-applied PushResult into a halt report entry.
func classifyConflict(key string, res *PushResult) PushRecordResult {
	switch {
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
