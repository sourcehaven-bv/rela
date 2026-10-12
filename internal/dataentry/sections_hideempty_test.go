package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// Tests for `hide_empty:` on view sections (TKT-JT101B).

// hideEmptyView configures a view on TKT-001 with two collections: features
// (TKT-001 implements FEAT-001, so one row) and inbound (nothing implements a
// ticket, so always empty).
func hideEmptyView(t *testing.T, sections []ViewSection) *App {
	t.Helper()
	app := seedCreateView(t, createViewOptions{})
	app.State().Cfg.Views["v"] = ViewConfig{
		Entry: ViewEntry{Type: "ticket"},
		Traverse: []ViewTraverse{
			{From: "entry", Follow: "implements", CollectAs: "features"},
			{From: "entry", FollowIncoming: "implements", CollectAs: "inbound"},
		},
		Sections: sections,
	}
	return app
}

func sectionHeadings(body v1ViewBody) []string {
	out := make([]string, 0, len(body.Sections))
	for _, s := range body.Sections {
		out = append(out, s.Heading)
	}
	return out
}

// TestHideEmpty_DropsOnlyEmptyOptedInSections pins the three cases: an empty
// section that opted in is gone, an empty section that did not opt in still
// renders, and a section with rows renders whatever it says.
func TestHideEmpty_DropsOnlyEmptyOptedInSections(t *testing.T) {
	app := hideEmptyView(t, []ViewSection{
		{Heading: "Features", Source: "features", Display: "cards", HideEmpty: true},
		{Heading: "Hidden", Source: "inbound", Display: "cards", HideEmpty: true},
		{Heading: "Shown", Source: "inbound", Display: "cards"},
	})
	body := fetchView(context.Background(), t, app)

	got := sectionHeadings(body)
	want := []string{"Features", "Shown"}
	if !slices.Equal(got, want) {
		t.Fatalf("sections = %v, want %v", got, want)
	}
	if len(body.Sections[0].Entities) != 1 {
		t.Errorf("Features has %d rows, want 1", len(body.Sections[0].Entities))
	}
}

// TestHideEmpty_KeepsCreateOnLaterSection guards the index alignment: the
// create resolver walks sections in step with the view config, so dropping a
// section before it runs would attach the button to the wrong section.
func TestHideEmpty_KeepsCreateOnLaterSection(t *testing.T) {
	app := hideEmptyView(t, []ViewSection{
		{Heading: "Hidden", Source: "inbound", Display: "cards", HideEmpty: true},
		{Heading: "Features", Source: "features", Display: "cards", Create: &dataentryconfig.SectionCreate{}},
	})
	body := fetchView(context.Background(), t, app)

	if len(body.Sections) != 1 || body.Sections[0].Heading != "Features" {
		t.Fatalf("sections = %v, want only Features", sectionHeadings(body))
	}
	if c := body.Sections[0].Create; c == nil || c.Relation != "implements" {
		t.Errorf("Features lost its create affordance: %+v", c)
	}
}

// TestOmitHiddenEmpty_KeepsIncomplete: a section whose collection is only
// partly filled is not known to be empty, so it stays.
func TestOmitHiddenEmpty_KeepsIncomplete(t *testing.T) {
	got := omitHiddenEmpty([]SectionData{
		{Heading: "a", HideEmpty: true, IsEmpty: true, Incomplete: true},
		{Heading: "b", HideEmpty: true, IsEmpty: true},
	})
	if len(got) != 1 || got[0].Heading != "a" {
		t.Fatalf("got %+v, want only the incomplete section", got)
	}
}

// TestHideEmpty_FollowsReadAccess pins that emptiness is decided after the
// read gate: a principal who cannot read the only row sees an empty section,
// so it hides for them and stays for a reader.
func TestHideEmpty_FollowsReadAccess(t *testing.T) {
	tests := []struct {
		name     string
		readable []string
		want     []string
	}{
		{name: "reader of the row", readable: []string{"ticket", "feature"}, want: []string{"Features"}},
		{name: "row not readable", readable: []string{"ticket"}, want: []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := hideEmptyView(t, []ViewSection{
				{Heading: "Features", Source: "features", Display: "cards", HideEmpty: true},
			})
			d := mustNewACL(t, &acl.Policy{
				Roles:       map[string]acl.RoleDef{"r": {Read: tc.readable}},
				Assignments: map[string]string{"bob": "r"},
			}, app.store)
			app.acl = d

			rec := viewsAs(principalCtx("bob"), t, app, d, "ticket", "TKT-001")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
			}
			var body v1ViewBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if got := sectionHeadings(body); !slices.Equal(got, tc.want) {
				t.Errorf("sections = %v, want %v", got, tc.want)
			}
		})
	}
}
