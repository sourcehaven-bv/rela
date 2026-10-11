package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// backlinksView is a feature view that collects the entry's backlinks into
// `mentions` and shows them in one section.
func backlinksView(where string, sec ViewSection) ViewConfig {
	sec.Source = "mentions"
	if sec.Heading == "" {
		sec.Heading = "Mentioned by"
	}
	return ViewConfig{
		Entry:    ViewEntry{Type: "feature"},
		Traverse: []ViewTraverse{{From: "entry", MentionedBy: true, CollectAs: "mentions", Where: where}},
		Sections: []ViewSection{sec},
	}
}

func decodeView(t *testing.T, code int, body []byte) v1.ViewResponse {
	t.Helper()
	if code != http.StatusOK {
		t.Fatalf("_views: got %d, want 200; body=%s", code, body)
	}
	var got v1.ViewResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v; body=%s", err, body)
	}
	return got
}

// TestViewMentions_TableGroupsByType covers the backlinks step with a
// `columns_by_type` table: hidden sources are absent, each type is a group
// with its own columns, grouped rows carry `_self`, and the built-in panel
// is switched off.
func TestViewMentions_TableGroupsByType(t *testing.T) {
	app, d := mentionedByApp(t, "ticket", "feature")
	seedEntity(app, &entity.Entity{ID: "FEAT-002", Type: "feature",
		Properties: map[string]any{"title": "other feature"}, Content: "see `FEAT-001`"})
	app.Cfg().Views["fv"] = backlinksView("", ViewSection{
		Display: "table",
		Columns: []ListColumn{{Property: "title"}},
		ColumnsByType: map[string][]ListColumn{
			"ticket": {{Property: "title", Link: "detail"}, {Property: "status"}},
		},
	})

	rec := viewsAs(aliceCtx(), t, app, d, "feature", "FEAT-001")
	got := decodeView(t, rec.Code, rec.Body.Bytes())
	if strings.Contains(rec.Body.String(), "REQ-001") || strings.Contains(rec.Body.String(), "SECRET") {
		t.Fatalf("LEAK: unreadable source in the view: %s", rec.Body)
	}
	if !got.HideMentionedByPanel {
		t.Error("hideMentionedByPanel = false for a view that collects backlinks")
	}
	sec := got.Sections[0]
	if !sec.IsGrouped || len(sec.Groups) != 2 || sec.Incomplete {
		t.Fatalf("section = %+v, want two complete groups", sec)
	}
	feat, tkt := sec.Groups[0], sec.Groups[1]
	if feat.EntityType != "feature" || len(feat.Columns) != 1 || len(feat.Rows) != 1 || feat.Rows[0].EntityID != "FEAT-002" {
		t.Errorf("feature group = %+v, want FEAT-002 with the fallback column", feat)
	}
	if tkt.EntityType != "ticket" || tkt.GroupName != "Tickets" || len(tkt.Columns) != 2 {
		t.Errorf("ticket group = %+v, want its own two columns", tkt)
	}
	if len(tkt.Rows) != 2 || tkt.Rows[0].EntityID != "TKT-001" || len(tkt.Rows[0].Cells) != 2 {
		t.Fatalf("ticket rows = %+v", tkt.Rows)
	}
	if tkt.Rows[0].Self == "" {
		t.Error("grouped row has no _self")
	}
}

// TestViewMentions_FieldsByType gives ticket rows their own fields and
// leaves other types on the section's fields.
func TestViewMentions_FieldsByType(t *testing.T) {
	app, d := mentionedByApp(t, "ticket", "feature")
	seedEntity(app, &entity.Entity{ID: "FEAT-002", Type: "feature",
		Properties: map[string]any{"title": "other feature"}, Content: "see `FEAT-001`"})
	app.Cfg().Views["fv"] = backlinksView("", ViewSection{
		Display:      "list",
		FieldsByType: map[string][]ViewSectionField{"ticket": {{Property: "status"}}},
	})

	got := featureView(t, app, d)
	ents := got.Sections[0].Entities
	if len(ents) != 3 || ents[0].ID != "FEAT-002" {
		t.Fatalf("entities = %+v", ents)
	}
	if len(ents[0].Fields) != 0 {
		t.Errorf("feature fields = %+v, want the section's (none)", ents[0].Fields)
	}
	if len(ents[1].Fields) != 1 || ents[1].Fields[0].Property != "status" {
		t.Errorf("ticket fields = %+v, want status", ents[1].Fields)
	}
}

// featureView fetches FEAT-001's view as alice.
func featureView(t *testing.T, app *App, d *acl.Declarative) v1.ViewResponse {
	t.Helper()
	rec := viewsAs(aliceCtx(), t, app, d, "feature", "FEAT-001")
	return decodeView(t, rec.Code, rec.Body.Bytes())
}

// seedMentioners adds n tickets that mention FEAT-001. Tickets above
// openFrom have status open; the rest are done.
func seedMentioners(app *App, n, openFrom int) {
	for i := 1; i <= n; i++ {
		status := "done"
		if i > openFrom {
			status = "open"
		}
		seedEntity(app, &entity.Entity{ID: fmt.Sprintf("TKT-%04d", i), Type: "ticket",
			Properties: map[string]any{"title": "t", "status": status}, Content: "`FEAT-001`"})
	}
}

// TestViewMentions_CapAndWhere pins the step limit and that `where:` filters
// while collecting, so matches past the first page are still found.
func TestViewMentions_CapAndWhere(t *testing.T) {
	cases := []struct {
		name           string
		n, openFrom    int
		where          string
		wantRows       int
		wantIncomplete bool
	}{
		{"under the cap", 5, 0, "", 5, false},
		{"over the cap", 201, 0, "", 200, true},
		{"where past the first page", 250, 200, "status = open", 50, false},
		{"where matches nothing", 30, 30, "status = open", 0, false},
		{"exactly the cap", 200, 0, "", 200, false},
		{"exactly the cap across pages", 450, 250, "status = open", 200, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature",
				Properties: map[string]any{"title": "the feature"}})
			seedMentioners(app, tc.n, tc.openFrom)
			d := mustNewACL(t, &acl.Policy{
				Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket", "feature"}}},
				Assignments: map[string]string{"alice": "viewer"},
			}, app.store)
			app.acl = d
			app.Cfg().Views["fv"] = backlinksView(tc.where, ViewSection{Display: "list"})

			sec := featureView(t, app, d).Sections[0]
			if len(sec.Entities) != tc.wantRows || sec.Incomplete != tc.wantIncomplete {
				t.Errorf("rows=%d incomplete=%v, want %d %v", len(sec.Entities), sec.Incomplete, tc.wantRows, tc.wantIncomplete)
			}
		})
	}
}

func TestViewMentions_PanelSwitch(t *testing.T) {
	app, d := mentionedByApp(t, "ticket", "feature")
	app.Cfg().Views["fv"] = ViewConfig{Entry: ViewEntry{Type: "feature"},
		Sections: []ViewSection{{Source: "entry", Display: "content"}}}
	if featureView(t, app, d).HideMentionedByPanel {
		t.Error("panel hidden on a view without a backlinks step")
	}
	off := false
	if app.Cfg().EntityViews == nil {
		app.Cfg().EntityViews = map[string]dataentryconfig.EntityViewConfig{}
	}
	app.Cfg().EntityViews["feature"] = dataentryconfig.EntityViewConfig{MentionedByPanel: &off}
	if !featureView(t, app, d).HideMentionedByPanel {
		t.Error("panel shown although entity_views.feature.mentioned_by_panel is false")
	}
}

// faultyMentions fails every backlink read.
type faultyMentions struct{ store.Store }

func (faultyMentions) MentionSources(context.Context, string, entity.Ref, int) ([]entity.Ref, error) {
	return nil, errors.New("index unavailable")
}

// faultyReload fails the batched header read of the backlink sources, after
// the index read has succeeded. The entry (F1) still loads.
type faultyReload struct{ store.Store }

func (f faultyReload) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	if len(q.IDs) > 0 && !slices.Contains(q.IDs, "F1") {
		return func(yield func(*entity.Entity, error) bool) { yield(nil, errors.New("connection reset")) }
	}
	return f.Store.ListEntities(ctx, q)
}

// TestViewMentions_FaultFailsTheView pins that an index fault is not shown
// as "nothing mentions this", and that the raw error stays off the wire.
func TestViewMentions_FaultFailsTheView(t *testing.T) {
	t.Run("index read", func(t *testing.T) {
		assertBacklinkFaultFails(t, faultyMentions{memstore.New()}, "index unavailable")
	})
	t.Run("source reload", func(t *testing.T) {
		assertBacklinkFaultFails(t, faultyReload{memstore.New()}, "connection reset")
	})
}

func assertBacklinkFaultFails(t *testing.T, st store.Store, raw string) {
	t.Helper()
	app, _, d, ctx := newBudgetAppOn(t, 3, st)
	app.Cfg().Views["fv"] = ViewConfig{
		Entry:    ViewEntry{Type: "feature"},
		Traverse: []ViewTraverse{{From: "entry", MentionedBy: true, CollectAs: "mentions"}},
		Sections: []ViewSection{{Heading: "Mentioned by", Source: "mentions", Display: "list"}},
	}
	rec := viewsAs(ctx, t, app, d, "feature", "F1")
	if rec.Code == http.StatusOK {
		t.Fatalf("_views answered 200 on an index fault: %s", rec.Body)
	}
	if strings.Contains(rec.Body.String(), raw) {
		t.Errorf("raw error on the wire: %s", rec.Body)
	}
}

// TestPropagateIncomplete pins that a collection derived from an incomplete
// one is incomplete too, directly or through `from: "*"`.
func TestPropagateIncomplete(t *testing.T) {
	rules := []ViewTraverse{
		{From: "entry", MentionedBy: true, CollectAs: "mentions"},
		{From: "mentions", Follow: "implements", CollectAs: "features"},
		{From: "features", Follow: "belongs-to", CollectAs: "cats"},
		{From: "entry", Follow: "blocks", CollectAs: "blocked"},
	}
	r := &viewResult{Incomplete: map[string]bool{"mentions": true}}
	r.propagateIncomplete(rules)
	for name, want := range map[string]bool{"mentions": true, "features": true, "cats": true, "blocked": false} {
		if r.Incomplete[name] != want {
			t.Errorf("Incomplete[%q] = %v, want %v", name, r.Incomplete[name], want)
		}
	}

	r = &viewResult{Incomplete: map[string]bool{"mentions": true}}
	r.propagateIncomplete(append(rules[:1:1], ViewTraverse{From: "*", Follow: "tagged", CollectAs: "labels"}))
	if !r.Incomplete["labels"] {
		t.Error(`a from: "*" step over an incomplete collection is not marked incomplete`)
	}

	r = &viewResult{}
	r.propagateIncomplete(rules)
	if len(r.Incomplete) != 0 {
		t.Errorf("Incomplete = %v, want none", r.Incomplete)
	}
}
