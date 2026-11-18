package dataentry

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// TestViews_FollowTheWorld: a dashboard card and the entity list that feeds
// kanban boards and calendars serve the face the request's world picks
// (TKT-6KLUT3U). facedGanttApp seeds PRJ-A at a draft and a published face;
// the gantt has its own test, TestGantt_FollowsTheWorld.
func TestViews_FollowTheWorld(t *testing.T) {
	app := facedGanttApp(t)
	app.setWorlds(ganttFaceWorlds{})
	installProjectDashboard(app)

	for _, world := range []string{"draft", "published"} {
		t.Run(world, func(t *testing.T) {
			want := "Root " + world
			list := routedBody(t, app, "/api/v1/projects?"+url.Values{"world": {world}}.Encode())
			if got := fmt.Sprint(list["data"]); !strings.Contains(got, want) || strings.Contains(got, otherRoot(world)) {
				t.Errorf("list in %s = %s, want only %q", world, got, want)
			}

			q := url.Values{"name": {"projects"}, "card": {"0"}, "world": {world}}
			card := routedBody(t, app, "/api/v1/_dashboard/card?"+q.Encode())
			if got := fmt.Sprint(card["rows"]); !strings.Contains(got, want) || strings.Contains(got, otherRoot(world)) {
				t.Errorf("card rows in %s = %s, want only %q", world, got, want)
			}
		})
	}
}

func otherRoot(world string) string {
	if world == "draft" {
		return "Root published"
	}
	return "Root draft"
}

// installProjectDashboard publishes a `projects` dashboard with one table
// card over every project.
func installProjectDashboard(app *App) {
	cur := app.State()
	next := *cur
	cfg := *cur.Cfg
	cfg.Lists = map[string]dataentryconfig.List{"projects": {EntityType: "project"}}
	cfg.Dashboards = map[string]dataentryconfig.DashboardConfig{"projects": {
		Title: "Projects", Cards: []dataentryconfig.DashboardCard{
			{Title: "Projects", List: "projects", Display: "table",
				Columns: []dataentryconfig.ListColumn{{Property: "title"}}},
		},
	}}
	next.Cfg = &cfg
	app.schema.Publish(&next)
}
