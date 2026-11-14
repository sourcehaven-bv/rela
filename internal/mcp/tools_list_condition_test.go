package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// listConditionSchema is the fixture of the list_entities condition tests
// (TKT-Z099FGY): tickets that implement features and are reported by users.
const listConditionSchema = `version: "1.0"
namespace: https://example.org/test#
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      title: {type: string, required: true}
      status: {type: string}
      assignee: {type: string}
      watchers: {type: string, list: true}
  feature:
    label: Feature
    id_prefix: FEAT
    properties:
      title: {type: string, required: true}
      status: {type: string}
  user:
    label: User
    id_prefix: USR
    properties:
      title: {type: string}
relations:
  implements: {label: implements, from: [ticket], to: [feature], inverse: implementedBy}
  editor-of: {label: editor of, from: [user], to: [ticket]}
  reports: {label: reports, from: [user], to: [ticket], inverse: reportedBy}
`

// listConditionFilters are the filters the comparison serves.
var listConditionFilters = map[string]string{
	"open":          "entity.status == 'open'",
	"not-done":      "entity.status ~= 'done' and entity.status ~= 'blocked'",
	"impl-open":     "related(entity, 'implements', { status = 'open' })",
	"unreported":    "not related(entity, 'reportedBy')",
	"reported":      "related(entity, 'reportedBy')",
	"contradiction": "related(entity, 'reportedBy') and not related(entity, 'reportedBy')",
	"mine":          "is_current_user(entity.assignee)",
	"watching":      "has_current_user(entity.watchers)",
	"mine-reported": "related(entity, 'reportedBy', { id = current_user.id })",
	"others-open":   "entity.assignee ~= current_user.id and entity.status == 'open'",
	// These do not lower: they keep the Go path for everyone.
	"open-or-reported": "entity.status == 'open' or related(entity, 'reportedBy')",
	"title-from-b":     "entity.title >= 'b'",
	"mine-from-b":      "is_current_user(entity.assignee) and entity.title >= 'b'",
}

// goOnlyFilters are the filters that never lower.
var goOnlyFilters = map[string]bool{"open-or-reported": true, "title-from-b": true, "mine-from-b": true}

// statusFilters read status, which frank does not see on any ticket; they
// keep the Go path for him.
var statusFilters = map[string]bool{"open": true, "not-done": true, "others-open": true}

// reportedFilters require a traversal into users, which dave may not read:
// the store answers them with an empty page and no read.
var reportedFilters = map[string]bool{"reported": true, "contradiction": true, "mine-reported": true}

// conditionFixture is a gated list_entities server over a counting store.
type conditionFixture struct {
	srv      *Server
	counting *storetest.Counting
	as       func(user string) context.Context
}

// newConditionFixtureOn seeds n tickets, three features and three users on
// base and serves them under a policy with one principal per access shape:
// alice reads everything, bob reads tickets only as their editor, dave
// reads no users, erin reads no tickets, frank sees no ticket status.
func newConditionFixtureOn(t *testing.T, base store.Store, n int) conditionFixture {
	t.Helper()
	ctx := context.Background()
	meta, err := metamodel.Parse([]byte(listConditionSchema))
	if err != nil {
		t.Fatal(err)
	}
	put := func(e *entity.Entity) {
		if cerr := base.CreateEntity(ctx, e); cerr != nil {
			t.Fatal(cerr)
		}
	}
	link := func(from, typ, to string) {
		if _, cerr := base.CreateRelation(ctx, entity.RelationKey{From: from, Type: typ, To: to}, nil); cerr != nil {
			t.Fatal(cerr)
		}
	}
	for i := 1; i <= 3; i++ {
		put(&entity.Entity{ID: fmt.Sprintf("FEAT-%d", i), Type: "feature", Properties: map[string]any{
			"title": fmt.Sprintf("Feature %d", i), "status": []string{"open", "done", "open"}[i-1],
		}})
	}
	for _, u := range []string{"alice", "bob", "dave"} {
		put(&entity.Entity{ID: u, Type: "user", Properties: map[string]any{"title": u}})
	}
	titles := []string{"b", "a", "B", "c", "a", "b"}
	for i := 1; i <= n; i++ {
		id := fmt.Sprintf("TKT-%04d", i)
		props := map[string]any{
			"title":  titles[i%len(titles)],
			"status": []string{"open", "done", "open", "blocked"}[i%4],
		}
		// Off-type and unset values: Go binds a list or a number under a
		// string declaration to nil, so `~=` keeps them and `==` drops them.
		switch i % 12 {
		case 10:
			delete(props, "status")
		case 11:
			props["status"] = []string{"done"}
		case 0:
			props["status"] = 5
		}
		switch {
		case i%12 == 6:
			props["assignee"] = ""
		case i%3 != 0:
			props["assignee"] = []string{"alice", "bob"}[i%2]
		}
		if i%5 != 0 {
			props["watchers"] = [][]string{{"alice"}, {"bob", "alice"}, {}, {"dave"}}[i%4]
		}
		put(&entity.Entity{ID: id, Type: "ticket", Properties: props})
		if i%4 != 0 {
			link(id, "implements", fmt.Sprintf("FEAT-%d", i%3+1))
		}
		if i%2 == 0 {
			link([]string{"alice", "dave"}[i%4/2], "reports", id)
		}
		if i%12 <= 7 {
			link("bob", "editor-of", id)
		}
	}

	counting := storetest.NewCounting(base)
	d, err := acl.NewDeclarative(&acl.Policy{
		Roles: map[string]acl.RoleDef{
			"all":      {Read: []string{"ticket", "feature", "user"}},
			"browse":   {Read: []string{"feature", "user"}},
			"tickets":  {Read: []string{"ticket", "feature"}},
			"features": {Read: []string{"feature"}},
			"editor":   {Read: []string{"ticket"}},
			"nostatus": {
				Read: []string{"ticket", "feature", "user"},
				Visible: map[string][]acl.FieldGrant{"ticket": {
					{Field: "title"}, {Field: "assignee"}, {Field: "watchers"},
				}},
			},
		},
		RoleRelations: map[string]acl.RoleRelationDef{"editor-of": {Confers: "editor"}},
		Assignments: map[string]string{
			"alice": "all", "bob": "browse", "dave": "tickets", "erin": "features", "frank": "nostatus",
		},
	}, aclstore.MustNewGraph(counting), counting)
	if err != nil {
		t.Fatal(err)
	}
	svc := appbuildtest.New(meta, appbuildtest.WithStore(counting), appbuildtest.WithDeclarative(d))
	t.Cleanup(func() { _ = svc.Close() })
	reads := svc.GatedReads()
	srv := &Server{logger: slog.New(slog.DiscardHandler)}
	setDeps(srv, Deps{
		Store:    reads.Reader,
		Related:  RelatedReads{Traversals: reads.Traversals, Conditions: reads.Conditions},
		Meta:     meta,
		World:    store.TrivialScope(),
		Families: store.TrivialScope(),
	})
	return conditionFixture{srv: srv, counting: counting, as: func(user string) context.Context {
		return acltest.Bind(principal.With(ctx, principal.Principal{User: user, Tool: principal.ToolMCP}), t, d)
	}}
}

// list serves list_entities for ctx's caller and returns the page's ids and
// the total.
func (f conditionFixture) list(ctx context.Context, t *testing.T, args map[string]any) (ids []string, total int) {
	t.Helper()
	result, err := f.srv.handleListEntities(ctx, makeToolRequest(args))
	if err != nil {
		t.Fatal(err)
	}
	text := getResultText(t, result)
	if isErrorResult(result) {
		t.Fatalf("list_entities %v: %s", args, text)
	}
	page := decodeEntityPage(t, text)
	for _, e := range page.Entities {
		ids = append(ids, e.ID)
	}
	return ids, page.Total
}

// goPath answers filter for ctx's caller on the Go path alone: every
// readable ticket, judged by the filter on the redacted row.
func (f conditionFixture) goPath(
	ctx context.Context, t *testing.T, filter string, offset, limit int,
) (ids []string, total int) {
	t.Helper()
	d := snapshotFor(ctx, f.srv).deps
	prog, err := compileListFilter(d.Meta, d.Related, "ticket", filter)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err = d.Related.Conditions.StampQueryIdentity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	page := listPage{faces: store.InWorld(store.TrivialScope()), offset: offset, limit: limit}
	rows, total, err := page.filterInGo(ctx, d, "ticket", prog)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range rows {
		ids = append(ids, e.ID)
	}
	return ids, total
}

// The comparison tests run serially, subtests included: every subtest
// reads the fixture's one counting store.
func TestListEntities_ConditionMatchesGoPath(t *testing.T) {
	assertConditionMatchesGoPath(t, newConditionFixtureOn(t, memstore.New(), 24))
}

// assertConditionMatchesGoPath serves every filter for every principal and
// page through list_entities, and compares the rows and the total with the
// Go path (TKT-Z099FGY). It also checks which path served the page, so the
// comparison cannot pass by both sides taking the Go path.
func assertConditionMatchesGoPath(t *testing.T, f conditionFixture) {
	t.Helper()
	for _, user := range []string{"alice", "bob", "dave", "erin", "frank"} {
		for name, filter := range listConditionFilters {
			for _, pg := range []struct{ offset, limit int }{{0, 4}, {4, 4}, {0, 50}, {30, 5}} {
				t.Run(fmt.Sprintf("%s %s %d+%d", user, name, pg.offset, pg.limit), func(t *testing.T) {
					ctx := f.as(user)
					f.counting.Reset()
					gotIDs, gotTotal := f.list(ctx, t, map[string]any{
						"type": "ticket", "filter": filter,
						"offset": float64(pg.offset), "limit": float64(pg.limit),
					})
					calls := f.counting.Calls()

					wantIDs, wantTotal := f.goPath(f.as(user), t, filter, pg.offset, pg.limit)
					if !slices.Equal(gotIDs, wantIDs) || gotTotal != wantTotal {
						t.Fatalf("served %v of %d, Go path %v of %d", gotIDs, gotTotal, wantIDs, wantTotal)
					}

					// CountMatched is the pushed path's signature. The Go
					// path reads the whole readable type (ListEntities, or
					// GraphQuery under a graph read scope) and counts in
					// memory.
					switch {
					case user == "erin":
						// erin reads no tickets: neither path reaches a row.
					case user == "dave" && reportedFilters[name]:
						if calls["CountMatched"]+calls["ListEntities"]+calls["GraphQuery"]+calls["GraphQueryHeaders"] != 0 {
							t.Errorf("a denied traversal must read nothing: %s", f.counting)
						}
					case goOnlyFilters[name] || user == "frank" && statusFilters[name]:
						if calls["CountMatched"] != 0 || calls["ListEntities"]+calls["GraphQuery"] == 0 {
							t.Errorf("%s must stay on the Go path for %s: %s", name, user, f.counting)
						}
					default:
						if calls["CountMatched"] != 1 || calls["ListEntities"]+calls["GraphQuery"] != 0 {
							t.Errorf("%s was not pushed down for %s: %s", name, user, f.counting)
						}
					}
				})
			}
		}
	}
}

// A filter the store answers costs the same over 100 tickets as over
// 1,000: one count and one page, whatever the traversal, `~=` or
// current_user it uses (TKT-Z099FGY).
func TestHandleListEntities_ConditionScaleFree(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"open", "not-done", "impl-open", "unreported", "reported", "mine", "watching", "mine-reported", "others-open",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			storetest.AssertScaleFree(t, func(n int) (storetest.Cost, string) {
				f := newConditionFixtureOn(t, memstore.New(), n)
				ctx := f.as("alice")
				f.counting.Reset()
				ids, _ := f.list(ctx, t, map[string]any{
					"type": "ticket", "filter": listConditionFilters[name], "offset": float64(2), "limit": float64(3),
				})
				if len(ids) != 3 {
					t.Fatalf("%s served %v, want a full page", name, ids)
				}
				return f.counting.Cost(), f.counting.String()
			})
		})
	}
}

// A current_user filter answers for the caller. Before the query identity
// was stamped it failed with no current user.
func TestHandleListEntities_CurrentUserFilterAnswersForCaller(t *testing.T) {
	t.Parallel()
	f := newConditionFixtureOn(t, memstore.New(), 12)
	for user, want := range map[string][]string{
		"alice": {"TKT-0001", "TKT-0004", "TKT-0008", "TKT-0009", "TKT-0012"},
		"dave":  {"TKT-0003", "TKT-0007", "TKT-0011"},
	} {
		ids, total := f.list(f.as(user), t, map[string]any{
			"type": "ticket", "filter": "has_current_user(entity.watchers)", "limit": float64(50),
		})
		if !slices.Equal(ids, want) || total != len(want) {
			t.Errorf("%s: %v of %d, want %v", user, ids, total, want)
		}
	}
}

// A current_user filter with no bound identity fails the call rather than
// matching against an empty one.
func TestHandleListEntities_CurrentUserFilterNeedsPrincipal(t *testing.T) {
	t.Parallel()
	f := newConditionFixtureOn(t, memstore.New(), 4)
	result, err := f.srv.handleListEntities(context.Background(), makeToolRequest(map[string]any{
		"type": "ticket", "filter": "is_current_user(entity.assignee)",
	}))
	if err == nil && !isErrorResult(result) {
		t.Fatalf("served %s without a principal", getResultText(t, result))
	}
}
