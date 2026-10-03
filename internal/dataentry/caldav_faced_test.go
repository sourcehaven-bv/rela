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

	for addr, want := range map[string]bool{
		"TSK-F":           false, // some face is stored
		"TSK-F@draft":     false, // the named face is stored
		"TSK-F@published": true,  // the named face is not
		"TSK-NOPE":        true,
	} {
		if got := b.entityIsGone(t.Context(), m, addr); got != want {
			t.Errorf("entityIsGone(%s) = %v, want %v", addr, got, want)
		}
	}

	if addr, ok := b.entityIDFor(t.Context(), "tasks", "task--TSK-F@draft@rela.ics", m); !ok || addr != "TSK-F@draft" {
		t.Errorf("entityIDFor faced task = %q, %v; want TSK-F@draft, true", addr, ok)
	}
	other := &caldavMapper{cfg: dataentryconfig.CalDAVCollection{EntityType: "note"}}
	if _, ok := b.entityIDFor(t.Context(), "tasks", "note--TSK-F@rela.ics", other); ok {
		t.Error("a task must not resolve through a collection of another type")
	}
}

// TestCalDAV_WriteAddress pins the face a CalDAV write edits (TKT-7IZHP0
// A15): the face the resource's address names. A bare id on a faced type is
// refused even when only one face exists, and a miss is left to the write's
// own not-found handling.
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
		{id: "TSK-F@draft", want: "TSK-F@draft"},
		{id: "TSK-F", wantAmbiguous: true},
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

// TestFeedUID_CarriesTheFace pins that a faced row's UID names its face and
// round-trips, so a CalDAV href or UID addresses one face.
func TestFeedUID_CarriesTheFace(t *testing.T) {
	for _, addr := range []string{"TSK-1", "TSK-1@draft"} {
		uid := feedUID("task", addr)
		typ, got, ok := splitFeedUID(uid)
		if !ok || typ != "task" || got != addr {
			t.Errorf("splitFeedUID(%q) = %q, %q, %v; want task, %q, true", uid, typ, got, ok, addr)
		}
	}
	if got := feedUID("task", "TSK-1@draft"); got != "task--TSK-1@draft@rela" {
		t.Errorf("feedUID = %q, want task--TSK-1@draft@rela", got)
	}
}
