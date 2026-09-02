package comments

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// idBytes is the entropy in a minted comment ID. 10 bytes -> 16 base32
// characters, which is far more than a per-target thread needs; comment IDs
// are opaque handles, never typed by a human, so there is no reason to trade
// collision headroom for brevity the way entity IDs do.
const idBytes = 10

// idEncoding is unpadded lowercase base32 — case-insensitive and free of the
// characters that would complicate a URL path segment.
var idEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

// Clock supplies the current time. Injected rather than read from the wall
// clock so tests can pin CreatedAt and assert List's ordering deterministically
// — the same treatment userstate gives expiry, and for the same reason.
//
// Nil: never; [NewService] substitutes time.Now.
type Clock func() time.Time

// Service is the write path for comments: it stamps identity, mints IDs,
// validates input, and delegates persistence to a [Store].
//
// It is the only thing that constructs a [Comment], which is what makes the
// server-written fields trustworthy — there is no path where a client-supplied
// ID or Author reaches storage.
type Service struct {
	store Store
	now   Clock
}

// NewService returns a Service over st.
//
// Nil: st is required and rejected when nil — a Service with no store would
// silently discard every comment, and the failure would surface far from its
// cause. clock may be nil, in which case time.Now is used.
func NewService(st Store, clock Clock) (*Service, error) {
	if st == nil {
		return nil, errors.New("comments: NewService requires a Store")
	}
	if clock == nil {
		clock = time.Now
	}
	return &Service{store: st, now: clock}, nil
}

// AddRequest is the caller-supplied part of a new comment.
//
// Note what is absent: ID, Author and CreatedAt. They are not fields a caller
// may set, so they are not fields this struct carries — the wire type cannot
// express the forgery, rather than expressing it and having it stripped later.
type AddRequest struct {
	Anchor Anchor
	Body   string
}

// Add stores a new comment on target, attributed to the ctx principal.
//
// The author is taken from ctx and never from the request. An unstamped
// principal is refused ([ErrUnknownAuthor]) rather than recorded as "unknown":
// a comment nobody is recorded as having written can never satisfy an
// *-own permission check, so storing one creates a record its author can
// neither edit nor delete.
func (s *Service) Add(ctx context.Context, target Target, req AddRequest) (Comment, error) {
	author, err := authorFrom(ctx)
	if err != nil {
		return Comment{}, err
	}
	if vErr := req.Anchor.Validate(); vErr != nil {
		return Comment{}, vErr
	}
	if vErr := ValidateBody(req.Body); vErr != nil {
		return Comment{}, vErr
	}

	existing, err := s.store.List(ctx, target)
	if err != nil {
		return Comment{}, err
	}
	if len(existing) >= MaxPerTarget {
		return Comment{}, fmt.Errorf("%w: %d is the limit", ErrTooManyComments, MaxPerTarget)
	}

	id, err := mintID()
	if err != nil {
		return Comment{}, err
	}
	c := Comment{
		ID:        id,
		Author:    author,
		CreatedAt: s.now().UTC(),
		Anchor:    req.Anchor,
		Body:      req.Body,
	}
	if err := s.store.Add(ctx, target, c); err != nil {
		return Comment{}, err
	}
	return c, nil
}

// List returns the target's comments, oldest first.
//
// The ACL gate is the caller's job: this returns everything stored for the
// target. Callers on a served path must have resolved the target's read
// verdict and the comment:read permission first.
func (s *Service) List(ctx context.Context, target Target) ([]Comment, error) {
	return s.store.List(ctx, target)
}

// Get returns one comment by ID.
//
// It exists chiefly so a handler can resolve a comment's author *before*
// deciding whether an *-own permission covers the requested mutation.
func (s *Service) Get(ctx context.Context, target Target, id string) (Comment, error) {
	list, err := s.store.List(ctx, target)
	if err != nil {
		return Comment{}, err
	}
	for _, c := range list {
		if c.ID == id {
			return c, nil
		}
	}
	return Comment{}, ErrNotFound
}

// Update replaces a comment's body and resolved flag.
//
// Author, CreatedAt and Anchor are not editable: an edit must not be able to
// change who said something or what it was said about. Authorization —
// including whether the ctx principal owns this comment — is the caller's job.
func (s *Service) Update(ctx context.Context, target Target, id, body string, resolved bool) error {
	if err := ValidateBody(body); err != nil {
		return err
	}
	return s.store.Update(ctx, target, id, body, resolved)
}

// Delete removes one comment. Authorization is the caller's job.
func (s *Service) Delete(ctx context.Context, target Target, id string) error {
	return s.store.Delete(ctx, target, id)
}

// EntityPut implements [store.EntityObserver]. Comments key off the entity ID
// only, so an ordinary create or update changes nothing for them.
func (s *Service) EntityPut(*entity.Entity) error { return nil }

// EntityDelete implements [store.EntityObserver] by dropping the deleted
// entity's comments.
//
// Without this a later entity reusing the ID would inherit the previous
// occupant's comments — rela permits ID reuse, so this is reachable, not
// theoretical.
func (s *Service) EntityDelete(id string) error {
	return s.store.DeleteTarget(context.Background(), Target{ID: id})
}

// EntityRenamed implements [store.EntityObserver] by re-keying the renamed
// entity's comments.
//
// This callback is load-bearing: rename emits EXACTLY this one notification —
// never EntityDelete(oldID) followed by EntityPut — so a service that did not
// handle it would leave every comment filed under an ID nothing resolves to.
// The comments would still be on disk and completely unreachable.
func (s *Service) EntityRenamed(oldID string, renamed *entity.Entity) error {
	if renamed == nil || oldID == renamed.ID {
		return nil
	}
	return s.store.Rename(context.Background(), oldID, renamed.ID)
}

// authorFrom resolves the comment author from ctx.
//
// Uses [principal.Stamped] rather than From because the two cases differ here:
// From's unknown/unknown default is a reasonable audit-log placeholder but an
// unusable comment author (see [Service.Add]). A reserved system identity is
// refused for the same reason — a system job has no business authoring
// commentary in a human's thread.
func authorFrom(ctx context.Context) (string, error) {
	p, ok := principal.Stamped(ctx)
	if !ok {
		return "", ErrUnknownAuthor
	}
	user := strings.TrimSpace(p.User)
	if user == "" || user == "unknown" {
		return "", ErrUnknownAuthor
	}
	if principal.IsReserved(user) {
		return "", fmt.Errorf("%w: %q is a reserved system identity", ErrUnknownAuthor, user)
	}
	return user, nil
}

// mintID returns a fresh opaque comment ID.
func mintID() (string, error) {
	buf := make([]byte, idBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("comments: generating id: %w", err)
	}
	return idEncoding.EncodeToString(buf), nil
}

// SortComments orders comments oldest-first with the ID as a tie-break.
//
// Shared by every backend so ordering is defined in ONE place: a thread must
// render identically regardless of which backend served it, and two
// implementations sorting "the same way" independently is how that guarantee
// quietly stops holding. The ID tie-break matters because a clock with
// coarse resolution can stamp two comments in one tick.
func SortComments(list []Comment) {
	sort.SliceStable(list, func(i, j int) bool {
		if !list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].CreatedAt.Before(list[j].CreatedAt)
		}
		return list[i].ID < list[j].ID
	})
}
