//go:build !postgres && !memorybackend && !sqlite

package appbuild_test

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const realtimeSchema = `version: "1.0"
entities:
  doc:
    label: Doc
    id_prefix: "DOC-"
    id_type: sequential
    properties:
      title: {type: string}
      owner: {type: string}
`

const realtimeYAML = `version: 1
channel_types:
  room:
    attach_to: [doc]
    roles:
      owner: { from: entity.owner }
    message_types:
      note:
        post: [owner]
triggers:
  titled:
    entity_type: doc
    on: enter
    when: "entity.title == 'x'"
    channel: room
    post: { type: note }
`

// realtimeProject writes a project with the given files (schema.yaml is
// always realtimeSchema unless files overrides it) and returns its root.
func realtimeProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	all := map[string]string{"schema.yaml": realtimeSchema}
	maps.Copy(all, files)
	for name, body := range all {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(body), 0o644))
	}
	for _, dir := range []string{".rela", "entities", "relations"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o755))
	}
	return root
}

func realtimeBase(t *testing.T, root string, opts ...appbuild.Option) (*appbuild.SharedBase, error) {
	t.Helper()
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	require.NoError(t, err)
	return appbuild.NewSharedBase(appbuild.Config{
		FS: fs, Paths: paths, ScriptEngine: script.NewEngine(), Audit: audit.Nop{},
	}, opts...)
}

func realtimeServices(t *testing.T, base *appbuild.SharedBase) *appbuild.Services {
	t.Helper()
	st := memstore.New()
	svc, err := base.Assemble(st, search.New(st, search.NewLinearSearch()), nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func writeProjectFile(t *testing.T, root, name, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(body), 0o644))
}

// Without realtime.yaml realtime is off and nothing else changes.
func TestRealtime_AbsentFileIsOff(t *testing.T) {
	base, err := realtimeBase(t, realtimeProject(t, nil))
	require.NoError(t, err)
	snap := appbuild.Realtime(realtimeServices(t, base)).Load()
	require.Nil(t, snap.Config)
	require.False(t, snap.Degraded)
}

// An invalid realtime.yaml fails the boot, as an invalid data-entry.yaml
// does, with an error naming the file and key.
func TestRealtime_InvalidFileFailsBoot(t *testing.T) {
	root := realtimeProject(t, map[string]string{
		"realtime.yaml": strings.Replace(realtimeYAML, "entity.owner", "entity.nope", 1),
	})
	_, err := realtimeBase(t, root)
	require.ErrorContains(t, err, "realtime.yaml: channel_types.room.roles.owner.from")
}

// The leak checks read acl.yaml even when the caller injected another ACL:
// NopACL changes who may write, not which fields acl.yaml hides.
func TestRealtime_InjectedACLStillChecksVisibleGrants(t *testing.T) {
	root := realtimeProject(t, map[string]string{
		"realtime.yaml": realtimeYAML,
		"acl.yaml": "user_entity_type: doc\nroles:\n  r:\n    read: [\"*\"]\n" +
			"    visible:\n      doc: [{field: title}]\n",
	})
	_, err := realtimeBase(t, root, appbuild.WithACL(acl.NopACL{}))
	require.ErrorContains(t, err, "visible: grant")
}

// A hot reload applies a valid file and keeps the previous config on an
// invalid one or on an anchor change; deleting the file turns realtime off.
func TestRealtime_Reload(t *testing.T) {
	root := realtimeProject(t, map[string]string{"realtime.yaml": realtimeYAML})
	base, err := realtimeBase(t, root)
	require.NoError(t, err)
	holder := appbuild.Realtime(realtimeServices(t, base))
	ctx := context.Background()
	first := holder.Load().Config
	require.NotNil(t, first)

	writeProjectFile(t, root, "realtime.yaml", strings.Replace(realtimeYAML, "attach_to: [doc]", "attach_to: [nope]", 1))
	var cfgErr *realtimecfg.Error
	require.ErrorAs(t, holder.Reload(ctx), &cfgErr)
	require.Same(t, first, holder.Load().Config, "a failed reload keeps the previous config")

	writeProjectFile(t, root, "realtime.yaml", strings.Replace(realtimeYAML, "attach_to: [doc]",
		"attach_to: [doc]\n    anchor: face", 1))
	require.Error(t, holder.Reload(ctx), "anchor face on a type without faces")

	writeProjectFile(t, root, "realtime.yaml", strings.Replace(realtimeYAML, "on: enter", "on: leave", 1))
	require.NoError(t, holder.Reload(ctx))
	next := holder.Load().Config
	require.NotSame(t, first, next)
	require.Equal(t, realtimecfg.TriggerLeave, next.Triggers()[0].On)

	require.NoError(t, os.Remove(filepath.Join(root, "realtime.yaml")))
	require.NoError(t, holder.Reload(ctx))
	require.Nil(t, holder.Load().Config)
}

// Deleting realtime.yaml and adding it back with another anchor is the same
// anchor change as editing it in place, so Reload refuses it.
func TestRealtime_ReloadAnchorAfterDelete(t *testing.T) {
	schema := realtimeSchema + `  page:
    label: Page
    id_prefix: "PAGE-"
    id_type: sequential
    properties:
      title: {type: string}
    faces:
      draft: {}
`
	const pages = `version: 1
channel_types:
  threads:
    attach_to: [page]
    anchor: face
    roles:
      reader: { from: attached }
    message_types:
      note:
        post: [reader]
`
	root := realtimeProject(t, map[string]string{"schema.yaml": schema, "realtime.yaml": pages})
	base, err := realtimeBase(t, root)
	require.NoError(t, err)
	holder := appbuild.Realtime(realtimeServices(t, base))
	ctx := context.Background()
	require.NotNil(t, holder.Load().Config)

	require.NoError(t, os.Remove(filepath.Join(root, "realtime.yaml")))
	require.NoError(t, holder.Reload(ctx))
	require.Nil(t, holder.Load().Config)

	writeProjectFile(t, root, "realtime.yaml", strings.Replace(pages, "    anchor: face\n", "", 1))
	require.ErrorContains(t, holder.Reload(ctx), "cannot change from face to family")
	require.Nil(t, holder.Load().Config)
}

// A schema reload that breaks realtime.yaml keeps the predecessor's config
// with automation off when the caller asked for the fallback, and fails the
// build otherwise.
func TestRealtime_SchemaReloadFallback(t *testing.T) {
	root := realtimeProject(t, map[string]string{"realtime.yaml": realtimeYAML})
	base, err := realtimeBase(t, root)
	require.NoError(t, err)
	origin := realtimeServices(t, base)
	prev := appbuild.Realtime(origin).Load().Config

	writeProjectFile(t, root, "schema.yaml", strings.Replace(realtimeSchema, "      owner: {type: string}\n", "", 1))
	_, err = realtimeBase(t, root)
	require.ErrorContains(t, err, "realtime.yaml")

	next, err := realtimeBase(t, root, appbuild.WithRealtimeFallback(origin))
	require.NoError(t, err)
	snap := appbuild.Realtime(realtimeServices(t, next)).Load()
	require.True(t, snap.Degraded)
	require.Same(t, prev, snap.Config)
	require.ErrorContains(t, snap.Problem, `doc has no property "owner"`)
}

func TestRealtime_NilHolder(t *testing.T) {
	require.Nil(t, appbuild.Realtime(nil))
	var holder *appbuild.RealtimeConfig
	require.Nil(t, holder.Load().Config)
	require.NoError(t, holder.Reload(context.Background()))
}
