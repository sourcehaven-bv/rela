package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// navStatusPushdownEntries are the sidebar entries the comparison counts,
// over the fixture of [newScopePushdownAppOn] and the lists of
// [conditionLists]. goOnly names the entries whose rules never lower; reads
// names those reading status, which frank may not see on every row.
var navStatusPushdownEntries = []struct {
	entry         dataentryconfig.NavigationEntry
	goOnly, reads bool
}{
	{entry: dataentryconfig.NavigationEntry{Label: "Order", List: "all", Status: []dataentryconfig.NavStatusRule{
		// Counts 0 for everyone, so the next rule decides; the second is
		// denied for dave, who reads no users, and so counts 0 for him.
		rule("error", "{count} nobody", "related(entity, 'implements', { title = 'Nobody' })"),
		rule("warning", "{count} reported", "related(entity, 'reportedBy')"),
		rule("info", "{count} unreported", "not related(entity, 'reportedBy')"),
	}}},
	{entry: dataentryconfig.NavigationEntry{Label: "Mine", List: "unreported", Status: []dataentryconfig.NavStatusRule{
		rule("error", "{count} mine", "is_current_user(entity.assignee)"),
		rule("warning", "{count} open impl", "related(entity, 'implements', { status = 'open' })"),
	}}},
	{entry: dataentryconfig.NavigationEntry{Label: "Watched", List: "watching", Status: []dataentryconfig.NavStatusRule{
		rule("info", "{count}", ""),
	}}},
	{entry: dataentryconfig.NavigationEntry{Label: "Reported", List: "reported", Status: []dataentryconfig.NavStatusRule{
		// The list's own traversal is denied for dave: no rows, no marker.
		rule("info", "{count}", "not related(entity, 'implements')"),
	}}},
	{entry: dataentryconfig.NavigationEntry{Label: "Status", List: "all", Status: []dataentryconfig.NavStatusRule{
		rule("error", "{count} blocked", "entity.status == 'blocked'"),
		rule("warning", "{count} not done", "entity.status ~= 'done'"),
	}}, reads: true},
	{entry: dataentryconfig.NavigationEntry{Label: "Either", List: "all", Status: []dataentryconfig.NavStatusRule{
		rule("info", "{count}", "entity.status == 'open' or related(entity, 'reportedBy')"),
	}}, goOnly: true},
}

// Every nav status entry that lowers is counted in the store, and gives the
// marker the Go path gives over the same principal's rows (TKT-I39Y7N4).
func TestNavStatus_InStoreMatchesGoPath(t *testing.T) {
	app, _, _ := newScopePushdownAppOn(t, memstore.New())
	d := withConditionLists(t, app)
	nav := make([]dataentryconfig.NavigationEntry, 0, len(navStatusPushdownEntries))
	for _, e := range navStatusPushdownEntries {
		nav = append(nav, e.entry)
	}
	installNavStatusConfig(app, map[string]dataentryconfig.List{"all": {EntityType: "ticket"}}, nav)
	wireRealViewConditions(t, app)

	for _, user := range []string{"alice", "bob", "dave", "erin", "frank"} {
		ctx := gateCtxFor(principalCtx(user), t, d)
		s := app.State()
		entries := sidebarNavStatusEntries(ctx, app.views.currentACL(), s.Cfg, "")
		if len(entries) != len(navStatusPushdownEntries) {
			t.Fatalf("%s: %d nav status entries, want %d", user, len(entries), len(navStatusPushdownEntries))
		}
		for i, ns := range entries {
			want := navStatusPushdownEntries[i]
			t.Run(user+" "+ns.Entry.Label, func(t *testing.T) {
				listCfg := s.Cfg.Lists[ns.List]
				got, gotOK, handled, gotErr := navStatusInStore(ctx, app, s, ns, listCfg)
				wantStatus, wantOK, wantErr := navStatusInGo(ctx, app, s, ns, listCfg)
				if wantErr != nil {
					t.Fatalf("go path: %v", wantErr)
				}
				pushed := !want.goOnly && (user != "frank" || !want.reads)
				if handled != pushed {
					t.Fatalf("handled = %v, want %v", handled, pushed)
				}
				if !handled {
					return
				}
				if gotErr != nil {
					t.Fatalf("store path: %v", gotErr)
				}
				if gotOK != wantOK || got != wantStatus {
					t.Errorf("store %v %+v, go %v %+v", gotOK, got, wantOK, wantStatus)
				}
			})
		}
	}
}
