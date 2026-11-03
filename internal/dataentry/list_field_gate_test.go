package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// listGateACL hides `status` from the viewer outright (a closed-world
// `visible:` block that does not name it), shows it to the auditor on some
// rows only (a `when:` grant), and grants it to hr on every row.
const listGateACL = `
roles:
  viewer:
    read: [ticket]
    visible:
      ticket:
        - field: title
  auditor:
    read: [ticket]
    visible:
      ticket:
        - field: title
        - field: status
          when: "entity.title == 'Alpha'"
  hr:
    read: [ticket]
    visible:
      ticket:
        - field: title
        - field: status
assignments:
  alice: viewer
  audrey: auditor
  harry: hr
`

// seedListGateTickets stores two tickets whose status values are the secret
// a filter or sort on a hidden field would disclose.
func seedListGateTickets(app *App) {
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "Alpha", "status": "bravo-secret"},
	})
	seedEntity(app, &entity.Entity{
		ID: "TKT-002", Type: "ticket",
		Properties: map[string]any{"title": "Charlie", "status": "alpha-secret"},
	})
}

// hiddenFieldQueries is every list query that reads a property: each filter
// operator, the array form, and the sort in both directions and as a
// secondary key. Each names `status`.
var hiddenFieldQueries = []struct{ name, query string }{
	{"eq", "filter[status]=bravo-secret"},
	{"eq-explicit", "filter[status][eq]=bravo-secret"},
	{"eq-empty", "filter[status]="},
	{"ne", "filter[status][ne]=bravo-secret"},
	{"contains", "filter[status][contains]=brav"},
	{"in", "filter[status][in]=bravo-secret,x"},
	{"in-array", "filter[status][in][]=bravo-secret"},
	{"lt", "filter[status][lt]=b"},
	{"lte", "filter[status][lte]=b"},
	{"gt", "filter[status][gt]=b"},
	{"gte", "filter[status][gte]=b"},
	{"sort", "sort=status"},
	{"sort-desc", "sort=-status"},
	{"sort-secondary", "sort=title,status"},
	{"filter-and-sort", "filter[title]=Alpha&sort=status"},
}

// TestListFieldGate_HiddenPropertyRejected pins MEAS-LIST-FILTER-SORT-GATED
// (BUG-WJSYQT): a filter or sort on a property the principal cannot read is
// refused with a 400 naming the property, for every operator. Before the fix
// each of these evaluated the raw value, so `contains`, `lt` and `gt` recovered
// a hidden value by binary search and `sort` ordered rows by it.
func TestListFieldGate_HiddenPropertyRejected(t *testing.T) {
	app, d := buildFieldPolicyApp(t, listGateACL)
	seedListGateTickets(app)

	for _, tc := range hiddenFieldQueries {
		t.Run(tc.name, func(t *testing.T) {
			_, rec := listEntitiesAs(aliceCtx(), t, app, d, "ticket", "tickets", tc.query)
			assertUnreadablePropertyRejected(t, rec, "status")
		})
	}
}

// TestListFieldGate_ConditionalVisibilityRejected pins the `when:` decision: a
// property visible on SOME rows only is refused like a hidden one. The auditor
// sees TKT-001's status, but a filter would still be evaluated on TKT-002's
// hidden value.
func TestListFieldGate_ConditionalVisibilityRejected(t *testing.T) {
	app, d := buildFieldPolicyApp(t, listGateACL)
	seedListGateTickets(app)

	for _, tc := range hiddenFieldQueries {
		t.Run(tc.name, func(t *testing.T) {
			_, rec := listEntitiesAs(principalCtx("audrey"), t, app, d, "ticket", "tickets", tc.query)
			assertUnreadablePropertyRejected(t, rec, "status")
		})
	}
}

// TestListFieldGate_GrantedPropertyServed is the control: a role that grants
// the property on every row filters and sorts on it, although another role in
// the same policy hides it.
func TestListFieldGate_GrantedPropertyServed(t *testing.T) {
	app, d := buildFieldPolicyApp(t, listGateACL)
	seedListGateTickets(app)
	ctx := principalCtx("harry")

	resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", "filter[status][contains]=brav")
	if rec.Code != http.StatusOK {
		t.Fatalf("hr filter on status: %d %s", rec.Code, rec.Body)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "TKT-001" {
		t.Errorf("hr filter on status: got %v, want [TKT-001]", responseIDs(resp))
	}

	resp, rec = listEntitiesAs(ctx, t, app, d, "ticket", "tickets", "sort=status")
	if rec.Code != http.StatusOK {
		t.Fatalf("hr sort on status: %d %s", rec.Code, rec.Body)
	}
	if got := strings.Join(responseIDs(resp), ","); got != "TKT-002,TKT-001" {
		t.Errorf("hr sort on status: got %s, want TKT-002,TKT-001", got)
	}
}

// TestListFieldGate_ReadablePropertiesServed pins what the gate leaves alone:
// a visible property, the id and modified pseudo-properties, and a property
// the metamodel does not declare, which redaction never hides.
func TestListFieldGate_ReadablePropertiesServed(t *testing.T) {
	app, d := buildFieldPolicyApp(t, listGateACL)
	seedListGateTickets(app)

	for _, q := range []string{
		"filter[title]=Alpha",
		"filter[title][contains]=a&sort=-title",
		"sort=id",
		"sort=-modified",
		"filter[undeclared]=x",
	} {
		t.Run(q, func(t *testing.T) {
			_, rec := listEntitiesAs(aliceCtx(), t, app, d, "ticket", "tickets", q)
			if rec.Code != http.StatusOK {
				t.Errorf("GET ?%s: %d %s", q, rec.Code, rec.Body)
			}
		})
	}
}

// TestListFieldGate_SharedPipelineGated pins that the gate sits below the
// list handler: export, _position and the other callers of
// scopedSortedEntities refuse a hidden property too.
func TestListFieldGate_SharedPipelineGated(t *testing.T) {
	app, d := buildFieldPolicyApp(t, listGateACL)
	seedListGateTickets(app)
	ctx := gateCtxFor(aliceCtx(), t, d)

	for _, tc := range hiddenFieldQueries {
		t.Run(tc.name, func(t *testing.T) {
			query, err := url.ParseQuery(tc.query)
			if err != nil {
				t.Fatal(err)
			}
			_, err = app.scopedSortedEntities(ctx, "ticket", query)
			if !errors.Is(err, errPropertyNotVisible) || !strings.Contains(err.Error(), `"status"`) {
				t.Errorf("scopedSortedEntities(?%s): err = %v, want errPropertyNotVisible naming status", tc.query, err)
			}
		})
	}
}

// assertUnreadablePropertyRejected checks for the 400 problem body that names
// property and leaks none of the seeded values.
func assertUnreadablePropertyRejected(t *testing.T, rec interface {
	Result() *http.Response
}, property string,
) {
	t.Helper()
	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
	var problem v1.Error
	if err := json.NewDecoder(res.Body).Decode(&problem); err != nil {
		t.Fatalf("decode problem body: %v", err)
	}
	if !strings.HasSuffix(problem.Type, "/property_not_visible") {
		t.Errorf("problem type = %q, want .../property_not_visible", problem.Type)
	}
	if !strings.Contains(problem.Detail, `"`+property+`"`) {
		t.Errorf("problem detail %q does not name %q", problem.Detail, property)
	}
	for _, secret := range []string{"bravo-secret", "alpha-secret", "TKT-00"} {
		if strings.Contains(problem.Detail, secret) || strings.Contains(problem.Title, secret) {
			t.Errorf("problem body names %q", secret)
		}
	}
}

// TestListFieldGate_FreeTextSkipsHiddenMatch pins RR-H3GP12: the list's `?q=`
// keeps a row only for a match on a field the principal can see. TKT-001's
// only "bravo" is in its status, which the viewer cannot see, so a kept row
// would confirm the guess.
func TestListFieldGate_FreeTextSkipsHiddenMatch(t *testing.T) {
	app, d := buildFieldPolicyApp(t, listGateACL)
	seedListGateTickets(app)

	for _, tc := range []struct{ name, user, want string }{
		{"hidden match dropped", "alice", ""},
		{"granted match kept", "harry", "TKT-001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, rec := listEntitiesAs(principalCtx(tc.user), t, app, d, "ticket", "tickets", "q=bravo")
			if rec.Code != http.StatusOK {
				t.Fatalf("GET ?q=bravo: %d %s", rec.Code, rec.Body)
			}
			if got := strings.Join(responseIDs(resp), ","); got != tc.want {
				t.Errorf("GET ?q=bravo = [%s], want [%s]", got, tc.want)
			}
		})
	}
}

// testGatedQuery builds a gatedListQuery for a test that exercises the
// pipeline below the gate. Test files are outside the guard below.
func testGatedQuery(params map[string][]string, isRelationKey func(string) bool) gatedListQuery {
	return gatedListQuery{params: params, isRelationKey: isRelationKey}
}

// TestGatedListQuery_BuiltOnlyByTheGate pins RR-T7SY1M: the filter, sort and
// pushdown take a gatedListQuery, and only listfieldgate.go may build one.
// A literal anywhere else would hand them a query the gate never checked.
func TestGatedListQuery_BuiltOnlyByTheGate(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") || name == "listfieldgate.go" {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if id, ok := lit.Type.(*ast.Ident); ok && id.Name == "gatedListQuery" {
				t.Errorf("%s: gatedListQuery literal outside listfieldgate.go; build it with gateListQuery",
					fset.Position(lit.Pos()))
			}
			return true
		})
	}
}

// TestListFieldGate_SharedKeyParser pins RR-C67ULZ: the gate sees the same
// property the filter evaluates for every key shape, including the ones the
// filter refuses.
func TestListFieldGate_SharedKeyParser(t *testing.T) {
	for _, tc := range []struct {
		key, property, operator string
		arrayForm, wantErr      bool
	}{
		{key: "filter[status]", property: "status", operator: "eq"},
		{key: "filter[status][contains]", property: "status", operator: "contains"},
		{key: "filter[status][in][]", property: "status", operator: "in", arrayForm: true},
		{key: "filter[status][]", property: "status", operator: "eq", arrayForm: true},
		{key: "filter[]", wantErr: true},
		{key: "filter[status][][x]", wantErr: true},
		{key: "filter[status][a][b]", wantErr: true},
	} {
		t.Run(tc.key, func(t *testing.T) {
			fk, err := parseFilterKey(tc.key)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseFilterKey(%q) err = %v, wantErr %v", tc.key, err, tc.wantErr)
			}
			if tc.wantErr {
				if !errors.Is(err, errBadFilter) {
					t.Errorf("err = %v, want errBadFilter", err)
				}
				return
			}
			want := filterKey{property: tc.property, operator: tc.operator, arrayForm: tc.arrayForm}
			if fk != want {
				t.Errorf("parseFilterKey(%q) = %+v, want %+v", tc.key, fk, want)
			}
		})
	}

	// A key the gate skips as a relation is skipped by the filter too: both
	// read the classifier the gate carries.
	gated, err := gateListQuery(context.Background(), hideEverything{}, func(k string) bool { return k == "implements" },
		"ticket", url.Values{"filter[implements]": {"F-1"}})
	if err != nil {
		t.Fatalf("relation key gated as a property: %v", err)
	}
	if gated.isRelationKey == nil || !gated.isRelationKey("implements") {
		t.Error("gated query lost the gate's relation classifier")
	}
}

// hideEverything is a fieldVisibility that refuses every property.
type hideEverything struct{}

func (hideEverything) FieldVisibleOnEveryRow(context.Context, string, string) bool { return false }

// TestDemoResolver_FieldVisibleOnEveryRowMatchesVerdicts pins RR-M70ORW: the
// demo fixture's list-gate answer agrees with the field verdicts it serves.
func TestDemoResolver_FieldVisibleOnEveryRowMatchesVerdicts(t *testing.T) {
	demo := DemoFieldVerdictResolver{}
	ctx := context.Background()
	for _, typ := range []string{"ticket", "feature"} {
		fv := demo.FieldVerdicts(ctx, &entity.Entity{ID: "X-1", Type: typ})
		for _, property := range []string{"priority", "kind", "status", "title"} {
			if got, want := demo.FieldVisibleOnEveryRow(ctx, typ, property), fv.IsVisible(property); got != want {
				t.Errorf("%s.%s: FieldVisibleOnEveryRow = %v, FieldVerdicts visible = %v", typ, property, got, want)
			}
		}
	}
}

// TestListFieldGate_RelationFilterComparesRedactedNeighborTitle pins the
// relation-filter finding of the BUG-WJSYQT review: a relation filter skips
// the field gate because it names a relation, and compares the neighbor's
// display title. The served headers are gated but not redacted (the list's
// visible reader carries no redactor), so before the fix alice's guess at the
// hidden feature title matched. The title must come from the redacted
// neighbor.
func TestListFieldGate_RelationFilterComparesRedactedNeighborTitle(t *testing.T) {
	const aclYAML = `
roles:
  viewer:
    read: [ticket, feature]
    visible:
      feature: []
  hr:
    read: [ticket, feature]
assignments:
  alice: viewer
  harry: hr
`
	app, d := buildFieldPolicyApp(t, aclYAML)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "Alpha"}})
	seedEntity(app, &entity.Entity{ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "Hidden Feature"}})
	seedRelation(app, entity.NewRelation("TKT-001", "implements", "FEAT-1"))
	rows := []*entity.Entity{{ID: "TKT-001", Type: "ticket"}}

	for _, tc := range []struct {
		name, user string
		match      bool
	}{
		{"hidden display title never matches", "alice", false},
		{"visible display title matches", "harry", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := gateCtxFor(principalCtx(tc.user), t, d)
			matched, err := matchRelationFilterMany(ctx, app.Services(), app.visibleReader, appRedactor(app), rows,
				"implements", dataentryconfig.DirectionOutgoing, "Hidden Feature")
			if err != nil {
				t.Fatal(err)
			}
			if matched["TKT-001"] != tc.match {
				t.Errorf("filter[implements]=Hidden Feature as %s: matched = %v, want %v",
					tc.user, matched["TKT-001"], tc.match)
			}
		})
	}
}

// TestListVisibilityConflicts pins the load-time lint for every configured
// filter or sort on a property that some reader can be refused: a role that
// reads the type and hides it with `visible:`, or a client baseline.
func TestListVisibilityConflicts(t *testing.T) {
	app := newTestAppV1(t)
	ticketRead := []string{"ticket"}
	policy := &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"viewer": {Read: ticketRead, Visible: map[string][]acl.FieldGrant{"ticket": {{Field: "title"}}}},
			"auditor": {Read: ticketRead, Visible: map[string][]acl.FieldGrant{
				"ticket": {{Field: "title"}, {Field: "status", When: "true"}}}},
			"hr": {Read: ticketRead, Visible: map[string][]acl.FieldGrant{
				"ticket": {{Field: "title"}, {Field: "status"}}}},
			// Hides status but cannot read tickets, so it shows no rows of
			// the type and hides nothing.
			"bystander": {Visible: map[string][]acl.FieldGrant{"ticket": {{Field: "title"}}}},
			"admin":     {Read: ticketRead},
		},
	}
	statusControl := []dataentryconfig.FilterControl{{Property: "status"}, {Relation: "implements"}}
	statusSort := []dataentryconfig.SortSpec{{Property: "status"}}
	cfg := &Config{
		Lists: map[string]dataentryconfig.List{
			"by-status": {EntityType: "ticket", Sort: statusSort},
			"grouped":   {EntityType: "ticket", GroupBy: &dataentryconfig.ListGroupBy{Property: "status"}},
			"filtered": {EntityType: "ticket",
				Filters: []dataentryconfig.FilterConfig{{Property: "status", Operator: "==", Value: "x"}}},
			"controlled": {EntityType: "ticket", FilterControls: statusControl},
			"by-title": {EntityType: "ticket",
				Sort: []dataentryconfig.SortSpec{{Property: "title"}, {Property: "modified"}}},
			"undeclared": {EntityType: "ticket", Sort: []dataentryconfig.SortSpec{{Property: "nickname"}}},
		},
		Kanbans: map[string]dataentryconfig.Kanban{
			"board": {EntityType: "ticket", FilterControls: statusControl},
		},
		Calendars: map[string]dataentryconfig.Calendar{
			"dated":    {Sources: []dataentryconfig.CalendarSource{{EntityType: "ticket", Date: "status"}}},
			"narrowed": {Sources: []dataentryconfig.CalendarSource{{EntityType: "ticket", Date: "title", Where: []string{"status=open"}}}},
		},
		Navigation: []dataentryconfig.NavigationEntry{{Entities: "ticket", Sort: statusSort}},
	}

	got := listVisibilityConflicts(cfg, app.Meta(), policy)
	surfaces := []string{
		`calendar "dated"`, `calendar "narrowed"`, `kanban "board"`, `list "by-status"`, `list "controlled"`,
		`list "filtered"`, `list "grouped"`, `sidebar entities "ticket"`,
	}
	if len(got) != len(surfaces) {
		t.Fatalf("got %d warnings, want %d (%v): %v", len(got), len(surfaces), surfaces, got)
	}
	for i, surface := range surfaces {
		if !strings.HasPrefix(got[i], surface+" ") || !strings.Contains(got[i], `"status"`) ||
			!strings.Contains(got[i], "role(s) auditor, viewer can") {

			t.Errorf("warning %d = %q, want %s, property status, roles auditor and viewer", i, got[i], surface)
		}
	}

	t.Run("client baseline", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			baseline acl.ClientBaseline
			want     bool
		}{
			{"visible leaves it out", acl.ClientBaseline{Restriction: acl.Restriction{
				Visible: map[string][]string{"ticket": {"title"}}}}, true},
			{"redact names it", acl.ClientBaseline{Restriction: acl.Restriction{
				Redact: map[string][]string{"ticket": {"status"}}}}, true},
			{"visible keeps it", acl.ClientBaseline{Restriction: acl.Restriction{
				Visible: map[string][]string{"ticket": {"title", "status"}}}}, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				clientPolicy := &acl.Policy{
					Roles:           map[string]acl.RoleDef{"admin": {Read: ticketRead}},
					ClientBaselines: map[string]acl.ClientBaseline{"bots": tc.baseline},
				}
				warnings := listVisibilityConflicts(&Config{Lists: map[string]dataentryconfig.List{
					"by-status": {EntityType: "ticket", Sort: statusSort},
				}}, app.Meta(), clientPolicy)
				if tc.want != (len(warnings) == 1 && strings.Contains(warnings[0], "client baseline(s) bots")) {
					t.Errorf("warnings = %v, want a bots warning: %v", warnings, tc.want)
				}
			})
		}
	})

	if listVisibilityConflicts(cfg, app.Meta(), nil) != nil {
		t.Error("nil policy: want no warnings")
	}
}
