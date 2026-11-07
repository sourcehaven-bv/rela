package appbuild_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const deletedSchema = `
entities:
  note:
    label: Note
    id_type: manual
    id_prefix: "NOTE-"
    properties:
      title: {type: string, required: true}
      secret: {type: string}
  log:
    label: Log
    id_type: sequential
    id_prefix: "LOG-"
    properties:
      title: {type: string, required: true}
automations:
  - name: gone
    on: {entity: note, deleted: true}
    do:
      - lua_file: gone.lua
        background: true
`

// goneScript logs the deleted note as the job saw it.
const goneScript = `
rela.create_entity("log", {title = entity.id .. ":" .. entity.properties.title .. ":" ..
  tostring(entity.properties.secret)})
`

const deletedEditorRole = `  editor:
    read: [note, log]
    create: [note, log]
    update: [note, log]
    delete: [note]
`

func writeDeletedProject(t *testing.T, policy string) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "schema.yaml"), []byte(deletedSchema), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "scripts", "gone.lua"), []byte(goneScript), 0o600))
	if policy != "" {
		require.NoError(t, os.WriteFile(filepath.Join(root, "acl.yaml"), []byte(policy), 0o600))
	}
	return root
}

func logTitles(t *testing.T, svc *appbuild.Services) []string {
	t.Helper()
	var out []string
	for e, err := range svc.Store().ListEntities(context.Background(), store.EntityQuery{Type: "log", Faces: store.AllFaces()}) {
		require.NoError(t, err)
		out = append(out, e.GetString("title"))
	}
	return out
}

// TestDeletedTrigger_EndToEnd: deleting a note runs the `on.deleted` script
// with the note's last state, as far as the job's identity may read it.
func TestDeletedTrigger_EndToEnd(t *testing.T) {
	tests := []struct {
		name   string
		policy string
		queue  bool
		want   []string
	}{
		{"no acl", "", false, []string{"NOTE-D1:one:hush"}},
		{"no acl, queued", "", true, []string{"NOTE-D1:one:hush"}},
		{"job identity without a read grant", "roles:\n" + deletedEditorRole +
			"  logger:\n    create: [log]\nassignments:\n  alice: editor\n  \"system:automation\": logger\n", false, nil},
		{"job identity with the secret hidden", "roles:\n" + deletedEditorRole +
			"  automation:\n    read: [note, log]\n    create: [log]\n    visible:\n      note:\n        - field: title\n" +
			"assignments:\n  alice: editor\n  \"system:automation\": automation\n",
			false, []string{"NOTE-D1:one:nil"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := backendtest.Options(t)
			if tc.queue {
				opts = append(opts, appbuild.WithBackgroundAutomationJobs())
			}
			svc, err := appbuild.Discover(writeDeletedProject(t, tc.policy), script.NewEngine(), opts...)
			require.NoError(t, err)
			t.Cleanup(func() { _ = svc.Close() })

			ctx := asAlice(t, svc)
			_, err = svc.EntityManager().CreateEntity(ctx, &entity.Entity{
				Type: "note", Properties: map[string]any{"title": "one", "secret": "hush"},
			}, entity.CreateOptions{ID: "NOTE-D1"})
			require.NoError(t, err)
			require.Empty(t, logTitles(t, svc), "a create must not fire `on.deleted`")

			if entitymanager.SupportsSoftDelete(svc.EntityManager()) {
				_, err = entitymanager.SoftDeleteEntity(ctx, svc.EntityManager(), "NOTE-D1")
			} else {
				_, err = svc.EntityManager().DeleteEntity(ctx, "NOTE-D1", true)
			}
			require.NoError(t, err)

			if tc.queue {
				require.Eventually(t, func() bool { return len(logTitles(t, svc)) == len(tc.want) },
					10*time.Second, 20*time.Millisecond)
			}
			require.Equal(t, tc.want, logTitles(t, svc))
		})
	}
}
