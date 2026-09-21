package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
		// Entities is read only to prove a row-filtering test is not vacuous.
		Entities []struct {
			ID string `json:"id"`
		} `json:"entities"`
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

// TestSectionCreate_ACLImplementationArms pins what the affordance does under
// the two non-declarative ACL implementations.
//
// This is the RR-CWWJGW shape, checked rather than assumed. The repo documents a
// fail-open hazard for predicates written against the READ gate: it returns
// nopReadGate under BOTH ReadOnlyACL and NopACL, and its HoldsPermission returns
// true, so such a predicate grants under `--read-only`. A UI gate that failed
// open there would render a create button on a server that refuses every write.
//
// This affordance derives from AuthorizeWrite, not the read gate, so it gets the
// right answer for the right reason — and both arms are load-bearing in opposite
// directions, which is why both are asserted:
//
//   - ReadOnlyACL denies every write, so the button must be ABSENT. Rendering it
//     would promise a verb the surface rejects.
//   - NopACL permits every write (no policy configured), so the button must be
//     PRESENT. Hiding it would make the feature dead for every deployment
//     without an acl.yaml — which is most of them.
func TestSectionCreate_ACLImplementationArms(t *testing.T) {
	tests := []struct {
		name      string
		impl      acl.ACL
		wantOffer bool
		why       string
	}{
		{
			name:      "ReadOnlyACL refuses every write",
			impl:      acl.ReadOnlyACL{},
			wantOffer: false,
			why:       "a create button on a read-only server promises a verb every write rejects",
		},
		{
			name:      "NopACL permits every write",
			impl:      acl.NopACL{},
			wantOffer: true,
			why:       "hiding it would make the feature dead wherever no acl.yaml is configured",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := seedCreateView(t, createViewOptions{create: &dataentryconfig.SectionCreate{}})
			app.acl = tc.impl

			body := fetchView(context.Background(), t, app)

			got := body.Sections[0].Create != nil
			if got != tc.wantOffer {
				t.Errorf("affordance present = %v, want %v — %s", got, tc.wantOffer, tc.why)
			}
		})
	}
}

// TestSectionCreate_CarriesNoBitAboutHiddenRows pins the confidentiality
// property of the affordance itself.
//
// The affordance appears on sections whose ROWS may be ACL-filtered, so the
// question is whether its presence says anything about what was filtered. It
// must not: it derives from the metamodel (which types the relation reaches),
// the form registry, and create permission — never from row data. A section
// that is empty because everything in it is hidden must look identical to one
// that is genuinely empty.
//
// That the affordance is unchanged while the ROWS differ is the whole assertion.
// Per CLAUDE.md the relation and type names are not secret (the metamodel is
// served over the API); what would be a real disclosure is the one-bit channel
// "something is hidden here", and this is what forecloses it.
func TestSectionCreate_CarriesNoBitAboutHiddenRows(t *testing.T) {
	// Two principals with the SAME create grant but different read grants, so
	// the rows differ and nothing else does.
	policy := func(read []string) *acl.Policy {
		return &acl.Policy{
			Roles: map[string]acl.RoleDef{"r": {
				Read:   read,
				Create: []string{"feature"},
			}},
			Assignments: map[string]string{"bob": "r"},
		}
	}

	var affordances []string
	var rowCounts []int
	for _, read := range [][]string{
		{"ticket", "feature"}, // sees the neighbor
		{"ticket"},            // neighbor is hidden — section renders empty
	} {
		app := seedCreateView(t, createViewOptions{create: &dataentryconfig.SectionCreate{}})
		d := mustNewACL(t, policy(read), app.store)
		app.acl = d
		// gateCtxFor, not a bare principal ctx: the ROW gate is what filters
		// neighbors, and a handler test bypasses the middleware that attaches
		// it. Without this the rows are unfiltered and the invariance below
		// compares two identical situations — which the anti-vacuity check at
		// the end catches, and did.
		ctx := gateCtxFor(principal.With(context.Background(),
			principal.Principal{User: "bob", Tool: principal.ToolDataEntry}), t, d)

		body := fetchView(ctx, t, app)
		sec := body.Sections[0]
		if sec.Create == nil {
			t.Fatalf("read=%v: the affordance must not depend on the read grant", read)
		}
		serialized, err := json.Marshal(sec.Create)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		affordances = append(affordances, string(serialized))
		rowCounts = append(rowCounts, len(sec.Entities))
	}

	// Anti-vacuity: the two principals really did see different rows, or this
	// test compares two identical situations and proves nothing.
	if rowCounts[0] == rowCounts[1] {
		t.Fatalf("both principals saw %d rows; the fixture is not exercising row "+
			"filtering, so the invariance below is meaningless", rowCounts[0])
	}
	if affordances[0] != affordances[1] {
		t.Errorf("the affordance differs with the row set:\n visible: %s\n hidden:  %s\n"+
			"that is a one-bit channel for whether rows were filtered", affordances[0], affordances[1])
	}
}

// TestSectionCreate_EdgeRefusedOnBothLinkDirections is AC10's relation half.
//
// The two `link_as` values take DIFFERENT server paths, which is the whole
// reason this is a table rather than one assertion:
//
//	from  a separate POST /{plural}/{id}/relations/{rel} → validateRelationOp
//	to    the edge rides the create body's `relations:`  → validateRelationsModernAffordances
//
// Only the first was gated. The second wrote an edge that `POST /relations/`
// refuses, and the create response reported `_relations: {"implements":
// {"creatable": false}}` for the edge it had just written. TKT-R4BMJM made that
// path reachable from a button — an outgoing section resolves to `link_as: to`.
//
// The verdict is DECLARED here on purpose. `validateRelationOp` is
// default-permissive for a relation type with no verdict entry
// (affordances.go), so the same test against a verdict-free schema would pass
// while exercising nothing — the trap the plan called out before either gate
// existed.
func TestSectionCreate_EdgeRefusedOnBothLinkDirections(t *testing.T) {
	const wantRule = `"rule_id":"relation-affordance:not-creatable:implements"`

	notCreatable := func(t *testing.T) *App {
		t.Helper()
		return seedTicketWithRelationVerdicts(t, RelationVerdicts{
			Types: map[string]RelationVerdict{"implements": {Creatable: false}},
		})
	}

	t.Run("link_as=from: separate POST to the relations endpoint", func(t *testing.T) {
		app := notCreatable(t)
		req := httptest.NewRequest(http.MethodPost,
			"/api/v1/tickets/TKT-001/relations/implements",
			strings.NewReader(`{"id":"FEAT-001"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		app.write.handleV1CreateRelation(rec, req, "ticket", entityRef{ID: "TKT-001"}, "implements")

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body)
		}
		if !strings.Contains(rec.Body.String(), wantRule) {
			t.Errorf("body must name the not-creatable rule: %s", rec.Body)
		}
	})

	t.Run("link_as=to: edge rides the create payload", func(t *testing.T) {
		app := notCreatable(t)
		body := `{"properties":{"title":"new"},` +
			`"relations":{"implements":{"data":[{"type":"feature","id":"FEAT-001"}]}}}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		app.write.handleV1CreateEntity(rec, req, "ticket", "tickets")

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403 — an edge refused on the relations endpoint "+
				"must not be writable by riding a create; body=%s", rec.Code, rec.Body)
		}
		if !strings.Contains(rec.Body.String(), wantRule) {
			t.Errorf("body must name the not-creatable rule: %s", rec.Body)
		}
		// Refused BEFORE the entity exists: denying after CreateEntity would
		// turn an authorization refusal into an orphaned entity.
		if strings.Contains(rec.Body.String(), `"id":"TKT-`) {
			t.Errorf("the refusal must not have created an entity: %s", rec.Body)
		}
	})

	t.Run("a creatable verdict still permits both", func(t *testing.T) {
		// The paired positive: the gate must not refuse the ordinary case.
		app := seedTicketWithRelationVerdicts(t, RelationVerdicts{
			Types: map[string]RelationVerdict{"implements": {Creatable: true, Removable: true}},
		})
		body := `{"properties":{"title":"new"},` +
			`"relations":{"implements":{"data":[{"type":"feature","id":"FEAT-001"}]}}}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		app.write.handleV1CreateEntity(rec, req, "ticket", "tickets")

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body)
		}
	})
}
