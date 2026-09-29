package dataentry

import (
	"context"
	"errors"
	"iter"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The guards below close the review findings on BUG-4SYAA6: a restore that
// recreates a deleted face never overwrites a live one, a failed read never
// counts as "deleted", and a deleted face's history needs the same gates as
// any read of the entity it belongs to.

// historyEditors may read, update and create every ticket face and holds
// the global history permission.
func historyEditors(t *testing.T) func(store.Store) *acl.Declarative {
	t.Helper()
	return func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{"editor": {
				Read: allTicketFaces, Update: allTicketFaces, Create: allTicketFaces,
				Permissions: []string{string(acl.PermHistoryRead)},
			}},
			Assignments: map[string]string{"bob": "editor"},
		}, st)
	}
}

// TestFacedHistory_RecreateNeverOverwritesALiveFace: the restore decided the
// face was deleted, then another writer recreated it before the write. The
// write is create-only, so the answer is a 409 and the live row is
// untouched, not a whole-record overwrite of it.
func TestFacedHistory_RecreateNeverOverwritesALiveFace(t *testing.T) {
	app, d := facedHistoryApp(t, historyEditors(t))
	snap := facedSnapshot("draft", "Travel")

	// TKT-1@draft is live: this is the state after the concurrent recreate.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/_history/ticket/TKT-1@draft/1/restore", http.NoBody)
	req = req.WithContext(gateCtxFor(principalCtx("bob"), t, d))
	rec := httptest.NewRecorder()
	restoreRecreate(app, rec, req, &snap, entity.Ref{ID: "TKT-1", Face: "draft"})

	if rec.Code != http.StatusConflict {
		t.Fatalf("recreate onto a live face = %d, want 409 (%s)", rec.Code, rec.Body)
	}
	if got := titleAt(t, app, "draft"); got != "Travel v2" {
		t.Errorf("draft title = %q; the live row must be left as it was", got)
	}
}

// failingHeaders fails every stored-faces read, the way a store fault does.
type failingHeaders struct{ store.Store }

var errHeaders = errors.New("headers unavailable")

func (failingHeaders) ListEntityHeaders(context.Context, store.EntityQuery) iter.Seq2[store.EntityHeader, error] {
	return func(yield func(store.EntityHeader, error) bool) { yield(store.EntityHeader{}, errHeaders) }
}

// TestFacedHistory_StoredFacesReadErrorIsAnError: the deleted-face rule
// opens history on a global permission, so a failed read of the stored
// faces must surface as an error. Answered as "nothing stored", it would
// route a live face the caller cannot see to that rule.
func TestFacedHistory_StoredFacesReadErrorIsAnError(t *testing.T) {
	app, d := facedHistoryApp(t, historyEditors(t))
	if _, err := app.store.DeleteFace(context.Background(), entity.Ref{ID: "TKT-1", Face: "draft"}); err != nil {
		t.Fatalf("delete draft: %v", err)
	}
	ctx := gateCtxFor(principalCtx("bob"), t, d)
	ref := entity.Ref{ID: "TKT-1", Face: "draft"}

	if _, ok, err := resolveHistorySubject(ctx, app.visibleReader, "ticket", ref); err != nil || !ok {
		t.Fatalf("control: deleted face = ok %v, err %v; want a readable subject", ok, err)
	}

	// The resolver keeps its working store; only the stored-faces read fails.
	vr := app.visibleReader
	vr.store = failingHeaders{Store: app.store}
	_, ok, err := resolveHistorySubject(ctx, vr, "ticket", ref)
	if !errors.Is(err, errHeaders) || ok {
		t.Errorf("failed stored-faces read = ok %v, err %v; want the read error", ok, err)
	}
}

// TestFacedHistory_DeniedWorldHidesNamedFaces: in a world the caller may not
// read, a named face is the uniform 404 whether it is live or deleted, so the
// answer does not tell the two apart.
func TestFacedHistory_DeniedWorldHidesNamedFaces(t *testing.T) {
	app, d := facedHistoryApp(t, historyEditors(t))
	if _, err := app.store.DeleteFace(context.Background(), entity.Ref{ID: "TKT-1", Face: "draft"}); err != nil {
		t.Fatalf("delete draft: %v", err)
	}
	ctx := withWorld(gateCtxFor(principalCtx("bob"), t, d), worldHandle{name: "published", denied: true})
	for _, face := range []entity.Face{"draft", "published"} {
		_, ok, err := resolveHistorySubject(ctx, app.visibleReader, "ticket", entity.Ref{ID: "TKT-1", Face: face})
		if err != nil || ok {
			t.Errorf("TKT-1@%s in a denied world = ok %v, err %v; want the uniform miss", face, ok, err)
		}
	}
}

// TestFacedHistory_DeletedFaceGates covers who may read a deleted face's
// history.
func TestFacedHistory_DeletedFaceGates(t *testing.T) {
	draftOnly := func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{
				"draft-historian": {
					Read:        []string{"ticket@draft"},
					Permissions: []string{string(acl.PermHistoryRead)},
				},
				"historian": {
					Read:        allTicketFaces,
					Permissions: []string{string(acl.PermHistoryRead)},
				},
			},
			Assignments: map[string]string{"dora": "draft-historian", "hank": "historian"},
		}, st)
	}
	app, d := facedHistoryApp(t, draftOnly)
	if _, err := app.store.DeleteFace(context.Background(), entity.Ref{ID: "TKT-1", Face: "draft"}); err != nil {
		t.Fatalf("delete draft: %v", err)
	}
	versions := facedHistory()
	versions.versions["TKT-2@draft"] = []store.VersionSnapshot{func() store.VersionSnapshot {
		s := snapshot("note", "a note", map[string]any{"title": "Note"})
		s.Face = "draft"
		return s
	}()}
	app.versions = versions

	for _, tc := range []struct {
		name string
		user string
		path string
		want int
		why  string
	}{
		{
			"sibling readable", "hank", "TKT-1@draft", http.StatusOK,
			"a deleted face of an entity the caller can read is served",
		},
		{
			"no live face readable", "dora", "TKT-1@draft", http.StatusNotFound,
			"the entity still lives, so its row gate applies; dora can read none of its live faces",
		},
		{
			"no lineage", "hank", "TKT-9@draft", http.StatusNotFound,
			"an empty lineage is the same 404 as a hidden live face",
		},
		{
			"lineage of another type", "hank", "TKT-2@draft", http.StatusNotFound,
			"a ticket face grant must not open a note's lineage",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if rec := historyAs(principalCtx(tc.user), t, app, d, http.MethodGet, tc.path); rec.Code != tc.want {
				t.Errorf("GET %s as %s = %d, want %d: %s (%s)", tc.path, tc.user, rec.Code, tc.want, tc.why, rec.Body)
			}
		})
	}
}
