//go:build !postgres && !memorybackend && !sqlite

package appbuild_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

const strictSchema = `version: "1.0"
entities:
  doc:
    label: Doc
    id_prefix: "DOC-"
    id_type: sequential
    properties:
      title: {type: string}
      escalated: {type: boolean}
`

const strictPolicy = `roles:
  bot:
    read: [doc]
    update: [doc]
assignments:
  system:bot: bot
`

// strictRealtime declares a session whose timer sets escalated as runAs.
func strictRealtime(runAs string) string {
	return `version: 1
channel_types:
  watch:
    attach_to: [doc]
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: string } }, post: [member] }
    session:
      initial: waiting
      run_as: ` + runAs + `
      states:
        waiting:
          after:
            soon:
              period: 1m
              to: done
              outputs:
                - set: { properties: { escalated: true } }
        done: { final: true }
`
}

// fileReader reads project files from a directory, as the validate and
// Configure loaders do.
type fileReader string

func (r fileReader) Load(_ context.Context, name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(string(r), name))
}

const ungranted = "system:bot cannot update doc"

// TestRunAsGrantCheck_StrictEverywhere pins that every loader refuses the
// same realtime.yaml with an ungranted run_as: boot, the LoadRealtime path
// `rela validate` and the Configure validation share, a hot reload (which
// keeps the previous config), and a schema/acl hot-reload successor
// (which falls back to the previous config in degraded mode).
func TestRunAsGrantCheck_StrictEverywhere(t *testing.T) {
	ungrantedPolicy := strings.Replace(strictPolicy, "    update: [doc]\n", "", 1)

	t.Run("boot", func(t *testing.T) {
		root := realtimeProject(t, map[string]string{
			"schema.yaml": strictSchema, "acl.yaml": ungrantedPolicy, "realtime.yaml": strictRealtime("system:bot"),
		})
		_, err := realtimeBase(t, root)
		require.ErrorContains(t, err, ungranted)
	})

	t.Run("validate and configure", func(t *testing.T) {
		root := realtimeProject(t, map[string]string{
			"schema.yaml": strictSchema, "realtime.yaml": strictRealtime("system:bot"),
		})
		meta, err := metamodel.Parse([]byte(strictSchema))
		require.NoError(t, err)
		p, err := acl.ParsePolicy([]byte(ungrantedPolicy), "acl.yaml")
		require.NoError(t, err)
		_, err = appbuild.LoadRealtime(context.Background(), fileReader(root), meta, p)
		require.ErrorContains(t, err, ungranted)
		granted, err := acl.ParsePolicy([]byte(strictPolicy), "acl.yaml")
		require.NoError(t, err)
		c, err := appbuild.LoadRealtime(context.Background(), fileReader(root), meta, granted)
		require.NoError(t, err)
		require.Empty(t, c.Warnings())
	})

	t.Run("reload", func(t *testing.T) {
		root := realtimeProject(t, map[string]string{
			"schema.yaml": strictSchema, "acl.yaml": strictPolicy, "realtime.yaml": strictRealtime("system:bot"),
		})
		base, err := realtimeBase(t, root)
		require.NoError(t, err)
		holder := appbuild.Realtime(realtimeServices(t, base))
		prev := holder.Load().Config
		writeProjectFile(t, root, "realtime.yaml", strictRealtime("system:other"))
		require.ErrorContains(t, holder.Reload(context.Background()), "system:other cannot update doc")
		require.Same(t, prev, holder.Load().Config, "the previous snapshot stays")
	})

	t.Run("hot-reload successor", func(t *testing.T) {
		root := realtimeProject(t, map[string]string{
			"schema.yaml": strictSchema, "acl.yaml": strictPolicy, "realtime.yaml": strictRealtime("system:bot"),
		})
		base, err := realtimeBase(t, root)
		require.NoError(t, err)
		origin := realtimeServices(t, base)
		prev := appbuild.Realtime(origin).Load().Config
		writeProjectFile(t, root, "acl.yaml", ungrantedPolicy)
		_, err = realtimeBase(t, root)
		require.ErrorContains(t, err, ungranted)
		next, err := realtimeBase(t, root, appbuild.WithRealtimeFallback(origin))
		require.NoError(t, err)
		snap := appbuild.Realtime(realtimeServices(t, next)).Load()
		require.True(t, snap.Degraded)
		require.Same(t, prev, snap.Config)
		require.ErrorContains(t, snap.Problem, ungranted)
	})
}
