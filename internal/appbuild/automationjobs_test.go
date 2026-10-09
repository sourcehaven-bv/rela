package appbuild_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

const backgroundSchema = `
entities:
  note:
    label: Note
    # manual: the rename tests need it (rename is manual-id only).
    id_type: manual
    id_prefix: "NOTE-"
    properties:
      title: {type: string, required: true}
      pushed: {type: string}
automations:
  - name: push
    on:
      entity: note
      created: true
      updated: true
    do:
      - lua_file: push.lua
        background: true
`

// pushScript records who ran it and what title it saw.
const pushScript = `
rela.update_entity(entity.id, {pushed = rela.principal.user .. ":" .. entity.properties.title})
`

func writeBackgroundProject(t *testing.T) string {
	t.Helper()
	return writeBackgroundSchema(t, backgroundSchema)
}

func writeBackgroundSchema(t *testing.T, schema string) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "schema.yaml"), []byte(schema), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "scripts", "push.lua"), []byte(pushScript), 0o600))
	return root
}

// noteSeq mints manual note ids.
var noteSeq atomic.Int64

func createNote(t *testing.T, svc *appbuild.Services) *entity.Entity {
	t.Helper()
	return createNoteWith(t, svc, map[string]any{"title": "one"})
}

func createNoteWith(t *testing.T, svc *appbuild.Services, props map[string]any) *entity.Entity {
	t.Helper()
	res, err := svc.EntityManager().CreateEntity(asAlice(), &entity.Entity{
		Type: "note", Properties: props,
	}, entity.CreateOptions{ID: fmt.Sprintf("NOTE-%d", noteSeq.Add(1))})
	require.NoError(t, err)
	require.Empty(t, res.AutomationErrors)
	return res.Entity
}

func pushed(t *testing.T, svc *appbuild.Services, id string) string {
	t.Helper()
	e, err := svc.Store().GetEntity(context.Background(), entity.Ref{ID: id})
	require.NoError(t, err)
	v, _ := e.Properties["pushed"].(string)
	return v
}

// TestBackgroundAction_ForegroundByDefault: without the queue option the
// action runs right after the save, as system:automation, and its own write
// does not schedule it again.
func TestBackgroundAction_ForegroundByDefault(t *testing.T) {
	svc, err := appbuild.Discover(writeBackgroundProject(t), script.NewEngine(), backendtest.Options(t)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	e := createNote(t, svc)
	require.Equal(t, principal.UserAutomation+":one", pushed(t, svc, e.ID))
}

// TestBackgroundAction_QueueMode: with the option the save returns and the
// queue runs the action; a later save runs it again with the new state.
func TestBackgroundAction_QueueMode(t *testing.T) {
	opts := append(backendtest.Options(t), appbuild.WithBackgroundAutomationJobs())
	svc, err := appbuild.Discover(writeBackgroundProject(t), script.NewEngine(), opts...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	e := createNote(t, svc)
	want := principal.UserAutomation + ":one"
	require.Eventually(t, func() bool { return pushed(t, svc, e.ID) == want }, 10*time.Second, 20*time.Millisecond)

	_, err = svc.EntityManager().PatchEntity(asAlice(), e.ID, entity.Patch{
		Properties: map[string]any{"title": "two"},
	})
	require.NoError(t, err)
	want = principal.UserAutomation + ":two"
	require.Eventually(t, func() bool { return pushed(t, svc, e.ID) == want }, 10*time.Second, 20*time.Millisecond)
}

// TestBackgroundAction_RenameRetriggers: a rename schedules the action for
// the new id, which a job naming the old id could not reach.
func TestBackgroundAction_RenameRetriggers(t *testing.T) {
	svc, err := appbuild.Discover(writeBackgroundProject(t), script.NewEngine(), backendtest.Options(t)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	e := createNote(t, svc)
	clearPushed(t, svc, e)

	_, err = svc.EntityManager().RenameEntity(asAlice(), e.ID, "NOTE-RENAMED", entity.RenameOptions{})
	require.NoError(t, err)
	require.Equal(t, principal.UserAutomation+":one", pushed(t, svc, "NOTE-RENAMED"))
}

// clearPushed removes the mark through the store, so only a later trigger
// can set it again.
func clearPushed(t *testing.T, svc *appbuild.Services, e *entity.Entity) {
	t.Helper()
	stored, err := svc.Store().GetEntity(context.Background(), e.Ref())
	require.NoError(t, err)
	delete(stored.Properties, "pushed")
	require.NoError(t, svc.Store().UpdateEntity(context.Background(), stored))
}

// TestBackgroundAction_RenameReschedulesPendingCreate: in queue mode a
// rename schedules a created-only trigger's action for the new id, since a
// job still pending for the old id finds nothing (code review S3).
func TestBackgroundAction_RenameReschedulesPendingCreate(t *testing.T) {
	root := writeBackgroundSchema(t, strings.Replace(backgroundSchema, "      updated: true\n", "", 1))
	opts := append(backendtest.Options(t), appbuild.WithBackgroundAutomationJobs())
	svc, err := appbuild.Discover(root, script.NewEngine(), opts...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	e := createNote(t, svc)
	want := principal.UserAutomation + ":one"
	require.Eventually(t, func() bool { return pushed(t, svc, e.ID) == want }, 10*time.Second, 20*time.Millisecond)
	clearPushed(t, svc, e)

	_, err = svc.EntityManager().RenameEntity(asAlice(), e.ID, "NOTE-RENAMED", entity.RenameOptions{})
	require.NoError(t, err)
	require.Eventually(t, func() bool { return pushed(t, svc, "NOTE-RENAMED") == want },
		10*time.Second, 20*time.Millisecond)
}

// TestBackgroundAction_RenameDecidesOnStoredValues: the rename hook reads
// the entity raw, so a condition on a field the renamer cannot see decides
// as it would for any save (security review).
func TestBackgroundAction_RenameDecidesOnStoredValues(t *testing.T) {
	schema := strings.Replace(backgroundSchema, "      pushed: {type: string}\n",
		"      pushed: {type: string}\n      secret: {type: string}\n", 1)
	schema = strings.Replace(schema, "      updated: true\n",
		"      updated: true\n      condition: \"entity.secret ~= 'hold'\"\n", 1)
	root := writeBackgroundSchema(t, schema)
	require.NoError(t, os.WriteFile(filepath.Join(root, "acl.yaml"), []byte(`roles:
  editor:
    read: [note]
    create: [note]
    update: [note]
    delete: [note]
    visible:
      note:
        - field: title
        - field: pushed
  automation:
    read: [note]
    update: [note]
assignments:
  alice: editor
  "system:automation": automation
`), 0o600))
	svc, err := appbuild.Discover(root, script.NewEngine(), backendtest.Options(t)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	held := createNoteWith(t, svc, map[string]any{"title": "one", "secret": "hold"})
	require.Empty(t, pushed(t, svc, held.ID))
	free := createNoteWith(t, svc, map[string]any{"title": "two", "secret": "go"})
	require.Equal(t, principal.UserAutomation+":two", pushed(t, svc, free.ID))
	clearPushed(t, svc, free)

	for _, e := range []*entity.Entity{held, free} {
		_, err = svc.EntityManager().RenameEntity(asAlice(), e.ID, e.ID+"-R", entity.RenameOptions{})
		require.NoError(t, err)
	}
	require.Empty(t, pushed(t, svc, held.ID+"-R"), "a hidden value must not flip the condition")
	require.Equal(t, principal.UserAutomation+":two", pushed(t, svc, free.ID+"-R"))
}

// TestBackgroundAction_IdentityNeedsGrants: under acl.yaml the job runs with
// its own grants, not the saver's. Without any, it cannot read the entity
// and ends without writing; the save itself stands.
func TestBackgroundAction_IdentityNeedsGrants(t *testing.T) {
	root := writeBackgroundProject(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "acl.yaml"), []byte(`roles:
  editor:
    read: [note]
    create: [note]
    update: [note]
assignments:
  alice: editor
`), 0o600))
	svc, err := appbuild.Discover(root, script.NewEngine(), backendtest.Options(t)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	e := createNote(t, svc)
	require.Empty(t, pushed(t, svc, e.ID))
}

// TestBackgroundAction_IdentityWithGrants: given a role, the job's
// identity reads the entity and writes as itself.
func TestBackgroundAction_IdentityWithGrants(t *testing.T) {
	root := writeBackgroundProject(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "acl.yaml"), []byte(`roles:
  editor:
    read: [note]
    create: [note]
    update: [note]
assignments:
  alice: editor
  "system:automation": editor
`), 0o600))
	svc, err := appbuild.Discover(root, script.NewEngine(), backendtest.Options(t)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	e := createNote(t, svc)
	require.Equal(t, principal.UserAutomation+":one", pushed(t, svc, e.ID))
}
