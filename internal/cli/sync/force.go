package sync

import (
	"context"
	"errors"
	"fmt"
)

// ErrForceUnknownRecord is returned by a force operation whose id is neither a
// local record (push --force) nor a remote record (pull --force), so there is
// nothing to resolve. The caller surfaces it as a clear error with no partial
// state.
var ErrForceUnknownRecord = errors.New("no such record to force")

// ForcePush resolves a conflict in favor of the LOCAL copy: it overwrites the
// remote record with the working-copy content regardless of the remote's
// current hash, then re-baselines the index to the new agreed hash. The id is a
// record key (entity id, or "from/type/to" for a relation).
//
// Force still uses a conditional request, but supplies the remote's CURRENT hash
// as If-Match (re-read first) rather than the stale index base — so it overwrites
// the conflicting remote rather than blindly clobbering with no precondition.
// This keeps the server's "no blind writes" invariant intact while letting the
// operator deliberately win.
func (e *Engine) ForcePush(ctx context.Context, key string) (*PushRecordResult, error) {
	snap, _, err := SnapshotLocal(ctx, e.store)
	if err != nil {
		return nil, err
	}
	rec, ok := snap.Records[key]
	if !ok {
		// Not present locally: the operator may mean "delete it on the remote
		// too" — but that is `push` of a local deletion, not force. A force-push
		// of a non-existent local record is an error (no partial state).
		return nil, fmt.Errorf("%w: %q is not a local record (nothing to push)", ErrForceUnknownRecord, key)
	}

	remoteHash, err := e.remoteHash(ctx, rec.Kind, key)
	if err != nil {
		return nil, err
	}

	res, err := e.pushUpsert(ctx, LocalChange{Record: rec, Key: key, Kind: rec.Kind, Base: remoteHash})
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// ForcePull resolves a conflict in favor of the REMOTE copy: it fetches the
// remote record and applies it locally, overwriting the working copy, then
// re-baselines the index to the remote hash. A remote tombstone (record absent
// remotely) mirrors as a local delete.
func (e *Engine) ForcePull(ctx context.Context, key string) (*PullRecordResult, error) {
	if e.applier == nil {
		return nil, errRemoteApplierRequired
	}
	kind := kindFromKey(key)

	res, err := e.applyRemote(ctx, kind, key)
	if err != nil {
		if errors.Is(err, errRemoteAbsent) {
			// Remote no longer has it → mirror the delete locally.
			if derr := e.deleteLocal(ctx, kind, key); derr != nil {
				return nil, derr
			}
			e.idx.Delete(key)
			return &PullRecordResult{Key: key, Outcome: OutcomePulledDelete}, nil
		}
		return nil, err
	}
	return res, nil
}

// remoteHash reads a record's current server hash (ETag) for use as the If-Match
// of a forced push. A 404 (absent remotely) yields an empty hash, which the
// server treats as a first-create precondition — the correct base for pushing a
// local record the remote does not have.
func (e *Engine) remoteHash(ctx context.Context, kind Kind, key string) (string, error) {
	switch kind {
	case KindEntity:
		fe, err := e.client.GetEntity(ctx, key)
		if err != nil {
			if isNotFound(err) {
				return "", nil
			}
			return "", err
		}
		return fe.Hash, nil
	default:
		from, relType, to, ok := splitRelationKey(key)
		if !ok {
			return "", fmt.Errorf("internal: malformed relation key %q", key)
		}
		fr, err := e.client.GetRelation(ctx, from, relType, to)
		if err != nil {
			if isNotFound(err) {
				return "", nil
			}
			return "", err
		}
		return fr.Hash, nil
	}
}
