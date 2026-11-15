package idproles_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/idproles"
	"github.com/Sourcehaven-BV/rela/internal/lock"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func newKV(t *testing.T) state.KV {
	t.Helper()
	mem := storage.NewMemFS()
	rfs, err := storage.NewRootedFS(mem, "/")
	require.NoError(t, err)
	return state.NewFSKV(rfs)
}

func newStore(t *testing.T) (*idproles.Store, state.KV) {
	t.Helper()
	kv := newKV(t)
	s, err := idproles.New(kv, lock.NewMemoryLocker())
	require.NoError(t, err)
	return s, kv
}

func TestNew_RejectsNil(t *testing.T) {
	t.Parallel()
	_, err := idproles.New(nil, lock.NewMemoryLocker())
	require.Error(t, err)
	_, err = idproles.New(newKV(t), nil)
	require.Error(t, err)
}

func TestSet(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		steps   [][]string // successive Set calls for subject "u1"
		want    []string
		wantErr string
	}{
		{name: "stores roles", steps: [][]string{{"cpo", "fg"}}, want: []string{"cpo", "fg"}},
		{name: "trims and deduplicates", steps: [][]string{{" cpo ", "cpo", "fg"}}, want: []string{"cpo", "fg"}},
		{name: "replaces", steps: [][]string{{"cpo"}, {"fg"}}, want: []string{"fg"}},
		{name: "empty list clears", steps: [][]string{{"cpo"}, {}}, want: nil},
		{name: "clearing an absent subject is a no-op", steps: [][]string{{}}, want: nil},
		{name: "blank role refused", steps: [][]string{{"cpo", "  "}}, wantErr: "role must not be empty"},
		{name: "control character refused", steps: [][]string{{"c\x01po"}}, wantErr: "control character"},
		{name: "over-long role refused", steps: [][]string{{strings.Repeat("r", idproles.MaxNameLen+1)}},
			wantErr: "longer than"},
		{name: "invalid utf-8 refused", steps: [][]string{{"\xff"}}, wantErr: "UTF-8"},
		{name: "too many roles refused", steps: [][]string{manyRoles(idproles.MaxRoles + 1)}, wantErr: "exceed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, _ := newStore(t)
			ctx := t.Context()
			var err error
			for _, roles := range tc.steps {
				if err = s.Set(ctx, "u1", "org1", "PER-1", roles); err != nil {
					break
				}
			}
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				got, rerr := s.RolesFor(ctx, "u1", "PER-1")
				require.NoError(t, rerr)
				require.Empty(t, got, "a refused Set stores nothing")
				return
			}
			require.NoError(t, err)
			got, err := s.RolesFor(ctx, "u1", "PER-1")
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestSet_RefusesBadSubjectAndOrg(t *testing.T) {
	t.Parallel()
	s, _ := newStore(t)
	require.ErrorContains(t, s.Set(t.Context(), "", "org", "PER-1", []string{"cpo"}), "subject must not be empty")
	require.ErrorContains(t, s.Set(t.Context(), "u\x00", "org", "PER-1", []string{"cpo"}), "control character")
	require.ErrorContains(t, s.Set(t.Context(), "u", "o\n", "PER-1", []string{"cpo"}), "control character")
	require.NoError(t, s.Set(t.Context(), "u", "", "PER-1", []string{"cpo"}), "an org id is optional")
	require.ErrorContains(t, s.Set(t.Context(), "u", "", "", []string{"cpo"}), "entity id must not be empty")
	require.NoError(t, s.Set(t.Context(), "u", "", "", nil), "a clear needs no entity")
}

func TestRolesFor_BoundToTheEntity(t *testing.T) {
	t.Parallel()
	s, _ := newStore(t)
	ctx := t.Context()
	require.NoError(t, s.Set(ctx, "u1", "o", "PER-1", []string{"cpo"}))
	got, err := s.RolesFor(ctx, "u1", "PER-1")
	require.NoError(t, err)
	require.Equal(t, []string{"cpo"}, got)
	got, err = s.RolesFor(ctx, "u1", "PER-2")
	require.NoError(t, err)
	require.Nil(t, got, "another entity holding the subject gets nothing")
}

func TestForgetEntity(t *testing.T) {
	t.Parallel()
	s, _ := newStore(t)
	ctx := t.Context()
	require.NoError(t, s.Set(ctx, "a@example.org", "", "PER-1", []string{"cpo"}))
	require.NoError(t, s.Set(ctx, "b@example.org", "", "PER-1", []string{"fg"}))
	require.NoError(t, s.Set(ctx, "c@example.org", "", "PER-2", []string{"cpo"}))

	n, err := s.ForgetEntity(ctx, "PER-1")
	require.NoError(t, err)
	require.Equal(t, 2, n)
	got, err := s.RolesFor(ctx, "a@example.org", "PER-1")
	require.NoError(t, err)
	require.Empty(t, got)
	got, err = s.RolesFor(ctx, "c@example.org", "PER-2")
	require.NoError(t, err)
	require.Equal(t, []string{"cpo"}, got, "another entity's record stays")

	n, err = s.ForgetEntity(ctx, "PER-1")
	require.NoError(t, err)
	require.Zero(t, n, "forgetting twice is a no-op")

	_, err = s.ForgetEntity(ctx, "")
	require.Error(t, err)
}

func TestHoldersWithAny(t *testing.T) {
	t.Parallel()
	s, _ := newStore(t)
	ctx := t.Context()
	require.NoError(t, s.Set(ctx, "u2", "o", "PER-2", []string{"cpo"}))
	require.NoError(t, s.Set(ctx, "u1", "o", "PER-1", []string{"fg", "cpo"}))
	require.NoError(t, s.Set(ctx, "u3", "o", "PER-3", []string{"member"}))

	h := func(n string) idproles.Holder { return idproles.Holder{Subject: "u" + n, EntityID: "PER-" + n} }
	tests := []struct {
		name   string
		claims []string
		want   []idproles.Holder
	}{
		{name: "one claim", claims: []string{"cpo"}, want: []idproles.Holder{h("1"), h("2")}},
		{name: "any of several", claims: []string{"fg", "member"}, want: []idproles.Holder{h("1"), h("3")}},
		{name: "no holder", claims: []string{"admin"}, want: nil},
		{name: "no claims", claims: nil, want: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := s.HoldersWithAny(ctx, tc.claims)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestHold_SerializesASubject pins the per-subject lock the webhook receiver
// takes around its fetch and store: a second holder waits for the first.
func TestHold_SerializesASubject(t *testing.T) {
	t.Parallel()
	s, _ := newStore(t)
	ctx := t.Context()
	release, err := s.Hold(ctx, "u1")
	require.NoError(t, err)
	other, err := s.Hold(ctx, "u2")
	require.NoError(t, err, "another subject is not blocked")
	other()
	require.NoError(t, s.Set(ctx, "u1", "o", "PER-1", []string{"cpo"}), "Set does not take the subject lock")

	done := make(chan struct{})
	go func() {
		r, herr := s.Hold(ctx, "u1")
		if herr == nil {
			r()
		}
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("a second Hold of the same subject did not wait")
	case <-time.After(20 * time.Millisecond):
	}
	release()
	<-done
	_, err = s.Hold(ctx, "")
	require.ErrorContains(t, err, "subject must not be empty")
}

func TestDamagedDocumentIsAnError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{name: "not json", doc: "{not json", want: "damaged"},
		{name: "unknown version", doc: `{"version":2,"subjects":{}}`, want: "unknown version"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, kv := newStore(t)
			ctx := t.Context()
			require.NoError(t, kv.Put(ctx, idproles.StateKey, []byte(tc.doc)))
			_, err := s.RolesFor(ctx, "u1", "PER-1")
			require.ErrorContains(t, err, tc.want)
			require.ErrorContains(t, s.Set(ctx, "u1", "", "PER-1", []string{"cpo"}), tc.want,
				"a write must not replace a damaged document and erase other subjects")
			raw, err := kv.Get(ctx, idproles.StateKey)
			require.NoError(t, err)
			require.Equal(t, tc.doc, string(raw))
		})
	}
}

// TestConcurrentSetsLoseNothing pins the lock: every concurrent writer's
// subject survives.
func TestConcurrentSetsLoseNothing(t *testing.T) {
	t.Parallel()
	s, _ := newStore(t)
	ctx := t.Context()
	const n = 20
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			require.NoError(t, s.Set(ctx, fmt.Sprintf("u%02d", i), "o", "PER-1", []string{"cpo"}))
		})
	}
	wg.Wait()
	got, err := s.HoldersWithAny(ctx, []string{"cpo"})
	require.NoError(t, err)
	require.Len(t, got, n)
}

func TestLockFailureIsReturned(t *testing.T) {
	t.Parallel()
	s, err := idproles.New(newKV(t), failingLocker{})
	require.NoError(t, err)
	require.ErrorContains(t, s.Set(t.Context(), "u", "", "PER-1", []string{"cpo"}), "lock")
	_, err = s.RolesFor(t.Context(), "u", "PER-1")
	require.ErrorContains(t, err, "lock")
	_, err = s.Hold(t.Context(), "u")
	require.ErrorContains(t, err, "lock")
}

func TestKVFailureIsReturned(t *testing.T) {
	t.Parallel()
	s, err := idproles.New(failingKV{}, lock.NewMemoryLocker())
	require.NoError(t, err)
	_, err = s.RolesFor(t.Context(), "u", "PER-1")
	require.ErrorContains(t, err, "read")
	require.ErrorContains(t, s.Set(t.Context(), "u", "", "PER-1", []string{"cpo"}), "read")
}

type failingLocker struct{}

func (failingLocker) Acquire(context.Context, string) (func(), error) {
	return nil, errors.New("boom")
}

type failingKV struct{}

func (failingKV) Get(context.Context, string) ([]byte, error) { return nil, errors.New("boom") }
func (failingKV) Put(context.Context, string, []byte) error   { return errors.New("boom") }

func manyRoles(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("r%d", i)
	}
	return out
}
