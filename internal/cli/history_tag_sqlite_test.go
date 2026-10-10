//go:build sqlite

package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const tagCLIMetamodel = `version: "1.0"
entities:
  doc:
    label: Doc
    plural: docs
    id_prefix: "DOC-"
    id_type: sequential
    properties:
      title: {type: string}
`

// tagCLIFixture is a sqlite project with one doc and the CLI bundles over
// it. run executes a command as the operator and returns its output.
type tagCLIFixture struct {
	svc *appbuild.Services
	b   *cliBundles
	id  string
}

// tagCLICtx is the operator context every fixture command runs with.
func tagCLICtx() context.Context {
	return principal.With(context.Background(), principal.Principal{User: "ops", Tool: principal.ToolCLI})
}

func newTagCLIFixture(t *testing.T) *tagCLIFixture {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".rela"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "metamodel.yaml"), []byte(tagCLIMetamodel), 0o600))
	svc, err := appbuild.Discover(root, script.NewEngine())
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	b, err := newCLIBundles(svc)
	require.NoError(t, err)

	e := entity.New("", "doc")
	e.SetString("title", "first")
	created, err := svc.EntityManager().CreateEntity(tagCLICtx(), e, entity.CreateOptions{})
	require.NoError(t, err)
	return &tagCLIFixture{svc: svc, b: b, id: created.Entity.ID}
}

func (f *tagCLIFixture) run(t *testing.T, cmd interface {
	Run(context.Context, *writeServices) error
}) (string, error) {
	t.Helper()
	buf := captureOut(t)
	runErr := cmd.Run(tagCLICtx(), f.b.write)
	return buf.String(), runErr
}

// versions lists the doc's versions.
func (f *tagCLIFixture) versions(t *testing.T) []store.VersionMeta {
	t.Helper()
	metas, err := f.b.write.Versions.ListVersions(tagCLICtx(), entity.Ref{ID: f.id})
	require.NoError(t, err)
	return metas
}

// TestHistoryTagCmd_SQLite drives history-tag, history and history-purge
// against a real sqlite store: tag, list, refuse a purge of the tagged row,
// purge it with --force-tags, and delete a tag.
func TestHistoryTagCmd_SQLite(t *testing.T) {
	f := newTagCLIFixture(t)
	id := f.id

	got, err := f.run(t, &HistoryTagCmd{ID: id, Name: "reviewed"})
	require.NoError(t, err)
	require.Contains(t, got, `Tagged `+id+` v1 as "reviewed"`)

	_, err = f.run(t, &HistoryTagCmd{ID: id, Name: "reviewed", Version: 99})
	require.ErrorContains(t, err, "no version 99")
	_, err = f.run(t, &HistoryTagCmd{ID: id, Name: "reviewed", Expect: "stale"})
	require.ErrorContains(t, err, "changed since token")
	_, err = f.run(t, &HistoryTagCmd{ID: id, Name: "Bad Name"})
	require.Error(t, err)

	buf := captureOut(t)
	require.NoError(t, (&HistoryCmd{ID: id}).Run(tagCLICtx(), f.b.read))
	require.Contains(t, buf.String(), "tags: reviewed")

	purge := HistoryPurgeCmd{ID: id, All: true, Reason: "test", ForceLive: true}
	got, err = f.run(t, &purge)
	require.NoError(t, err)
	require.Contains(t, got, `tagged "reviewed"`)
	require.Contains(t, got, "--force-tags")

	purge.Commit, purge.Yes = true, true
	got, err = f.run(t, &purge)
	require.NoError(t, err)
	require.Contains(t, got, "Refused: a version tag points at a target row")
	require.Len(t, f.versions(t), 1, "a refused purge deletes nothing")

	_, err = f.run(t, &HistoryTagCmd{ID: id, Name: "kept"})
	require.NoError(t, err)
	got, err = f.run(t, &HistoryTagCmd{ID: id, Name: "kept", Delete: true})
	require.NoError(t, err)
	require.Contains(t, got, `Deleted tag "kept"`)
	_, err = f.run(t, &HistoryTagCmd{ID: id, Name: "kept", Delete: true})
	require.ErrorContains(t, err, `has no tag "kept"`)

	purge.ForceTags = true
	got, err = f.run(t, &purge)
	require.NoError(t, err)
	require.Contains(t, got, "Purged 1 version row(s)")
	metas := f.versions(t)
	require.Len(t, metas, 1, "the tagged version is gone; only the tombstone remains")
	require.Equal(t, store.VersionOpPurge, metas[0].Op)
	require.Empty(t, metas[0].Tags)
}

// TestHistoryPurgeCmd_SQLiteForceLive commits a --force-live purge of an
// untagged version: the row is deleted and a tombstone written.
func TestHistoryPurgeCmd_SQLiteForceLive(t *testing.T) {
	f := newTagCLIFixture(t)
	// Tagging captures the unswept state; untagging leaves the version
	// without a tag.
	_, err := f.run(t, &HistoryTagCmd{ID: f.id, Name: "capture"})
	require.NoError(t, err)
	_, err = f.run(t, &HistoryTagCmd{ID: f.id, Name: "capture", Delete: true})
	require.NoError(t, err)
	before := f.versions(t)
	require.Len(t, before, 1)
	require.Equal(t, store.VersionOpCreate, before[0].Op)

	got, err := f.run(t, &HistoryPurgeCmd{
		ID: f.id, All: true, Reason: "test", ForceLive: true, Commit: true, Yes: true,
	})
	require.NoError(t, err)
	require.Contains(t, got, "Purged 1 version row(s)")
	require.Contains(t, got, "Wrote a purge tombstone")
	after := f.versions(t)
	require.Len(t, after, 1)
	require.Equal(t, store.VersionOpPurge, after[0].Op)
}

// taggingPurger tags the doc's version between the preview and the commit
// of a purge, the race the commit refusal exists for.
type taggingPurger struct {
	store.VersionService
	tag func() error
}

func (p taggingPurger) PurgeVersions(ctx context.Context, req store.VersionPurgeRequest) (*store.PurgeResult, error) {
	if !req.DryRun {
		if err := p.tag(); err != nil {
			return nil, err
		}
	}
	return p.VersionService.PurgeVersions(ctx, req)
}

// TestHistoryPurgeCmd_SQLiteRefusedAtCommit: a tag added after the preview
// makes the store refuse the commit. The command must fail and must not
// audit a purge that deleted nothing.
func TestHistoryPurgeCmd_SQLiteRefusedAtCommit(t *testing.T) {
	f := newTagCLIFixture(t)
	_, err := f.run(t, &HistoryTagCmd{ID: f.id, Name: "capture"})
	require.NoError(t, err)
	_, err = f.run(t, &HistoryTagCmd{ID: f.id, Name: "capture", Delete: true})
	require.NoError(t, err)

	sink := audit.NewMemory()
	f.b.write.Audit = sink
	name, err := store.ParseVersionTagName("late")
	require.NoError(t, err)
	f.b.write.Versions = taggingPurger{VersionService: f.b.write.Versions, tag: func() error {
		_, tagErr := appbuild.VersionTags(f.svc).TagVersion(tagCLICtx(), entity.Ref{ID: f.id}, name, 1)
		return tagErr
	}}

	_, err = f.run(t, &HistoryPurgeCmd{
		ID: f.id, All: true, Reason: "test", ForceLive: true, Commit: true, Yes: true,
	})
	require.ErrorContains(t, err, string(store.PurgeRefusedTagged))
	for _, r := range sink.Records() {
		require.NotEqual(t, audit.OpPurgeVersion, r.Op, "a refused purge was audited")
	}
	metas := f.versions(t)
	require.Len(t, metas, 1, "nothing was deleted")
	require.Equal(t, []string{"late"}, metas[0].Tags)
}
