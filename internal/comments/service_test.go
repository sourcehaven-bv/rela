package comments_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/comments/memcomments"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

var testBase = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func newService(t *testing.T) (*comments.Service, comments.Store) {
	t.Helper()
	st := memcomments.New()
	svc, err := comments.NewService(st, func() time.Time { return testBase })
	require.NoError(t, err)
	return svc, st
}

func aliceCtx() context.Context {
	return principal.With(context.Background(),
		principal.Principal{User: "alice@example.com", Tool: "data-entry"})
}

func propAnchor() comments.Anchor {
	return comments.Anchor{Kind: comments.AnchorProperty, Ref: "status"}
}

func TestNewService_RequiresStore(t *testing.T) {
	_, err := comments.NewService(nil, nil)
	require.Error(t, err, "a Service with no store would discard every comment")
}

// TestAdd_StampsAuthorFromPrincipal is the ticket's key test (AC3). The wire
// type cannot express an author, so the only thing that can set one is the
// service — but this pins the resulting value, not just the type shape.
func TestAdd_StampsAuthorFromPrincipal(t *testing.T) {
	svc, _ := newService(t)

	got, err := svc.Add(aliceCtx(), comments.Target{Type: "ticket", ID: "TKT-1"},
		comments.AddRequest{Anchor: propAnchor(), Body: "a remark"})
	require.NoError(t, err)

	require.Equal(t, "alice@example.com", got.Author)
	require.True(t, testBase.Equal(got.CreatedAt))
	require.NotEmpty(t, got.ID, "the server mints the id")
}

// TestAdd_MintsUniqueIDs pins that ids are server-generated and distinct, so a
// caller cannot overwrite one comment by reusing another's id.
func TestAdd_MintsUniqueIDs(t *testing.T) {
	svc, _ := newService(t)
	ctx := aliceCtx()
	tgt := comments.Target{Type: "ticket", ID: "TKT-1"}

	seen := map[string]bool{}
	for range 50 {
		c, err := svc.Add(ctx, tgt, comments.AddRequest{Anchor: propAnchor(), Body: "x"})
		require.NoError(t, err)
		require.False(t, seen[c.ID], "minted ids must not repeat")
		seen[c.ID] = true
	}
}

// TestAdd_RefusesUnstampedPrincipal pins that an unattributable comment is
// refused rather than stored as "unknown". A comment nobody is recorded as
// having written can never satisfy an *-own check, so its author could neither
// edit nor delete it.
func TestAdd_RefusesUnstampedPrincipal(t *testing.T) {
	svc, st := newService(t)

	_, err := svc.Add(context.Background(), comments.Target{Type: "ticket", ID: "TKT-1"},
		comments.AddRequest{Anchor: propAnchor(), Body: "a remark"})
	require.ErrorIs(t, err, comments.ErrUnknownAuthor)

	stored, err := st.List(context.Background(), comments.Target{ID: "TKT-1"})
	require.NoError(t, err)
	require.Empty(t, stored, "nothing is stored when the author cannot be resolved")
}

func TestAdd_RefusesReservedPrincipal(t *testing.T) {
	svc, _ := newService(t)
	ctx := principal.With(context.Background(),
		principal.Principal{User: "system:version-sweep", Tool: "internal"})

	_, err := svc.Add(ctx, comments.Target{Type: "ticket", ID: "TKT-1"},
		comments.AddRequest{Anchor: propAnchor(), Body: "x"})
	require.ErrorIs(t, err, comments.ErrUnknownAuthor)
}

func TestAdd_ValidatesAnchor(t *testing.T) {
	svc, _ := newService(t)
	ctx := aliceCtx()
	tgt := comments.Target{Type: "ticket", ID: "TKT-1"}

	t.Run("unknown kind", func(t *testing.T) {
		_, err := svc.Add(ctx, tgt, comments.AddRequest{
			Anchor: comments.Anchor{Kind: "wat", Ref: "x"}, Body: "b"})
		require.ErrorIs(t, err, comments.ErrInvalidAnchor)
	})

	t.Run("empty ref", func(t *testing.T) {
		_, err := svc.Add(ctx, tgt, comments.AddRequest{
			Anchor: comments.Anchor{Kind: comments.AnchorProperty, Ref: "  "}, Body: "b"})
		require.ErrorIs(t, err, comments.ErrInvalidAnchor)
	})

	t.Run("section anchor accepted", func(t *testing.T) {
		_, err := svc.Add(ctx, tgt, comments.AddRequest{
			Anchor: comments.Anchor{Kind: comments.AnchorSection, Ref: "acceptance-criteria"},
			Body:   "b"})
		require.NoError(t, err)
	})
}

// TestAdd_EnforcesPerTargetCap pins the resource bound: the file backend reads
// a target's whole thread on every List, so an unbounded thread is a slow leak.
func TestAdd_EnforcesPerTargetCap(t *testing.T) {
	svc, st := newService(t)
	ctx := aliceCtx()
	tgt := comments.Target{Type: "ticket", ID: "TKT-1"}

	for i := range comments.MaxPerTarget {
		require.NoError(t, st.Add(ctx, tgt, comments.Comment{
			ID:        string(rune('a'+i%26)) + strings.Repeat("x", i%7+1),
			Author:    "alice@example.com",
			CreatedAt: testBase.Add(time.Duration(i) * time.Second),
			Anchor:    propAnchor(),
			Body:      "filler",
		}))
	}

	_, err := svc.Add(ctx, tgt, comments.AddRequest{Anchor: propAnchor(), Body: "one too many"})
	require.ErrorIs(t, err, comments.ErrTooManyComments)
}

func TestValidateBody(t *testing.T) {
	tests := []struct {
		name string
		body string
		want error
	}{
		{"plain prose", "a normal comment", nil},
		{"newlines and tabs allowed", "line one\nline\ttwo", nil},
		{"markdown allowed", "**bold** and `code`", nil},
		{"unicode allowed", "emoji 🎉 and 日本語", nil},
		{"empty", "", comments.ErrEmptyBody},
		{"whitespace only", "   \n\t ", comments.ErrEmptyBody},
		{"too long", strings.Repeat("x", comments.MaxBodyBytes+1), comments.ErrBodyTooLong},
		{"at the limit", strings.Repeat("x", comments.MaxBodyBytes), nil},
		{"NUL byte", "before\x00after", comments.ErrBodyControlChars},
		{"escape byte", "before\x1bafter", comments.ErrBodyControlChars},
		{"DEL byte", "before\x7fafter", comments.ErrBodyControlChars},
		{"carriage return", "before\rafter", comments.ErrBodyControlChars},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := comments.ValidateBody(tc.body)
			if tc.want == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestAdd_RejectsInvalidBody(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.Add(aliceCtx(), comments.Target{Type: "ticket", ID: "TKT-1"},
		comments.AddRequest{Anchor: propAnchor(), Body: "  "})
	require.ErrorIs(t, err, comments.ErrEmptyBody)
}

func TestUpdate_ValidatesBody(t *testing.T) {
	svc, _ := newService(t)
	ctx := aliceCtx()
	tgt := comments.Target{Type: "ticket", ID: "TKT-1"}

	c, err := svc.Add(ctx, tgt, comments.AddRequest{Anchor: propAnchor(), Body: "original"})
	require.NoError(t, err)

	require.ErrorIs(t, svc.Update(ctx, tgt, c.ID, "", false), comments.ErrEmptyBody)

	// The original survives a rejected update.
	got, err := svc.Get(ctx, tgt, c.ID)
	require.NoError(t, err)
	require.Equal(t, "original", got.Body)
}

func TestGet_ReportsNotFound(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.Get(aliceCtx(), comments.Target{Type: "ticket", ID: "TKT-1"}, "nope")
	require.ErrorIs(t, err, comments.ErrNotFound)
}

// countingStore records which read each call takes.
//
// The delegate is a named field rather than an embedded interface: embedding
// would make this satisfy comments.Store however the interface grows, turning a
// forgotten method into a nil-dereference at call time instead of a build
// failure here.
type countingStore struct {
	inner comments.Store
	lists int
	gets  int
}

var _ comments.Store = (*countingStore)(nil)

func (s *countingStore) List(ctx context.Context, target comments.Target) ([]comments.Comment, error) {
	s.lists++
	return s.inner.List(ctx, target)
}

func (s *countingStore) Get(ctx context.Context, target comments.Target, id string) (comments.Comment, error) {
	s.gets++
	return s.inner.Get(ctx, target, id)
}

func (s *countingStore) Add(ctx context.Context, target comments.Target, c comments.Comment) error {
	return s.inner.Add(ctx, target, c)
}

func (s *countingStore) Update(
	ctx context.Context, target comments.Target, id, body string, resolved bool,
) error {
	return s.inner.Update(ctx, target, id, body, resolved)
}

func (s *countingStore) SetResolved(
	ctx context.Context, target comments.Target, id string, resolved bool,
) (bool, error) {
	return s.inner.SetResolved(ctx, target, id, resolved)
}

func (s *countingStore) Delete(ctx context.Context, target comments.Target, id string) error {
	return s.inner.Delete(ctx, target, id)
}

func (s *countingStore) DeleteTarget(ctx context.Context, target comments.Target) error {
	return s.inner.DeleteTarget(ctx, target)
}

func (s *countingStore) DeleteAllFaces(ctx context.Context, entityID string) error {
	return s.inner.DeleteAllFaces(ctx, entityID)
}

func (s *countingStore) Rename(ctx context.Context, oldID, newID string) error {
	return s.inner.Rename(ctx, oldID, newID)
}

// TestGet_DoesNotReadTheWholeThread is the point of TKT-4LG36M, and it needs a
// test because the regression is invisible: a Get reimplemented as a
// list-and-scan returns exactly the right comment, so every other test here
// still passes while each authorization check drags up to MaxPerTarget bodies
// out of the database to read one author field.
//
// Counting the calls is the only way to observe the difference from outside,
// which is the same reason the store's read paths are pinned with
// storetest.Counting budgets.
func TestGet_DoesNotReadTheWholeThread(t *testing.T) {
	st := &countingStore{inner: memcomments.New()}
	svc, err := comments.NewService(st, func() time.Time { return testBase })
	require.NoError(t, err)

	ctx := aliceCtx()
	tgt := comments.Target{Type: "ticket", ID: "TKT-1"}
	var id string
	for i := range 5 {
		c, addErr := svc.Add(ctx, tgt, comments.AddRequest{
			Anchor: propAnchor(),
			Body:   "comment " + strconv.Itoa(i),
		})
		require.NoError(t, addErr)
		id = c.ID
	}

	st.lists, st.gets = 0, 0
	got, err := svc.Get(ctx, tgt, id)
	require.NoError(t, err)
	require.Equal(t, id, got.ID)

	require.Equal(t, 1, st.gets, "one comment must cost one single-row read")
	require.Zero(t, st.lists, "reading one comment must not list the thread")
}

// TestEntityRenamed_ReKeysComments pins the critical design-review finding
// (RR-FCUS1V): rename emits exactly one callback, so without this every comment
// on a renamed entity would be filed under an id nothing resolves to.
func TestEntityRenamed_ReKeysComments(t *testing.T) {
	svc, st := newService(t)
	ctx := aliceCtx()

	_, err := svc.Add(ctx, comments.Target{Type: "ticket", ID: "TKT-old"},
		comments.AddRequest{Anchor: propAnchor(), Body: "still relevant"})
	require.NoError(t, err)

	require.NoError(t, svc.EntityRenamed(ctx, "TKT-old", "TKT-new"))

	moved, err := st.List(ctx, comments.Target{ID: "TKT-new"})
	require.NoError(t, err)
	require.Len(t, moved, 1, "comments follow the rename")
	require.Equal(t, "still relevant", moved[0].Body)

	old, err := st.List(ctx, comments.Target{ID: "TKT-old"})
	require.NoError(t, err)
	require.Empty(t, old)
}

func TestEntityRenamed_IgnoresNoOps(t *testing.T) {
	svc, _ := newService(t)
	require.NoError(t, svc.EntityRenamed(aliceCtx(), "TKT-1", "TKT-1"))
}

// TestEntityDeleted_DropsComments pins AC9. ID reuse is permitted in rela, so a
// later entity taking the same id must not inherit the previous occupant's
// comments and present someone else's remarks as its own.
func TestEntityDeleted_DropsComments(t *testing.T) {
	svc, st := newService(t)
	ctx := aliceCtx()

	_, err := svc.Add(ctx, comments.Target{Type: "ticket", ID: "TKT-1"},
		comments.AddRequest{Anchor: propAnchor(), Body: "gone soon"})
	require.NoError(t, err)

	require.NoError(t, svc.EntityDeleted(ctx, "TKT-1"))

	got, err := st.List(ctx, comments.Target{ID: "TKT-1"})
	require.NoError(t, err)
	require.Empty(t, got)
}

// TestEntityFaceDeleted_DropsOnlyThatFace pins BUG-R1PQY9's delete half: a face
// delete used to fire no notification, so the face's thread outlived it and a
// face of the same name created later inherited it. The sibling faces'
// threads must survive, because their content still exists.
func TestEntityFaceDeleted_DropsOnlyThatFace(t *testing.T) {
	svc, st := newService(t)
	ctx := aliceCtx()

	for _, face := range []entity.Face{"", "draft", "published"} {
		_, err := svc.Add(ctx, comments.Target{Type: "ticket", ID: "TKT-1", Face: face},
			comments.AddRequest{Anchor: propAnchor(), Body: "on " + string(face)})
		require.NoError(t, err)
	}

	require.NoError(t, svc.EntityFaceDeleted(ctx, "TKT-1", "draft"))

	gone, err := st.List(ctx, comments.Target{ID: "TKT-1", Face: "draft"})
	require.NoError(t, err)
	require.Empty(t, gone)
	for _, face := range []entity.Face{"", "published"} {
		kept, err := st.List(ctx, comments.Target{ID: "TKT-1", Face: face})
		require.NoError(t, err)
		require.Len(t, kept, 1, "face %q keeps its thread", face)
	}
}

// TestFaceMoved pins the thread half of a data-migration face move
// (BUG-6OZBP9): the content moved, so its comments move with it.
func TestFaceMoved(t *testing.T) {
	src := comments.Target{Type: "ticket", ID: "TKT-1"}
	dst := comments.Target{Type: "ticket", ID: "TKT-1", Face: "draft"}

	seed := func(t *testing.T) (*comments.Service, comments.Store, comments.Comment) {
		t.Helper()
		svc, st := newService(t)
		c, err := svc.Add(aliceCtx(), src, comments.AddRequest{Anchor: propAnchor(), Body: "before faces"})
		require.NoError(t, err)
		require.NoError(t, svc.Update(aliceCtx(), src, c.ID, "edited", true))
		got, err := st.Get(aliceCtx(), src, c.ID)
		require.NoError(t, err)
		return svc, st, got
	}

	t.Run("moves the thread with every field intact", func(t *testing.T) {
		svc, st, want := seed(t)
		require.NoError(t, svc.FaceMoved(aliceCtx(), "ticket", "TKT-1", "", "draft"))

		got, err := st.List(aliceCtx(), dst)
		require.NoError(t, err)
		require.Equal(t, []comments.Comment{want}, got)
		left, err := st.List(aliceCtx(), src)
		require.NoError(t, err)
		require.Empty(t, left)
	})

	t.Run("is idempotent", func(t *testing.T) {
		svc, st, _ := seed(t)
		require.NoError(t, svc.FaceMoved(aliceCtx(), "ticket", "TKT-1", "", "draft"))
		require.NoError(t, svc.FaceMoved(aliceCtx(), "ticket", "TKT-1", "", "draft"))
		got, err := st.List(aliceCtx(), dst)
		require.NoError(t, err)
		require.Len(t, got, 1)
	})

	t.Run("a half-finished move converges and keeps the destination's copy", func(t *testing.T) {
		svc, st, c := seed(t)
		// A crashed run copied the comment but never cleared the source.
		// The destination copy differs, so we can tell which one survived.
		copied := c
		copied.Body = "destination copy"
		require.NoError(t, st.Add(aliceCtx(), dst, copied))

		require.NoError(t, svc.FaceMoved(aliceCtx(), "ticket", "TKT-1", "", "draft"))

		got, err := st.List(aliceCtx(), dst)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, "destination copy", got[0].Body)
		left, err := st.List(aliceCtx(), src)
		require.NoError(t, err)
		require.Empty(t, left)
	})

	t.Run("merges into an occupied destination and leaves other faces alone", func(t *testing.T) {
		svc, st, _ := seed(t)
		_, err := svc.Add(aliceCtx(), dst, comments.AddRequest{Anchor: propAnchor(), Body: "already on draft"})
		require.NoError(t, err)
		other := comments.Target{Type: "ticket", ID: "TKT-1", Face: "published"}
		_, err = svc.Add(aliceCtx(), other, comments.AddRequest{Anchor: propAnchor(), Body: "on published"})
		require.NoError(t, err)

		require.NoError(t, svc.FaceMoved(aliceCtx(), "ticket", "TKT-1", "", "draft"))

		got, err := st.List(aliceCtx(), dst)
		require.NoError(t, err)
		require.Len(t, got, 2)
		kept, err := st.List(aliceCtx(), other)
		require.NoError(t, err)
		require.Len(t, kept, 1)
	})

	t.Run("a comment posted at the source during the move survives", func(t *testing.T) {
		_, inner, _ := seed(t)
		st := &postsDuringList{Store: inner, at: src}
		svc, err := comments.NewService(st, nil)
		require.NoError(t, err)

		require.NoError(t, svc.FaceMoved(aliceCtx(), "ticket", "TKT-1", "", "draft"))

		left, err := inner.List(aliceCtx(), src)
		require.NoError(t, err)
		require.Len(t, left, 1, "the late comment was deleted with the moved ones")
		require.Equal(t, "posted mid-move", left[0].Body)
	})

	t.Run("same face is a no-op", func(t *testing.T) {
		svc, st, _ := seed(t)
		require.NoError(t, svc.FaceMoved(aliceCtx(), "ticket", "TKT-1", "", ""))
		got, err := st.List(aliceCtx(), src)
		require.NoError(t, err)
		require.Len(t, got, 1)
	})
}

// postsDuringList adds a comment at target right after the first List of it
// returns, standing in for a user posting while FaceMoved runs.
type postsDuringList struct {
	comments.Store
	at     comments.Target
	posted bool
}

func (s *postsDuringList) List(ctx context.Context, target comments.Target) ([]comments.Comment, error) {
	list, err := s.Store.List(ctx, target)
	if err != nil || s.posted || target.Key() != s.at.Key() {
		return list, err
	}
	s.posted = true
	late := comments.Comment{ID: "late", Author: "bob@example.com", CreatedAt: testBase,
		Anchor: propAnchor(), Body: "posted mid-move"}
	return list, s.Add(ctx, target, late)
}
