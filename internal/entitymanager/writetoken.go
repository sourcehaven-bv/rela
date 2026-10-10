package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Writes that return the caller's token (TKT-SM20FG, D2/D4). A sync loop
// writes the merged state and then moves its base tag onto exactly that
// state. Tagging "the current state" after the write would tag a user edit
// that landed in between as the base, and the next merge would lose it. So
// the write returns the token of the row it wrote, as the caller's reader
// sees it, and the tag compares against that token.
//
// The token is never [store.VersionOf] of the in-memory input: a backend
// decodes values into other Go types (an int written comes back int64), and
// the token covers the type. The written row is read back inside the write's
// transaction instead, which on every backend holds off other writers of
// that row ([writeWithUniqueCheck] runs the write in a Tx whenever a capture
// is requested).
//
// On [VersionTags], not [Manager]: Manager sits on its plimsoll method load
// line, and every runtime that may tag gets the tag writer.

// staleWriteToken is returned in place of a token when the row changed
// after the write, before the caller's reader could read it (a cascade or
// another writer). It never equals a [store.VersionOf] token, so a tag
// expecting it reports a conflict and the caller re-reads.
const staleWriteToken store.EntityVersion = "stale"

// writeCapture is one [VersionTags.WriteAsSeen] request riding ctx: the
// caller's expectation for the first patch, and the first entity row a write
// stores.
type writeCapture struct {
	expect store.EntityVersion
	view   CallerView

	mu      sync.Mutex
	claimed bool // the first PatchEntity took the expectation
	done    bool
	row     *entity.Entity
}

type writeCaptureKey struct{}

// captureFrom returns the capture on ctx, or nil.
func captureFrom(ctx context.Context) *writeCapture {
	c, _ := ctx.Value(writeCaptureKey{}).(*writeCapture)
	return c
}

// claimExpectation hands the caller's expectation to the first PatchEntity
// under ctx and to no later one: automations and cascades reuse ctx for
// writes to other entities, which the caller expected nothing of.
func claimExpectation(ctx context.Context) (store.EntityVersion, CallerView, bool) {
	c := captureFrom(ctx)
	if c == nil {
		return "", nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.claimed {
		return "", nil, false
	}
	c.claimed = true
	return c.expect, c.view, c.expect != ""
}

// writeCaptureFor returns the pending capture a write of e satisfies, or
// nil. Nested writes share ctx and run after the first write, so only the
// first write is captured.
func writeCaptureFor(ctx context.Context, _ *entity.Entity) *writeCapture {
	c := captureFrom(ctx)
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.done {
		return nil
	}
	return c
}

// record reads e's row back through view, the write's transaction.
func (c *writeCapture) record(ctx context.Context, view store.Store, e *entity.Entity) error {
	row, err := view.GetEntity(ctx, e.Ref())
	if err != nil {
		return fmt.Errorf("entitymanager: read back %s: %w", e.Ref(), err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.done {
		c.done, c.row = true, row
	}
	return nil
}

// written returns the captured row, or nil when no write was captured.
func (c *writeCapture) written() *entity.Entity {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.row
}

// WriteTokenFunc returns the caller's token of the row a write stored. Pass
// the entity the write returned.
type WriteTokenFunc = func(result *entity.Entity) (store.EntityVersion, error)

// WriteAsSeen prepares ctx for ONE entity write, made through whichever
// [Manager] handle the caller writes with (so its field gate and elevation
// apply unchanged), and returns the function that yields the written row's
// token as view reads it.
//
// When expect is set, the first [Manager.PatchEntity] under the returned
// ctx is a compare-and-set on the caller's view, as in
// [VersionTags.TagCurrent]: it applies only while view's read of the face
// has token expect, and a mismatch is a *[store.VersionConflictError] with
// nothing written. The store write is conditional on the raw row read before
// the compare, so a write landing after that read conflicts too. It is not
// retried: the caller merged against what it read, so a moved row needs a
// new merge, not a replay. Creates ignore expect.
//
// view is required.
func (v *VersionTags) WriteAsSeen(
	ctx context.Context, expect store.EntityVersion, view CallerView,
) (writeCtx context.Context, token WriteTokenFunc) {
	c := &writeCapture{expect: expect, view: view}
	token = func(result *entity.Entity) (store.EntityVersion, error) {
		if view == nil {
			return "", errors.New("entitymanager: WriteAsSeen needs the caller's view")
		}
		return tokenAsSeen(ctx, v.m.deps.Store, c.written(), result, view)
	}
	return context.WithValue(ctx, writeCaptureKey{}, c), token
}

// tokenAsSeen is the caller's token of written, the row captured inside the
// write. The caller's reader reads the face, then the raw row is read again:
// when the raw row still equals written, nothing landed between the write
// and the reader's read, so the reader saw written. Otherwise the token is
// [staleWriteToken].
func tokenAsSeen(
	ctx context.Context, st store.Store, written, result *entity.Entity, view CallerView,
) (store.EntityVersion, error) {
	if written == nil || result == nil {
		return staleWriteToken, nil
	}
	ref := written.Ref()
	if result.ID != written.ID {
		// Defensive: the capture took another entity's write.
		return staleWriteToken, nil
	}
	seen, err := view(ctx, ref)
	if errors.Is(err, store.ErrNotFound) {
		// The caller wrote a row it cannot read back; it has no token.
		return staleWriteToken, nil
	}
	if err != nil {
		return "", fmt.Errorf("entitymanager: read %s: %w", ref, err)
	}
	now, err := st.GetEntity(ctx, ref)
	if errors.Is(err, store.ErrNotFound) {
		return staleWriteToken, nil
	}
	if err != nil {
		return "", fmt.Errorf("entitymanager: read %s: %w", ref, err)
	}
	if store.VersionOf(now) != store.VersionOf(written) {
		return staleWriteToken, nil
	}
	return store.VersionOf(seen), nil
}
