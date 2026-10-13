package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

func getMentionedByAs(ctx context.Context, t *testing.T, app *App, d *acl.Declarative,
	method, addr, rawQuery string,
) *httptest.ResponseRecorder {
	t.Helper()
	url := "/api/v1/features/" + addr + "/mentioned-by"
	if rawQuery != "" {
		url += "?" + rawQuery
	}
	req := httptest.NewRequest(method, url, http.NoBody).WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	handleV1MentionedBy(app, rec, req, "feature", addr)
	return rec
}

func decodeMentionedBy(t *testing.T, rec *httptest.ResponseRecorder) v1.MentionedByResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("mentioned-by: got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var got v1.MentionedByResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rec.Body)
	}
	return got
}

// mentionedByApp seeds FEAT-001 and three bodies that mention it: two
// tickets and a SECRET entity alice may not read.
func mentionedByApp(t *testing.T, readTypes ...string) (*App, *acl.Declarative) {
	t.Helper()
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature",
		Properties: map[string]any{"title": "the feature"}})
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "first ticket"}, Content: "implements `FEAT-001`"})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket",
		Properties: map[string]any{"title": "second ticket"}, Content: "also `FEAT-001`"})
	seedEntity(app, &entity.Entity{ID: "TKT-003", Type: "ticket",
		Properties: map[string]any{"title": "unrelated"}, Content: "nothing"})
	seedEntity(app, &entity.Entity{ID: "REQ-001", Type: "requirement",
		Properties: map[string]any{"title": "SECRET-REQ-TITLE"}, Content: "`FEAT-001`"})
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: readTypes}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)
	return app, d
}

func TestMentionedBy_ListsReadableSources(t *testing.T) {
	app, d := mentionedByApp(t, "ticket", "feature")
	rec := getMentionedByAs(aliceCtx(), t, app, d, http.MethodGet, "FEAT-001", "")
	got := decodeMentionedBy(t, rec)

	want := []v1.MentionSource{
		{ID: "TKT-001", Mention: v1.Mention{Type: "ticket", Title: "first ticket"}},
		{ID: "TKT-002", Mention: v1.Mention{Type: "ticket", Title: "second ticket"}},
	}
	if len(got.Items) != len(want) {
		t.Fatalf("items = %+v, want %+v", got.Items, want)
	}
	for i := range want {
		if got.Items[i] != want[i] {
			t.Errorf("item %d = %+v, want %+v", i, got.Items[i], want[i])
		}
	}
	if got.NextCursor != "" {
		t.Errorf("next_cursor = %q, want none", got.NextCursor)
	}
	if strings.Contains(rec.Body.String(), "REQ-001") || strings.Contains(rec.Body.String(), "SECRET") {
		t.Errorf("LEAK: unreadable source in the response: %s", rec.Body)
	}
}

// TestMentionedBy_HiddenTargetIs404 pins the uniform not-found: backlinks of
// an entity the caller cannot read answer exactly like a missing id.
func TestMentionedBy_HiddenTargetIs404(t *testing.T) {
	app, d := mentionedByApp(t, "ticket")
	hidden := getMentionedByAs(aliceCtx(), t, app, d, http.MethodGet, "FEAT-001", "")
	missing := getMentionedByAs(aliceCtx(), t, app, d, http.MethodGet, "FEAT-404", "")
	if hidden.Code != http.StatusNotFound || missing.Code != http.StatusNotFound {
		t.Fatalf("codes = %d, %d; want 404, 404", hidden.Code, missing.Code)
	}
	if strings.ReplaceAll(hidden.Body.String(), "FEAT-001", "X") != strings.ReplaceAll(missing.Body.String(), "FEAT-404", "X") {
		t.Errorf("hidden and missing differ:\n%s\n%s", hidden.Body, missing.Body)
	}
}

func TestMentionedBy_RedactsHiddenTitle(t *testing.T) {
	app, d := mentionedByApp(t, "ticket", "feature")
	setTestResolver(t, app, fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"title": false}}})
	rec := getMentionedByAs(aliceCtx(), t, app, d, http.MethodGet, "FEAT-001", "")
	got := decodeMentionedBy(t, rec)
	if len(got.Items) != 2 {
		t.Fatalf("items = %+v", got.Items)
	}
	if strings.Contains(rec.Body.String(), "first ticket") {
		t.Errorf("LEAK: redacted title in the response: %s", rec.Body)
	}
	if got.Items[0].Title != "TKT-001" {
		t.Errorf("title = %q, want the id", got.Items[0].Title)
	}
}

func TestMentionedBy_Pages(t *testing.T) {
	app, d := mentionedByApp(t, "ticket", "feature")
	first := decodeMentionedBy(t,
		getMentionedByAs(aliceCtx(), t, app, d, http.MethodGet, "FEAT-001", "limit=1"))
	if len(first.Items) != 1 || first.Items[0].ID != "TKT-001" || first.NextCursor == "" {
		t.Fatalf("first page = %+v", first)
	}
	second := decodeMentionedBy(t, getMentionedByAs(aliceCtx(), t, app, d, http.MethodGet,
		"FEAT-001", "limit=1&cursor="+first.NextCursor))
	if len(second.Items) != 1 || second.Items[0].ID != "TKT-002" {
		t.Fatalf("second page = %+v", second)
	}
}

func TestMentionedBy_RejectsBadInput(t *testing.T) {
	app, d := mentionedByApp(t, "ticket", "feature")
	for _, tc := range []struct {
		name, method, query string
		want                int
	}{
		{"zero limit", http.MethodGet, "limit=0", http.StatusBadRequest},
		{"limit over the cap", http.MethodGet, "limit=201", http.StatusBadRequest},
		{"non-numeric limit", http.MethodGet, "limit=x", http.StatusBadRequest},
		{"malformed cursor", http.MethodGet, "cursor=..%2Fetc", http.StatusBadRequest},
		{"POST", http.MethodPost, "", http.StatusMethodNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := getMentionedByAs(aliceCtx(), t, app, d, tc.method, "FEAT-001", tc.query)
			if rec.Code != tc.want {
				t.Errorf("got %d, want %d; body=%s", rec.Code, tc.want, rec.Body)
			}
		})
	}
}

// TestMentionedBy_FacedSourceFollowsTheWorld: a faced source is listed at
// the face the request's world serves, and only if that face's body is the
// one that mentions the target. The published world is the default here.
func TestMentionedBy_FacedSourceFollowsTheWorld(t *testing.T) {
	for _, tc := range []struct {
		name             string
		draft, published string
		wantID, wantFace string
	}{
		{name: "the served face mentions", draft: "nothing", published: "see `FEAT-1`",
			wantID: "POL-1", wantFace: "published"},
		{name: "only an unserved face mentions", draft: "see `FEAT-1`", published: "nothing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, _ := facedApp(t, nil)
			ctx := context.Background()
			for face, body := range map[entity.Face]string{"draft": tc.draft, "published": tc.published} {
				e, err := app.store.GetEntity(ctx, entity.Ref{ID: "POL-1", Face: face})
				if err != nil {
					t.Fatal(err)
				}
				e.Content = body
				if err := app.store.UpdateEntity(ctx, e); err != nil {
					t.Fatal(err)
				}
			}
			rec := httptest.NewRecorder()
			app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
				"/api/v1/features/FEAT-1/mentioned-by", http.NoBody))
			got := decodeMentionedBy(t, rec)
			if tc.wantID == "" {
				if len(got.Items) != 0 {
					t.Errorf("items = %+v, want none", got.Items)
				}
				return
			}
			if len(got.Items) != 1 || got.Items[0].ID != tc.wantID || got.Items[0].Face != tc.wantFace {
				t.Errorf("items = %+v, want %s@%s", got.Items, tc.wantID, tc.wantFace)
			}
		})
	}
}
