package dataentry

import (
	"context"
	"net/http"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// stateTicketYAML is the faced ticket of facedHistoryApp with a status state
// machine whose entry value is `open`.
const stateTicketYAML = `
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    faces:
      draft: {}
      published: {}
      review: {}
    properties:
      title: {type: string, required: true}
      status: {type: ticket-status}
types:
  ticket-status:
    values: [open, doing, done]
    initial: open
    transitions:
      - {from: open, to: doing}
      - {from: doing, to: done}
`

// stateTicketApp serves a deleted `draft` face whose one recorded version is
// `done`, with bob holding read, update and history read on every face,
// create on createFaces, and statusOptions (when non-nil) as bob's `options:`
// grant on status.
func stateTicketApp(t *testing.T, createFaces, statusOptions []string) (*App, *acl.Declarative) {
	t.Helper()
	return stateTicketAppFrom(t, stateTicketYAML, createFaces, statusOptions, nil)
}

// stateTicketAppFrom is stateTicketApp over schemaYAML, with perms as bob's
// extra global permissions.
func stateTicketAppFrom(
	t *testing.T, schemaYAML string, createFaces, statusOptions, perms []string,
) (*App, *acl.Declarative) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(schemaYAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	snap := snapshot("ticket", "body", map[string]any{"title": "Travel", "status": "done"})
	snap.Face = "draft"
	history := historyStore{versions: map[string][]store.VersionSnapshot{"TKT-1@draft": {snap}}}

	app, d := facedHistoryAppWith(t, meta, history, func(st store.Store) *acl.Declarative {
		role := acl.RoleDef{
			Read: allTicketFaces, Update: allTicketFaces, Create: createFaces,
			Permissions: append([]string{string(acl.PermHistoryRead)}, perms...),
		}
		for _, opt := range statusOptions {
			if role.Options == nil {
				role.Options = map[string][]acl.OptionGrant{}
			}
			role.Options["ticket"] = append(role.Options["ticket"], acl.OptionGrant{Field: "status", Option: opt})
		}
		return mustNewACL(t, &acl.Policy{
			Roles:       map[string]acl.RoleDef{"editor": role},
			Assignments: map[string]string{"bob": "editor"},
		}, st)
	})
	// The real field resolver, so the restore's per-field gate applies the
	// `options:` grant.
	resolver, err := affordances.New(meta, storeRelationLookup{st: app.store}, d)
	if err != nil {
		t.Fatalf("affordances.New: %v", err)
	}
	app.fieldResolver = &policyResolver{inner: resolver}
	if _, err := app.store.DeleteFace(context.Background(), entity.Ref{ID: "TKT-1", Face: "draft"}); err != nil {
		t.Fatalf("delete draft: %v", err)
	}
	return app, d
}

// BUG-KK1UXH over HTTP: restoring a deleted face whose recorded status is
// past the entry value brings it back at that status. It used to be a 422,
// because the recreate applied the entry rule of a new record.
func TestHistoryRestore_DeletedFacePastEntryState(t *testing.T) {
	app, d := stateTicketApp(t, allTicketFaces, nil)

	rec := historyAs(principalCtx("bob"), t, app, d, http.MethodPost, "TKT-1@draft/1/restore")
	if rec.Code != http.StatusOK {
		t.Fatalf("restore of a deleted `done` face = %d (%s)", rec.Code, rec.Body)
	}
	got, err := app.store.GetEntity(context.Background(), entity.Ref{ID: "TKT-1", Face: "draft"})
	if err != nil {
		t.Fatalf("read the restored face: %v", err)
	}
	if status := got.GetString("status"); status != "done" {
		t.Errorf("restored status = %q, want %q", status, "done")
	}
}

// The exemption is from the entry rule only, not from the field gate: a
// principal whose `options:` grant does not include the recorded status
// cannot restore it.
func TestHistoryRestore_PastEntryStateStillFieldGated(t *testing.T) {
	app, d := stateTicketApp(t, allTicketFaces, []string{"open", "doing"})

	rec := historyAs(principalCtx("bob"), t, app, d, http.MethodPost, "TKT-1@draft/1/restore")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("restore of a status outside bob's options = %d, want 403 (%s)", rec.Code, rec.Body)
	}
	if _, err := app.store.GetEntity(context.Background(), entity.Ref{ID: "TKT-1", Face: "draft"}); err == nil {
		t.Error("the denied restore recreated the face")
	}
}

// guardedTicketYAML is stateTicketYAML with the edge into `done` guarded.
const guardedTicketYAML = `
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    faces:
      draft: {}
      published: {}
      review: {}
    properties:
      title: {type: string, required: true}
      status: {type: ticket-status}
types:
  ticket-status:
    values: [open, doing, done]
    initial: open
    transitions:
      - {from: open, to: doing}
      - {from: doing, to: done, guard: finish}
`

// The transition guards bind over HTTP too: bob may restore the `done`
// version only while he holds the guard of the edge into `done`.
func TestHistoryRestore_PastEntryStateNeedsGuard(t *testing.T) {
	tests := []struct {
		name  string
		perms []string
		want  int
	}{
		{"guard not held", nil, http.StatusForbidden},
		{"guard held", []string{"finish"}, http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app, d := stateTicketAppFrom(t, guardedTicketYAML, allTicketFaces, nil, tc.perms)

			rec := historyAs(principalCtx("bob"), t, app, d, http.MethodPost, "TKT-1@draft/1/restore")
			if rec.Code != tc.want {
				t.Fatalf("restore = %d, want %d (%s)", rec.Code, tc.want, rec.Body)
			}
			_, err := app.store.GetEntity(context.Background(), entity.Ref{ID: "TKT-1", Face: "draft"})
			if restored := err == nil; restored != (tc.want == http.StatusOK) {
				t.Errorf("face restored = %v, want %v", restored, tc.want == http.StatusOK)
			}
		})
	}
}

// Nor from the create grant: the recreate is authorized as a create on the
// face, so a principal without create on `draft` cannot restore it.
func TestHistoryRestore_PastEntryStateNeedsCreate(t *testing.T) {
	app, d := stateTicketApp(t, []string{"ticket@published", "ticket@review"}, nil)

	rec := historyAs(principalCtx("bob"), t, app, d, http.MethodPost, "TKT-1@draft/1/restore")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("restore without create on the face = %d, want 403 (%s)", rec.Code, rec.Body)
	}
	if _, err := app.store.GetEntity(context.Background(), entity.Ref{ID: "TKT-1", Face: "draft"}); err == nil {
		t.Error("the denied restore recreated the face")
	}
}
