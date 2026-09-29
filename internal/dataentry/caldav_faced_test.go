package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestCalDAV_FacedEntityIsNotGone pins the liveness probes behind a CalDAV
// write for a faced entity, which has no zero-face row (DEC-NPZICR). A
// zero-face probe called it deleted, so a PUT through its alias answered the
// permanent 404 that tells the client to drop its copy, and the collection
// membership check refused it outright.
func TestCalDAV_FacedEntityIsNotGone(t *testing.T) {
	app := caldavTestApp(t)
	faced := &entity.Entity{ID: "TSK-F", Type: "task", Face: "draft", Properties: map[string]any{"title": "F"}}
	if err := app.store.CreateEntity(t.Context(), faced); err != nil {
		t.Fatal(err)
	}
	b := &caldavBackend{app: app}
	m := &caldavMapper{cfg: dataentryconfig.CalDAVCollection{EntityType: "task"}}

	if b.entityIsGone(t.Context(), m, "TSK-F") {
		t.Error("a faced entity with a stored face is not gone")
	}
	if !b.entityIsGone(t.Context(), m, "TSK-NOPE") {
		t.Error("an entity with no stored face is gone")
	}

	if id, ok := b.entityIDFor(t.Context(), "tasks", "task--TSK-F@rela.ics", m); !ok || id != "TSK-F" {
		t.Errorf("entityIDFor faced task = %q, %v; want TSK-F, true", id, ok)
	}
	other := &caldavMapper{cfg: dataentryconfig.CalDAVCollection{EntityType: "note"}}
	if _, ok := b.entityIDFor(t.Context(), "tasks", "note--TSK-F@rela.ics", other); ok {
		t.Error("a task must not resolve through a collection of another type")
	}
}
