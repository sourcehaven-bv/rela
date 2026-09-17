package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// createViewOptions configures the fixture below.
type createViewOptions struct {
	// create is the section's opt-in block; nil leaves the section read-only.
	create *dataentryconfig.SectionCreate
	// traverse overrides the default outgoing `implements` rule.
	traverse *ViewTraverse
	// entryType/otherType default to ticket -> feature.
	skipFeatureForm bool
}

// seedCreateView builds a ticket entry with one `implements` section, seeds the
// entities, and returns the app ready to serve `_views`.
func seedCreateView(t *testing.T, opts createViewOptions) *App {
	t.Helper()
	app := newTestAppV1(t)
	state := app.State()
	if !opts.skipFeatureForm {
		state.Cfg.Forms["create_feature"] = dataentryconfig.Form{EntityType: "feature"}
	}
	state.Cfg.Forms["create_ticket"] = dataentryconfig.Form{EntityType: "ticket"}

	tr := ViewTraverse{From: "entry", Follow: "implements", CollectAs: "features"}
	if opts.traverse != nil {
		tr = *opts.traverse
	}
	state.Cfg.Views["v"] = ViewConfig{
		Entry:    ViewEntry{Type: "ticket"},
		Traverse: []ViewTraverse{tr},
		Sections: []ViewSection{{
			Heading: "Features",
			Source:  tr.CollectAs,
			Display: "cards",
			Create:  opts.create,
		}},
	}

	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "entry"},
	})
	seedEntity(app, &entity.Entity{
		ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "other"},
	})
	seedRelation(app, &entity.Relation{From: "TKT-001", Type: "implements", To: "FEAT-001"})
	return app
}

// fetchView serves the detail view and decodes the response.
//
// ctx carries the principal when a test is exercising the ACL gate; a plain
// Background context exercises the default (no ACL configured) path.
func fetchView(ctx context.Context, t *testing.T, app *App) v1ViewBody {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_views/ticket/TKT-001", http.NoBody)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	app.views.handleV1Views(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	var body v1ViewBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode view response: %v", err)
	}
	return body
}

// v1ViewBody is a decode target narrow enough to assert on without coupling to
// every field of the real response.
type v1ViewBody struct {
	Create   []viewCreateBody `json:"create"`
	Sections []struct {
		Heading string          `json:"heading"`
		Create  *viewCreateBody `json:"create"`
	} `json:"sections"`
}

type viewCreateBody struct {
	Relation string `json:"relation"`
	LinkAs   string `json:"linkAs"`
	PeerID   string `json:"peerId"`
	Flow     string `json:"flow"`
	Heading  string `json:"heading"`
	Targets  []struct {
		EntityType string `json:"entityType"`
		FormID     string `json:"formId"`
		Label      string `json:"label"`
		Template   string `json:"template"`
	} `json:"targets"`
}

// TestSectionCreate_AbsentWithoutOptIn is the companion to the guard test in
// api_v1_test.go, asserted through the typed path rather than the raw-key one.
// It is the DEFAULT, and TKT-651W's invariant, so it is worth pinning twice.
func TestSectionCreate_AbsentWithoutOptIn(t *testing.T) {
	app := seedCreateView(t, createViewOptions{create: nil})
	body := fetchView(context.Background(), t, app)

	if body.Sections[0].Create != nil {
		t.Error("a section with no create: block must carry no create affordance")
	}
	if len(body.Create) != 0 {
		t.Error("the header menu must be absent when no section opted in")
	}
}

// TestSectionCreate_OptInEmitsAffordance covers AC1 and AC2's single-type case:
// the opt-in works, and the server resolves everything the client needs.
func TestSectionCreate_OptInEmitsAffordance(t *testing.T) {
	app := seedCreateView(t, createViewOptions{create: &dataentryconfig.SectionCreate{}})
	body := fetchView(context.Background(), t, app)

	got := body.Sections[0].Create
	if got == nil {
		t.Fatal("a section with create: {} must carry the affordance")
	}
	if got.Relation != "implements" {
		t.Errorf("relation = %q, want %q", got.Relation, "implements")
	}
	// Outgoing traversal: the NEW entity is the TO of the edge.
	if got.LinkAs != "to" {
		t.Errorf("linkAs = %q, want %q — an outgoing section creates the edge's target", got.LinkAs, "to")
	}
	if got.PeerID != "TKT-001" {
		t.Errorf("peerId = %q, want the entry id", got.PeerID)
	}
	if got.Flow != "modal" {
		t.Errorf("flow = %q, want the modal default", got.Flow)
	}
	if len(got.Targets) != 1 || got.Targets[0].EntityType != "feature" {
		t.Fatalf("targets = %+v, want exactly the one reachable type", got.Targets)
	}
	if got.Targets[0].FormID != "create_feature" {
		t.Errorf("formId = %q, want the server-resolved form", got.Targets[0].FormID)
	}
	// Not placed in the header unless asked.
	if len(body.Create) != 0 {
		t.Error("header menu must stay empty when in: defaults to [section]")
	}
}

// TestSectionCreate_IncomingSectionLinksAsFrom pins the direction that decides
// whether the created edge points the right way. Getting this backwards
// produces a link that looks present and is wrong.
func TestSectionCreate_IncomingSectionLinksAsFrom(t *testing.T) {
	app := newTestAppV1(t)
	state := app.State()
	state.Cfg.Forms["create_ticket"] = dataentryconfig.Form{EntityType: "ticket"}
	state.Cfg.Views["v"] = ViewConfig{
		Entry:    ViewEntry{Type: "feature"},
		Traverse: []ViewTraverse{{From: "entry", FollowIncoming: "implements", CollectAs: "tickets"}},
		Sections: []ViewSection{{
			Heading: "Tickets", Source: "tickets", Display: "cards",
			Create: &dataentryconfig.SectionCreate{},
		}},
	}
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "f"}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_views/feature/FEAT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.views.handleV1Views(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body)
	}
	var body v1ViewBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	got := body.Sections[0].Create
	if got == nil {
		t.Fatal("incoming section with create: must carry the affordance")
	}
	if got.LinkAs != "from" {
		t.Errorf("linkAs = %q, want %q — an incoming section creates the edge's source", got.LinkAs, "from")
	}
	if len(got.Targets) != 1 || got.Targets[0].EntityType != "ticket" {
		t.Errorf("targets = %+v, want the relation's FROM types", got.Targets)
	}
}

// TestSectionCreate_PerTypeTemplateIsResolvedServerSide covers AC6. The mapping
// from config to type happens on the server so the client never re-derives it.
func TestSectionCreate_PerTypeTemplateIsResolvedServerSide(t *testing.T) {
	app := seedCreateView(t, createViewOptions{create: &dataentryconfig.SectionCreate{
		Types: map[string]dataentryconfig.SectionCreateTarget{
			"feature": {Template: "spike"},
		},
	}})
	body := fetchView(context.Background(), t, app)

	targets := body.Sections[0].Create.Targets
	if len(targets) != 1 {
		t.Fatalf("targets = %+v, want 1", targets)
	}
	if targets[0].Template != "spike" {
		t.Errorf("template = %q, want %q", targets[0].Template, "spike")
	}
}

// TestSectionCreate_HeaderMenuIsUnionOfOptedInSections covers AC7, including the
// dedup rule: two sections over one relation are one menu entry.
func TestSectionCreate_HeaderMenuIsUnionOfOptedInSections(t *testing.T) {
	app := newTestAppV1(t)
	state := app.State()
	state.Cfg.Forms["create_feature"] = dataentryconfig.Form{EntityType: "feature"}
	state.Cfg.Views["v"] = ViewConfig{
		Entry:    ViewEntry{Type: "ticket"},
		Traverse: []ViewTraverse{{From: "entry", Follow: "implements", CollectAs: "features"}},
		Sections: []ViewSection{
			{
				Heading: "Features", Source: "features", Display: "cards",
				Create: &dataentryconfig.SectionCreate{
					In: []string{dataentryconfig.SectionCreateInHeader},
				},
			},
			{
				// Same relation, also in the header: must NOT produce a second
				// menu entry, or the user sees "New Feature" twice.
				Heading: "Features again", Source: "features", Display: "list",
				Create: &dataentryconfig.SectionCreate{
					In: []string{dataentryconfig.SectionCreateInHeader},
				},
			},
			{
				// Opted in, but section-only: must not reach the header.
				Heading: "Section only", Source: "features", Display: "list",
				Create: &dataentryconfig.SectionCreate{},
			},
		},
	}
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "t"}})

	body := fetchView(context.Background(), t, app)
	if len(body.Create) != 1 {
		t.Fatalf("header menu = %+v, want exactly one entry (deduped by relation)", body.Create)
	}
	if body.Create[0].Heading != "Features" {
		t.Errorf("heading = %q, want the first opted-in section's", body.Create[0].Heading)
	}
	if body.Create[0].Relation != "implements" {
		t.Errorf("relation = %q, want %q", body.Create[0].Relation, "implements")
	}
}

// TestSectionCreate_NoAffordanceWithoutCreateForm covers the other half of the
// derivation: a type nothing can create is not offered, even when opted in.
func TestSectionCreate_NoAffordanceWithoutCreateForm(t *testing.T) {
	app := seedCreateView(t, createViewOptions{
		create:          &dataentryconfig.SectionCreate{},
		skipFeatureForm: true,
	})
	body := fetchView(context.Background(), t, app)

	if body.Sections[0].Create != nil {
		t.Error("a type with no create form must not be offered")
	}
}

// TestSectionCreate_AmbiguousSectionGetsNoAffordance covers the runtime half of
// the "no single relation" rule. Config load refuses these, but the resolver
// must not depend on that — a synthesized or hand-edited config could reach it.
func TestSectionCreate_AmbiguousSectionGetsNoAffordance(t *testing.T) {
	app := seedCreateView(t, createViewOptions{
		create:   &dataentryconfig.SectionCreate{},
		traverse: &ViewTraverse{From: "entry", Follow: "implements", CollectAs: "features", Recursive: true},
	})
	body := fetchView(context.Background(), t, app)

	if body.Sections[0].Create != nil {
		t.Error("a recursive section has no single peer to link to; it must get no affordance")
	}
}

// TestSectionCreate_GatedByCreatePermission covers AC1's negative half and AC10's
// affordance side: the button is derived from the principal's create permission,
// not merely from a form existing.
//
// This is the check that did NOT exist before TKT-R4BMJM — the resolver offered
// a target whenever a form was configured, with no principal involved.
func TestSectionCreate_GatedByCreatePermission(t *testing.T) {
	tests := []struct {
		name       string
		createable []string
		wantOffer  bool
	}{
		{name: "principal may create the target", createable: []string{"feature"}, wantOffer: true},
		{name: "principal may not create the target", createable: nil, wantOffer: false},
		{
			name:       "principal may create something else entirely",
			createable: []string{"ticket"},
			wantOffer:  false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := seedCreateView(t, createViewOptions{create: &dataentryconfig.SectionCreate{}})

			d := mustNewACL(t, &acl.Policy{
				Roles: map[string]acl.RoleDef{"r": {
					Read:   []string{"ticket", "feature"},
					Create: tc.createable,
				}},
				Assignments: map[string]string{"bob": "r"},
			}, app.store)
			app.acl = d

			ctx := principal.With(context.Background(),
				principal.Principal{User: "bob", Tool: principal.ToolDataEntry})
			body := fetchView(ctx, t, app)

			got := body.Sections[0].Create != nil
			if got != tc.wantOffer {
				t.Errorf("affordance present = %v, want %v — the button must follow create permission, "+
					"not merely the existence of a form", got, tc.wantOffer)
			}
		})
	}
}
