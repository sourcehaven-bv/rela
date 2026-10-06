//go:build sqlite

package appbuild_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/scheduler"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// dbOnlySchema declares one entity type and one Lua validation rule whose
// script lives in validations/.
const dbOnlySchema = `version: "1.0"
entities:
  item:
    label: Item
    plural: items
    id_prefix: "ITEM-"
    id_type: sequential
    properties:
      title:
        type: string
relations: {}
validations:
  - name: no-bad-titles
    description: "Title must not be bad"
    entity_type: item
    lua_file: no-bad.lua
    severity: error
`

// TestSQLite_DatabaseOnlyProjectRunsItsConfig is the end-to-end check of a
// project shipped as rela.db alone. Its schema, an action script, a Lua
// validation rule's script and a scheduled task with its script are stored
// with `rela db load` semantics, then every file is removed from disk. Each
// consumer must still find its config through the database:
//
//   - the action runs from actions/ through the Lua deps' project files;
//   - the validation rule's lua_file is read from validations/;
//   - the background scheduler reads schedules.yaml through the config
//     loader and runs its task's script from scripts/.
func TestSQLite_DatabaseOnlyProjectRunsItsConfig(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "schema.yaml", dbOnlySchema)
	writeFile(t, root, "actions/greet.lua", "return { message = 'action from the database' }\n")
	writeFile(t, root, "validations/no-bad.lua",
		"if entity.properties.title == 'bad' then return { message = 'bad title' } end\nreturn nil\n")
	writeFile(t, root, "schedules.yaml",
		"tasks:\n  - name: stamp\n    script: stamp.lua\n    every: 1h\n")
	writeFile(t, root, "scripts/stamp.lua",
		"rela.create_entity('item', { title = 'from the schedule' })\n")
	writeFile(t, root, ".rela/.keep", "")
	names := bake(t, root)
	if len(names) != 5 {
		t.Fatalf("stored %v, want the schema, schedules.yaml and three scripts", names)
	}
	removeAll(t, root, "schema.yaml", "schedules.yaml", "actions", "validations", "scripts")

	svc, err := discover(t, root)
	if err != nil {
		t.Fatalf("boot from the database alone: %v", err)
	}
	defer func() { _ = svc.Close() }()
	ctx := context.Background()

	t.Run("action", func(t *testing.T) {
		resp, err := script.NewEngine().ExecuteAction(ctx, "greet.lua", svc.LuaWriteDeps(), nil, nil, 5*time.Second, "")
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		if resp.Message != "action from the database" {
			t.Fatalf("action message = %q", resp.Message)
		}
	})

	t.Run("validation", func(t *testing.T) {
		if err := svc.Store().CreateEntity(ctx, &entity.Entity{
			ID: "ITEM-900", Type: "item", Properties: map[string]any{"title": "bad"},
		}); err != nil {
			t.Fatal(err)
		}
		if len(svc.Meta().Validations) != 1 {
			t.Fatalf("validations = %d, want the stored rule", len(svc.Meta().Validations))
		}
		res, err := svc.Validator().CheckRuleFull(ctx, svc.Meta().Validations[0])
		if err != nil {
			t.Fatal(err)
		}
		if len(res.LoadErrors) > 0 || len(res.ScriptErrors) > 0 {
			t.Fatalf("rule did not run: load errors %v, script errors %v", res.LoadErrors, res.ScriptErrors)
		}
		if len(res.Violations) != 1 || res.Violations[0].EntityID != "ITEM-900" ||
			res.Violations[0].Message != "bad title" {
			t.Fatalf("violations = %+v, want ITEM-900 with the script's message", res.Violations)
		}
	})

	t.Run("scheduled task", func(t *testing.T) {
		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		scheduler.StartBackground(runCtx, svc, slog.New(slog.NewTextHandler(io.Discard, nil)))

		deadline := time.Now().Add(10 * time.Second)
		for {
			if scheduledItemExists(t, svc.Store()) {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal("the scheduled task never created its entity")
			}
			time.Sleep(50 * time.Millisecond)
		}
	})
}

// scheduledItemExists reports whether the scheduled task's entity exists.
func scheduledItemExists(t *testing.T, st store.Store) bool {
	t.Helper()
	for e, err := range st.ListEntities(context.Background(), store.EntityQuery{Type: "item", Faces: store.AllFaces()}) {
		if err != nil {
			t.Fatal(err)
		}
		if e.Properties["title"] == "from the schedule" {
			return true
		}
	}
	return false
}
