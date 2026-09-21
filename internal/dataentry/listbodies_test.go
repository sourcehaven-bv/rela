package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// Body budgets (BUG-SDMD6O). A collection read filters, sorts and counts over
// a WHOLE type before it knows which rows it will serve, and the rows it does
// serve render properties, never bodies. So the pipeline must be handed no
// body at all unless the request asked for one — and then exactly as many as
// the page carries, never as many as the type holds.
//
// # Why this is a body COUNT and not a heap measurement
//
// The defect was reported as +101 MB vs +1 MB on pgstore, so the instrument
// the ticket reaches for is a DB-gated heap assertion. That instrument cannot
// do this job, and the bug's own history says why: the identical probe on
// memstore reports ~1 MB whether or not the defect is present, because
// memstore's Clone shares the body STRING rather than copying its bytes. A
// heap test is therefore a test that cannot fail on the default backend, and
// it runs only where RELA_TEST_DATABASE_URL is set — which is one CI job and
// almost no developer machine. The regression would have been reintroduced
// and merged exactly as it was the first time.
//
// Counting bodies asks the question the defect is actually about — "did this
// pipeline read markdown it will not render?" — and that question is
// backend-independent, exact, and has no threshold to tune. It fails
// identically on memstore, fsstore and pgstore. The bytes those bodies cost
// is what varies by backend; whether they were read at all does not.
//
// See [storetest.BodyWatch] for the instrument and the fuller argument.

// newBodyWatchApp builds a list-serving app over a BodyWatch store holding n
// tickets, every one with a non-empty body — so any body the pipeline touches
// is counted rather than silently being an empty string.
func newBodyWatchApp(t *testing.T, n int) (*App, *storetest.BodyWatch, *acl.Declarative, context.Context) {
	t.Helper()
	watch := storetest.NewBodyWatch(memstore.New())
	ctx := context.Background()
	mk := func(id, typ, title, body string) {
		e := entity.New(id, typ)
		e.SetString("title", title)
		e.Content = body
		if err := watch.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	mk("T1", "team", "Team one", "team body")
	mk("P1", "person", "Person one", "person body")
	if _, err := watch.CreateRelation(ctx, "P1", "member-of", "T1", nil); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 5; i++ {
		mk(fmt.Sprintf("F%d", i), "feature", fmt.Sprintf("Feature %d", i), "feature body")
	}
	for i := 1; i <= n; i++ {
		id := fmt.Sprintf("TKT-%04d", i)
		// A distinctive, non-empty body per row. Length is irrelevant to the
		// assertion (bodies are COUNTED, not weighed), so these stay short and
		// the fixture stays fast.
		mk(id, "ticket", "Ticket "+id, "body of "+id)
		if _, err := watch.CreateRelation(ctx, id, "implements", fmt.Sprintf("F%d", i%5+1), nil); err != nil {
			t.Fatal(err)
		}
		if _, err := watch.CreateRelation(ctx, id, "assigned-to", "P1", nil); err != nil {
			t.Fatal(err)
		}
	}
	app := newAppFromParts(budgetConfig(), budgetMeta(), newFixture(), appbuildtest.WithStore(watch))
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"editor": {Read: []string{"*"}}},
		Assignments: map[string]string{"T1": "editor"},
	}, app.store)
	app.acl = d
	watch.Reset()
	return app, watch, d, principalCtx("P1")
}

// The core invariant: a default list request reads NO body, whatever the type
// holds. Zero is the honest bound — not "small", not "proportional to the
// page" — because the served rows render properties and the paged-out rows
// are not rendered at all.
//
// Run at two sizes so a body count that grows with the type names itself as
// growth rather than as a moved constant. On develop before TKT-1U8XYN this
// reported 10 and 50.
func TestListBodies_DefaultListReadsNoBodies(t *testing.T) {
	for _, n := range []int{10, 50} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			app, watch, d, ctx := newBodyWatchApp(t, n)
			resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", "per_page=5")
			if rec.Code != http.StatusOK {
				t.Fatalf("list: %d %s", rec.Code, rec.Body)
			}
			if len(resp.Data) != 5 {
				t.Fatalf("rows = %d, want the requested page of 5", len(resp.Data))
			}
			if got := watch.Bodies(); got != 0 {
				t.Errorf("list of %d rows read %d bodies, want 0 (%s)", n, got, watch)
			}
		})
	}
}

// Sorting, filtering and free-text intersection all run across the whole SET,
// which is what made "the full load is required" look true. It is not: every
// one of them reads properties. Each query shape therefore keeps the zero-body
// bound.
//
// A `?q=` arm is included deliberately, and it is the one shape that does NOT
// assert zero — see [TestListBodies_FreeTextBodiesScaleWithHitsNotType] for
// what it asserts instead and why.
func TestListBodies_QueryShapesKeepTheBound(t *testing.T) {
	for _, tc := range []struct{ name, query string }{
		{name: "sorted", query: "per_page=5&sort=title"},
		{name: "filtered", query: "per_page=5&filter[title]=Ticket+TKT-0001"},
		{name: "paged deep", query: "per_page=5&page=2"},
		// A free-text query narrow enough to match ONE row. The list
		// pipeline's own contribution is still zero; the single body is the
		// search gate's, and pinning it here keeps that cost attributed
		// rather than blamed on the list.
		{name: "free text narrow", query: "per_page=5&q=TKT-0001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, n := range []int{10, 50} {
				app, watch, d, ctx := newBodyWatchApp(t, n)
				_, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", tc.query)
				if rec.Code != http.StatusOK {
					t.Fatalf("list n=%d: %d %s", n, rec.Code, rec.Body)
				}
				// One tolerated body on the free-text arm, zero everywhere
				// else. Expressed as a bound that does not move with n, which
				// is the property under test.
				want := 0
				if strings.Contains(tc.query, "q=") {
					want = 1
				}
				if got := watch.Bodies(); got != want {
					t.Errorf("n=%d: read %d bodies, want %d (%s)", n, got, want, watch)
				}
			}
		})
	}
}

// Free-text list bodies are the search layer's, not the list pipeline's, and
// they scale with HITS rather than with the type.
//
// The distinction is worth pinning because it is easy to misread. A `?q=` that
// matches every row of the type reads a body per row, which looks exactly like
// the defect BUG-SDMD6O describes — a body per entity of the type — and is
// not. The reads come from internal/search/visible.go, which loads each HIT to
// decide whether the match landed on a field the principal may see; dropping
// them would turn a field-visibility gate into an oracle. They are bounded by
// maxFreeTextSearchResults, and a narrower query reads fewer.
//
// So the honest assertion is about what the count tracks, not about its size:
// hold the type size fixed and vary the MATCH count, and the bodies follow the
// matches. A regression that made the list pipeline itself load bodies on the
// `?q=` path would break this, because the count would then follow n.
func TestListBodies_FreeTextBodiesScaleWithHitsNotType(t *testing.T) {
	const n = 50
	// One query matches every ticket, the other exactly one — over the SAME
	// fixture, so the type size is held constant and only the hits vary.
	app, watch, d, ctx := newBodyWatchApp(t, n)
	if _, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", "per_page=5&q=Ticket"); rec.Code != http.StatusOK {
		t.Fatalf("broad: %d %s", rec.Code, rec.Body)
	}
	broad := watch.Bodies()

	app, watch, d, ctx = newBodyWatchApp(t, n)
	if _, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets", "per_page=5&q=TKT-0001"); rec.Code != http.StatusOK {
		t.Fatalf("narrow: %d %s", rec.Code, rec.Body)
	}
	narrow := watch.Bodies()

	if narrow != 1 {
		t.Errorf("a single-hit query read %d bodies, want 1 — the search gate reads one entity per hit", narrow)
	}
	if broad <= narrow {
		t.Errorf("bodies did not follow the hit count: broad=%d narrow=%d over the same %d-row type",
			broad, narrow, n)
	}
}

// include_content=true is the ONE list shape that legitimately wants bodies,
// and it must pay for the PAGE, not for the type. This is the other half of
// the invariant: a fix that simply stopped loading bodies everywhere would
// pass the zero-body tests above and silently serve empty content here.
func TestListBodies_IncludeContentPaysForThePageOnly(t *testing.T) {
	const perPage = 5
	for _, n := range []int{10, 50} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			app, watch, d, ctx := newBodyWatchApp(t, n)
			resp, rec := listEntitiesAs(ctx, t, app, d, "ticket", "tickets",
				fmt.Sprintf("per_page=%d&include_content=true", perPage))
			if rec.Code != http.StatusOK {
				t.Fatalf("list: %d %s", rec.Code, rec.Body)
			}
			if got := watch.Bodies(); got != perPage {
				t.Errorf("include_content read %d bodies for a %d-row page of %d rows, want %d (%s)",
					got, perPage, n, perPage, watch)
			}
			// The bodies must actually REACH the wire: counting reads alone
			// would pass if they were loaded and then dropped.
			for _, row := range resp.Data {
				if !strings.HasPrefix(row.Content, "body of ") {
					t.Errorf("row %s served content %q, want its body", row.ID, row.Content)
				}
			}
		})
	}
}
