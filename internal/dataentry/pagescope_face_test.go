package dataentry

import (
	"maps"
	"net/url"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// An incoming page scope reaches a row only over an edge whose tail face the
// reader may read (#2366). POL-1@draft cites FEAT-DRAFT and POL-1@published
// cites FEAT-PUB (contentEdgeApp). A reader granted only policy@published
// sees POL-1 as a row, but must not learn through FEAT-DRAFT's tab or card
// that the hidden draft face cites it.
func TestPageScope_IncomingReachIsGatedOnTheTailFace(t *testing.T) {
	citedBy := &dataentryconfig.PageTabScope{Relation: "cites", Direction: dataentryconfig.DirectionIncoming}
	for _, reader := range contentEdgeReaders {
		t.Run(reader.name, func(t *testing.T) {
			app, _ := contentEdgeApp(t, reader)
			installPagesConfig(app, map[string]dataentryconfig.List{"policies": {EntityType: "policy"}},
				map[string]dataentryconfig.Page{"feat": {
					PageHeader: dataentryconfig.PageHeader{Label: "Feature", EntityType: "feature"},
					Tabs: []dataentryconfig.PageTab{
						{ID: "policies", Label: "Policies", List: "policies", Scope: citedBy},
					},
				}}, nil)
			installCitedByDashboard(app, citedBy)

			for _, tc := range []struct {
				anchor string
				want   bool
			}{
				{"FEAT-DRAFT", reader.draft},
				{"FEAT-PUB", true},
			} {
				q := url.Values{"scope_page": {"feat"}, "scope_tab": {"policies"}, "anchor": {tc.anchor}}
				list := routedBody(t, app, "/api/v1/policys?"+q.Encode())
				if got := mentions(t, list["data"], "POL-1"); got != tc.want {
					t.Errorf("%s tab lists POL-1 = %v, want %v: %v", tc.anchor, got, tc.want, list["data"])
				}

				wantTotal := 0
				if tc.want {
					wantTotal = 1
				}
				for card, display := range []string{"count", "table"} {
					q := url.Values{"name": {"feat"}, "card": {[]string{"0", "1"}[card]}, "anchor": {tc.anchor}}
					value := routedBody(t, app, "/api/v1/_dashboard/card?"+q.Encode())
					if got, _ := value["total"].(float64); int(got) != wantTotal {
						t.Errorf("%s %s card total = %v, want %d", tc.anchor, display, value["total"], wantTotal)
					}
					if got := mentions(t, value["rows"], "POL-1"); got != (tc.want && display == "table") {
						t.Errorf("%s %s card rows name POL-1 = %v: %v", tc.anchor, display, got, value["rows"])
					}
				}
			}
		})
	}
}

// installCitedByDashboard publishes a `feat` entity dashboard over features
// with a count card and a table card of the policies citing the anchor.
func installCitedByDashboard(app *App, scope *dataentryconfig.PageTabScope) {
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Lists = maps.Clone(cur.Cfg.Lists)
	cfg.Dashboards = map[string]dataentryconfig.DashboardConfig{"feat": {
		Title: "Feature", EntityType: "feature", Cards: []dataentryconfig.DashboardCard{
			{Title: "Policies", List: "policies", Display: "count", Scope: scope},
			{Title: "Rows", List: "policies", Display: "table",
				Columns: []dataentryconfig.ListColumn{{Property: "title"}}, Scope: scope},
		},
	}}
	next.Cfg = &cfg
	app.schema.Publish(&next)
}
