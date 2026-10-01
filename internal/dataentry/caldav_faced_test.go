package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
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

// TestCalDAV_WriteAddress pins the face a CalDAV write edits (TKT-7IZHP0
// A15): the one readable face the request's world admits, refused when there
// are several, and a miss left to the write's own not-found handling.
func TestCalDAV_WriteAddress(t *testing.T) {
	app := caldavTestApp(t)
	for _, e := range []*entity.Entity{
		{ID: "TSK-F", Type: "task", Face: "draft", Properties: map[string]any{"title": "F"}},
		{ID: "TSK-G", Type: "task", Face: "draft", Properties: map[string]any{"title": "G"}},
		{ID: "TSK-G", Type: "task", Face: "published", Properties: map[string]any{"title": "G"}},
	} {
		if err := app.store.CreateEntity(t.Context(), e); err != nil {
			t.Fatal(err)
		}
	}
	b := &caldavBackend{app: app}
	m := &caldavMapper{cfg: dataentryconfig.CalDAVCollection{EntityType: "task"}}
	ctx := withWorld(t.Context(), worldHandle{scope: store.NewWorldScope(map[string]store.TypeResolution{
		"task": {Chain: []entity.Face{"draft", "published"}, Fallback: store.FallbackExclude},
	})})

	tests := []struct {
		id            string
		want          string
		wantAmbiguous bool
	}{
		{id: "TSK-F", want: "TSK-F@draft"},
		{id: "TSK-G", wantAmbiguous: true},
		{id: "TSK-NOPE", want: "TSK-NOPE"},
	}
	for _, tc := range tests {
		got, ambiguous, err := b.writeAddress(ctx, m, tc.id)
		if err != nil || got != tc.want || ambiguous != tc.wantAmbiguous {
			t.Errorf("writeAddress(%s) = %q, %v, %v; want %q, %v", tc.id, got, ambiguous, err, tc.want, tc.wantAmbiguous)
		}
	}
}
