package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/caldavalias"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TKT-NXRFDU: a CalDAV write to a row the caller cannot read must answer
// exactly like a write to a row that does not exist. Any difference (a 503
// "temporarily unreadable", a 403, a different body) confirms to the caller
// that the hidden row exists.

const (
	hiddenCalDAVUser = "alice"
	hiddenTaskID     = "TSK-hidden"
	readableTaskID   = "TSK-mine"
	goneTaskID       = "TSK-gone"
)

// caldavAppWithRowPolicy is caldavTestApp with one declarative policy shared
// by the entity manager and the router's read gate, as in production. alice
// owns TSK-mine and so may read and write it; TSK-hidden exists but no grant
// reaches it, so it is invisible to her on both paths.
func caldavAppWithRowPolicy(t *testing.T) *App {
	t.Helper()
	app := caldavTestApp(t,
		task(readableTaskID, "Mine", "todo", ""),
		task(hiddenTaskID, "Secret", "todo", ""),
	)
	meta := app.State().Meta
	meta.Entities["person"] = metamodel.EntityDef{
		Label: "Person", IDPrefix: "P-", DisplayProperty: "title",
		Properties:    map[string]metamodel.PropertyDef{"title": {Type: metamodel.PropertyTypeString}},
		PropertyOrder: []string{"title"},
	}
	if meta.Relations == nil {
		meta.Relations = map[string]metamodel.RelationDef{}
	}
	meta.Relations["owns"] = metamodel.RelationDef{Label: "owns", From: []string{"person"}, To: []string{"task"}}

	seedEntity(app, &entity.Entity{ID: hiddenCalDAVUser, Type: "person", Properties: map[string]any{"title": "Alice"}})
	seedRelation(app, entity.NewRelation(hiddenCalDAVUser, "owns", readableTaskID))

	all := []string{"task"}
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{"owner": {
			Read: all, Create: all, Update: all, Delete: all,
		}},
		RoleRelations: map[string]acl.RoleRelationDef{"owns": {Confers: "owner"}},
	}, app.store)

	svc := appbuildtest.New(meta, appbuildtest.WithStore(app.store),
		appbuildtest.WithFS(app.fs, app.paths), appbuildtest.WithDeclarative(d))
	rebindApp(app, app.fs, app.paths, svc)
	setTestACL(t, app, d)
	app.SetPrincipalResolver(func(*http.Request) principal.Principal {
		return principal.Principal{User: hiddenCalDAVUser, Tool: principal.ToolDataEntry}
	})
	return app
}

func seedCalDAVAlias(t *testing.T, app *App, href, entityID string) {
	t.Helper()
	if err := app.caldavAliases.Put(t.Context(), caldavalias.Alias{
		Principal: hiddenCalDAVUser, Collection: "tasks",
		Href: href + ".ics", UID: href, EntityID: entityID,
	}); err != nil {
		t.Fatalf("seed alias: %v", err)
	}
}

func caldavTodoBody(uid, summary string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VTODO\r\nUID:" + uid +
		"\r\nDTSTAMP:20261012T080000Z\r\nSUMMARY:" + summary +
		"\r\nSTATUS:NEEDS-ACTION\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
}

func caldavTaskPath(uid string) string {
	return "/api/v1/_caldav/principal/calendars/tasks/" + uid + ".ics"
}

// requireTaskState asserts the stored row exists with this title and status.
func requireTaskState(t *testing.T, app *App, id, title, status string) {
	t.Helper()
	e, err := app.store.GetEntity(context.Background(), entity.Ref{ID: id})
	if err != nil || e == nil {
		t.Fatalf("%s is gone from the store: %v", id, err)
	}
	if got := e.Properties["title"]; got != title {
		t.Errorf("%s title = %v, want %q", id, got, title)
	}
	if got := e.Properties["status"]; got != status {
		t.Errorf("%s status = %v, want %q", id, got, status)
	}
}

// requireSameAnswer asserts a request against a hidden row got the same status
// and body as the same request against a missing row.
func requireSameAnswer(t *testing.T, hidden, gone *httptest.ResponseRecorder) {
	t.Helper()
	hiddenCode, goneCode := hidden.Code, gone.Code
	hiddenBody, goneBody := hidden.Body.String(), gone.Body.String()
	if goneCode != http.StatusNotFound {
		t.Fatalf("missing row = %d, want 404\n%s", goneCode, goneBody)
	}
	if hiddenCode != goneCode {
		t.Errorf("hidden row = %d, missing row = %d; the status discloses the hidden row\n%s",
			hiddenCode, goneCode, hiddenBody)
	}
	if hiddenBody != goneBody {
		t.Errorf("hidden and missing rows answer with different bodies; the body discloses the hidden row\n"+
			"hidden:  %q\nmissing: %q", hiddenBody, goneBody)
	}
}

// TestCalDAV_PutToHiddenEntityAnswersLikeDeleted: a PUT through an alias that
// points at an existing task the caller cannot read gets the deletion answer
// (404, same body), not the 503 "temporarily unreadable" that would confirm
// the row exists, and the task is not written.
func TestCalDAV_PutToHiddenEntityAnswersLikeDeleted(t *testing.T) {
	app := caldavAppWithRowPolicy(t)
	seedCalDAVAlias(t, app, "HIDDEN-PROBE", hiddenTaskID)
	seedCalDAVAlias(t, app, "GONE-PROBE", goneTaskID)

	gone := doCalDAV(t, app, http.MethodPut, caldavTaskPath("GONE-PROBE"),
		caldavTodoBody("GONE-PROBE", "Overwrite"))
	hidden := doCalDAV(t, app, http.MethodPut, caldavTaskPath("HIDDEN-PROBE"),
		caldavTodoBody("HIDDEN-PROBE", "Overwrite"))

	requireSameAnswer(t, hidden, gone)
	requireTaskState(t, app, hiddenTaskID, "Secret", "todo")
}

// TestCalDAV_DeleteOfHiddenEntityAnswersLikeMissing: a DELETE of a task the
// caller cannot read answers like a DELETE of one that does not exist, and the
// task survives, while a soft DELETE on a readable task succeeds. Covers
// both on_delete forms: the soft transition the test
// collection configures, and a hard delete.
func TestCalDAV_DeleteOfHiddenEntityAnswersLikeMissing(t *testing.T) {
	for _, tc := range []struct {
		name     string
		onDelete *dataentryconfig.CalDAVOnDelete
	}{
		{"soft on_delete", &dataentryconfig.CalDAVOnDelete{Set: map[string]string{"status": "done"}}},
		{"hard on_delete", &dataentryconfig.CalDAVOnDelete{Hard: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := caldavAppWithRowPolicy(t)
			coll := app.State().Cfg.CalDAV.Static["tasks"]
			coll.OnDelete = tc.onDelete
			app.State().Cfg.CalDAV.Static["tasks"] = coll

			seedCalDAVAlias(t, app, "HIDDEN-PROBE", hiddenTaskID)
			seedCalDAVAlias(t, app, "GONE-PROBE", goneTaskID)

			gone := doCalDAV(t, app, http.MethodDelete, caldavTaskPath("GONE-PROBE"), "")
			hidden := doCalDAV(t, app, http.MethodDelete, caldavTaskPath("HIDDEN-PROBE"), "")

			requireSameAnswer(t, hidden, gone)
			requireTaskState(t, app, hiddenTaskID, "Secret", "todo")

			// Control: the same DELETE on a readable task goes through. Soft
			// only: the readable task carries the `owns` edge that grants
			// access, so a non-cascading hard delete of it is refused for
			// that edge, which says nothing about the read gate.
			if tc.onDelete.Hard {
				return
			}
			seedCalDAVAlias(t, app, "MINE-PROBE", readableTaskID)
			mine := doCalDAV(t, app, http.MethodDelete, caldavTaskPath("MINE-PROBE"), "")
			if mine.Code >= 400 {
				t.Fatalf("DELETE on a readable task = %d, want success\n%s", mine.Code, mine.Body.String())
			}
			requireTaskState(t, app, readableTaskID, "Mine", "done")
		})
	}
}

// TestCalDAV_PutToReadableEntityStillSucceeds is the control: the same policy
// and the same PUT shape succeed on a task the caller can read, so the refusals
// above come from the read gate and not from broken wiring.
func TestCalDAV_PutToReadableEntityStillSucceeds(t *testing.T) {
	app := caldavAppWithRowPolicy(t)
	seedCalDAVAlias(t, app, "MINE-PROBE", readableTaskID)

	rec := doCalDAV(t, app, http.MethodPut, caldavTaskPath("MINE-PROBE"),
		caldavTodoBody("MINE-PROBE", "Mine edited"))
	if rec.Code >= 400 {
		t.Fatalf("PUT on a readable task = %d, want success\n%s", rec.Code, rec.Body.String())
	}
	requireTaskState(t, app, readableTaskID, "Mine edited", "todo")
}
