package appbuild

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func commentsMeta(t *testing.T, enabled bool) *metamodel.Metamodel {
	t.Helper()
	src := `
version: "1"
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    properties:
      title:
        type: string
`
	if enabled {
		src += "comments:\n  enabled: true\n  on: [ticket]\n"
	}
	m, err := metamodel.Parse([]byte(src))
	require.NoError(t, err)
	return m
}

func paths(t *testing.T) *project.Context {
	t.Helper()
	root := t.TempDir()
	return &project.Context{Root: root, CacheDir: filepath.Join(root, project.CacheDir)}
}

// TestBuildComments_DisabledYieldsNilService pins AC1's wiring half: with no
// `comments:` block the service is genuinely nil, which is what lets the
// data-entry app decide not to serve the routes at all.
func TestBuildComments_DisabledYieldsNilService(t *testing.T) {
	p := paths(t)
	svc, err := buildComments(storage.NewOsFS(), p, commentsMeta(t, false))
	require.NoError(t, err)
	require.Nil(t, svc, "a nil service IS the disabled signal")

	_, statErr := os.Stat(filepath.Join(p.CacheDir, commentsDirName))
	require.ErrorIs(t, statErr, os.ErrNotExist, "a disabled feature creates no storage")
}

func TestBuildComments_EnabledYieldsService(t *testing.T) {
	svc, err := buildComments(storage.NewOsFS(), paths(t), commentsMeta(t, true))
	require.NoError(t, err)
	require.NotNil(t, svc)
}

// TestBuildComments_EnabledWithoutPathsFails pins that a configured feature
// with nowhere to store data is a wiring error, not a silent downgrade —
// otherwise an operator who switched commenting on would find it quietly
// missing.
func TestBuildComments_EnabledWithoutPathsFails(t *testing.T) {
	t.Run("nil paths", func(t *testing.T) {
		_, err := buildComments(storage.NewOsFS(), nil, commentsMeta(t, true))
		require.Error(t, err)
	})

	t.Run("nil filesystem", func(t *testing.T) {
		_, err := buildComments(nil, paths(t), commentsMeta(t, true))
		require.Error(t, err)
	})
}

// recordingRewriter captures the notifications it receives.
//
// The interface assertion is the compile-time guard that a fan-out subscriber
// is substitutable for the entitymanager's single-rewriter field.
var _ entitymanager.AliasRewriter = (*recordingRewriter)(nil)

type recordingRewriter struct {
	renamed []string
	deleted []string
	err     error
}

func (r *recordingRewriter) EntityRenamed(_ context.Context, oldID, newID string) error {
	r.renamed = append(r.renamed, oldID+"->"+newID)
	return r.err
}

func (r *recordingRewriter) EntityDeleted(_ context.Context, id string) error {
	r.deleted = append(r.deleted, id)
	return r.err
}

// TestAliasFanout_NilWhenNothingSubscribes pins that an unused hook stays nil,
// so the Manager's nil fast path still applies.
func TestAliasFanout_NilWhenNothingSubscribes(t *testing.T) {
	require.Nil(t, newAliasFanout())
	require.Nil(t, newAliasFanout(nil, nil))
}

// TestAliasFanout_UnwrapsSingleSubscriber keeps the common case free of an
// indirection: with one subscriber there is nothing to fan out to.
func TestAliasFanout_UnwrapsSingleSubscriber(t *testing.T) {
	only := &recordingRewriter{}
	got := newAliasFanout(nil, only)
	require.Same(t, only, got)
}

func TestAliasFanout_NotifiesEverySubscriber(t *testing.T) {
	a, b := &recordingRewriter{}, &recordingRewriter{}
	fanout := newAliasFanout(a, b)
	ctx := context.Background()

	require.NoError(t, fanout.EntityRenamed(ctx, "TKT-old", "TKT-new"))
	require.NoError(t, fanout.EntityDeleted(ctx, "TKT-1"))

	require.Equal(t, []string{"TKT-old->TKT-new"}, a.renamed)
	require.Equal(t, []string{"TKT-old->TKT-new"}, b.renamed)
	require.Equal(t, []string{"TKT-1"}, a.deleted)
	require.Equal(t, []string{"TKT-1"}, b.deleted)
}

// TestAliasFanout_OneFailureDoesNotSkipOthers pins the join-don't-short-circuit
// rule. A failing CalDAV rewrite must not stop the comment store learning about
// a rename — that would strand a thread for an unrelated reason.
func TestAliasFanout_OneFailureDoesNotSkipOthers(t *testing.T) {
	boom := errors.New("boom")
	failing := &recordingRewriter{err: boom}
	healthy := &recordingRewriter{}
	fanout := newAliasFanout(failing, healthy)

	err := fanout.EntityRenamed(context.Background(), "TKT-old", "TKT-new")
	require.ErrorIs(t, err, boom)
	require.Equal(t, []string{"TKT-old->TKT-new"}, healthy.renamed,
		"a failing subscriber must not prevent the next one being notified")
}
