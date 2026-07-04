package dataentry

import (
	"context"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/calfeed"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// compile-time: declarativeFeed satisfies feedProvider.
var _ feedProvider = (*declarativeFeed)(nil)

func feedTestMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Version: "1.0",
		Entities: map[string]metamodel.EntityDef{
			"task": {
				Label: "Task", IDPrefix: "TSK-", DisplayProperty: "title",
				Properties: map[string]metamodel.PropertyDef{
					"title":    {Type: metamodel.PropertyTypeString},
					"due":      {Type: metamodel.PropertyTypeDate},
					"status":   {Type: metamodel.PropertyTypeString},
					"schedule": {Type: metamodel.PropertyTypeString},
				},
			},
			"party": {
				Label: "Party", IDPrefix: "PTY-",
				Properties: map[string]metamodel.PropertyDef{
					"name": {Type: metamodel.PropertyTypeString},
					"on":   {Type: metamodel.PropertyTypeDate},
					"ends": {Type: metamodel.PropertyTypeDate},
				},
			},
		},
	}
}

// fakeSource is an in-memory entitySource keyed by type.
type fakeSource struct {
	byType map[string][]*entity.Entity
}

func (f fakeSource) listType(_ context.Context, t string) ([]*entity.Entity, error) {
	return f.byType[t], nil
}

func (f fakeSource) getEntity(_ context.Context, t, id string) (*entity.Entity, bool, error) {
	for _, e := range f.byType[t] {
		if e.ID == id {
			return e, true, nil
		}
	}
	return nil, false, nil
}

func mkTask(id, title, status, due string, mod time.Time) *entity.Entity {
	return &entity.Entity{
		ID: id, Type: "task", UpdatedAt: mod,
		Properties: map[string]any{"title": title, "status": status, "due": due},
	}
}

func testLinker(t, id string) string { return "/entity/" + t + "/" + id }

func newTestFeed(t *testing.T, cfg dataentryconfig.Feed, src entitySource) *declarativeFeed {
	t.Helper()
	d, err := newDeclarativeFeed("cal", cfg, feedTestMeta(), src, testLinker)
	if err != nil {
		t.Fatalf("newDeclarativeFeed: %v", err)
	}
	return d
}

func TestDeclarativeFeed_ListMapsAndFilters(t *testing.T) {
	mod := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	src := fakeSource{byType: map[string][]*entity.Entity{
		"task": {
			mkTask("TSK-1", "Renew passport", "todo", "2026-07-10", mod),
			mkTask("TSK-2", "Done thing", "done", "2026-07-11", mod), // filtered out
			mkTask("TSK-3", "No due date", "todo", "", mod),          // skipped (no date)
		},
	}}
	cfg := dataentryconfig.Feed{Sources: []dataentryconfig.FeedSource{
		{EntityType: "task", Where: []string{"status != done"}, Date: "due", Summary: "title", Alarm: "-PT9H"},
	}}
	d := newTestFeed(t, cfg, src)

	events, cursor, err := d.List(context.Background(), feedListOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1 (status filter + no-date skip); got %+v", len(events), events)
	}
	e := events[0]
	if e.UID != "task-TSK-1@rela" {
		t.Errorf("UID = %q, want task-TSK-1@rela", e.UID)
	}
	if e.Summary != "Renew passport" {
		t.Errorf("summary = %q", e.Summary)
	}
	if e.Start != time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC) {
		t.Errorf("start = %v, want 2026-07-10", e.Start)
	}
	if e.URL != "/entity/task/TSK-1" {
		t.Errorf("url = %q", e.URL)
	}
	if len(e.Alarms) != 1 || e.Alarms[0].Trigger != "-PT9H" {
		t.Errorf("alarms = %+v", e.Alarms)
	}
	if cursor != mod.UTC().Format(time.RFC3339Nano) {
		t.Errorf("cursor = %q, want max(modified) %q", cursor, mod.Format(time.RFC3339Nano))
	}
}

func TestDeclarativeFeed_MultiSourceMerges(t *testing.T) {
	mod := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	src := fakeSource{byType: map[string][]*entity.Entity{
		"task":  {mkTask("TSK-1", "Task A", "todo", "2026-07-10", mod)},
		"party": {{ID: "PTY-1", Type: "party", UpdatedAt: mod, Properties: map[string]any{"name": "Bash", "on": "2026-07-20"}}},
	}}
	cfg := dataentryconfig.Feed{Sources: []dataentryconfig.FeedSource{
		{EntityType: "task", Date: "due", Summary: "title"},
		{EntityType: "party", Date: "on", Summary: "name"},
	}}
	d := newTestFeed(t, cfg, src)

	events, _, err := d.List(context.Background(), feedListOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("want 2 merged events, got %d", len(events))
	}
	uids := map[string]bool{}
	for _, e := range events {
		uids[e.UID] = true
	}
	if !uids["task-TSK-1@rela"] || !uids["party-PTY-1@rela"] {
		t.Errorf("merged UIDs wrong: %v", uids)
	}
}

func TestDeclarativeFeed_SummaryFallsBackToDisplayProperty(t *testing.T) {
	mod := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	src := fakeSource{byType: map[string][]*entity.Entity{
		"task": {mkTask("TSK-1", "Fallback title", "todo", "2026-07-10", mod)},
	}}
	// No Summary → uses DisplayProperty "title".
	cfg := dataentryconfig.Feed{Sources: []dataentryconfig.FeedSource{{EntityType: "task", Date: "due"}}}
	d := newTestFeed(t, cfg, src)

	events, _, _ := d.List(context.Background(), feedListOpts{})
	if len(events) != 1 || events[0].Summary != "Fallback title" {
		t.Errorf("summary fallback failed: %+v", events)
	}
}

func TestDeclarativeFeed_SinceFiltersByModified(t *testing.T) {
	early := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	late := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	src := fakeSource{byType: map[string][]*entity.Entity{
		"task": {
			mkTask("TSK-1", "Old", "todo", "2026-07-10", early),
			mkTask("TSK-2", "New", "todo", "2026-07-11", late),
		},
	}}
	cfg := dataentryconfig.Feed{Sources: []dataentryconfig.FeedSource{{EntityType: "task", Date: "due", Summary: "title"}}}
	d := newTestFeed(t, cfg, src)

	// A cursor between early and late returns only the late one.
	since := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	events, cursor, err := d.List(context.Background(), feedListOpts{Since: since})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(events) != 1 || events[0].Summary != "New" {
		t.Fatalf("since filter wrong: %+v", events)
	}
	if cursor != late.UTC().Format(time.RFC3339Nano) {
		t.Errorf("cursor = %q, want %q", cursor, late.Format(time.RFC3339Nano))
	}
	// Ignoring since (empty) returns both — proving advisory-only correctness.
	all, _, _ := d.List(context.Background(), feedListOpts{})
	if len(all) != 2 {
		t.Errorf("full list want 2, got %d", len(all))
	}
}

func TestDeclarativeFeed_Get(t *testing.T) {
	mod := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	src := fakeSource{byType: map[string][]*entity.Entity{
		"task": {
			mkTask("TSK-1", "A", "todo", "2026-07-10", mod),
			mkTask("TSK-2", "Done", "done", "2026-07-11", mod),
		},
	}}
	cfg := dataentryconfig.Feed{Sources: []dataentryconfig.FeedSource{
		{EntityType: "task", Where: []string{"status != done"}, Date: "due", Summary: "title"},
	}}
	d := newTestFeed(t, cfg, src)
	ctx := context.Background()

	// get(uid) for TSK-1 == that uid's event in list.
	ev, ok, err := d.Get(ctx, "task-TSK-1@rela")
	if err != nil || !ok {
		t.Fatalf("Get TSK-1: ok=%v err=%v", ok, err)
	}
	if ev.Summary != "A" {
		t.Errorf("Get summary = %q", ev.Summary)
	}
	// Filtered-out entity → not in feed.
	if _, ok, _ := d.Get(ctx, "task-TSK-2@rela"); ok {
		t.Error("Get returned a filtered-out (done) entity")
	}
	// Unknown uid → not in feed.
	if _, ok, _ := d.Get(ctx, "task-NOPE@rela"); ok {
		t.Error("Get returned a nonexistent entity")
	}
	// Malformed uid → not ok, no error.
	if _, ok, err := d.Get(ctx, "garbage"); ok || err != nil {
		t.Errorf("Get(garbage) = ok:%v err:%v", ok, err)
	}
}

func TestDeclarativeFeed_GetMatchesList(t *testing.T) {
	// AC8: the same uid yields the same event from list and get.
	mod := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	src := fakeSource{byType: map[string][]*entity.Entity{
		"task": {mkTask("TSK-1", "Consistent", "todo", "2026-07-10", mod)},
	}}
	cfg := dataentryconfig.Feed{Sources: []dataentryconfig.FeedSource{{EntityType: "task", Date: "due", Summary: "title", Alarm: "-PT9H"}}}
	d := newTestFeed(t, cfg, src)
	ctx := context.Background()

	list, _, _ := d.List(ctx, feedListOpts{})
	got, ok, _ := d.Get(ctx, list[0].UID)
	if !ok {
		t.Fatal("Get did not find the listed event")
	}
	if got.UID != list[0].UID || got.Summary != list[0].Summary || got.Start != list[0].Start {
		t.Errorf("list/get mismatch:\n list=%+v\n get =%+v", list[0], got)
	}
}

func TestDeclarativeFeed_RruleAndEndDate(t *testing.T) {
	mod := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	// A task with an entity-level rrule property + a party with an end date range.
	task := mkTask("TSK-1", "Recurring", "todo", "2026-07-10", mod)
	task.Properties["schedule"] = "FREQ=WEEKLY"
	party := &entity.Entity{ID: "PTY-1", Type: "party", UpdatedAt: mod, Properties: map[string]any{
		"name": "Long party", "on": "2026-07-20", "ends": "2026-07-22",
	}}
	src := fakeSource{byType: map[string][]*entity.Entity{"task": {task}, "party": {party}}}

	cfg := dataentryconfig.Feed{Sources: []dataentryconfig.FeedSource{
		{EntityType: "task", Date: "due", Summary: "title", Rrule: "FREQ=DAILY"}, // literal, all tasks
	}}
	d := newTestFeed(t, cfg, src)
	events, _, _ := d.List(context.Background(), feedListOpts{})
	if len(events) != 1 || events[0].RRule != "FREQ=DAILY" {
		t.Fatalf("literal rrule not applied: %+v", events)
	}

	// Property-referenced rrule reads from the entity's own property.
	cfgProp := dataentryconfig.Feed{Sources: []dataentryconfig.FeedSource{
		{EntityType: "task", Date: "due", Summary: "title", Rrule: "schedule"},
	}}
	dp := newTestFeed(t, cfgProp, src)
	ev, _, _ := dp.List(context.Background(), feedListOpts{})
	if len(ev) != 1 || ev[0].RRule != "FREQ=WEEKLY" {
		t.Fatalf("property rrule not read from entity: %+v", ev)
	}

	// end_date maps a range.
	cfgEnd := dataentryconfig.Feed{Sources: []dataentryconfig.FeedSource{
		{EntityType: "party", Date: "on", EndDate: "ends", Summary: "name"},
	}}
	de := newTestFeed(t, cfgEnd, src)
	ee, _, _ := de.List(context.Background(), feedListOpts{})
	if len(ee) != 1 || ee[0].End != time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("end_date not mapped: %+v", ee)
	}
}

func TestDeclarativeFeed_EmptyFeed(t *testing.T) {
	src := fakeSource{byType: map[string][]*entity.Entity{}}
	cfg := dataentryconfig.Feed{Sources: []dataentryconfig.FeedSource{{EntityType: "task", Date: "due", Summary: "title"}}}
	d := newTestFeed(t, cfg, src)

	f, err := d.renderFeed(context.Background())
	if err != nil {
		t.Fatalf("renderFeed: %v", err)
	}
	if len(f.Events) != 0 {
		t.Errorf("empty feed has %d events", len(f.Events))
	}
	if f.Name != "cal" {
		t.Errorf("calendar name = %q, want the feed key 'cal'", f.Name)
	}
	// Serializes to a valid empty VCALENDAR.
	body := calfeed.ICal{Now: time.Now()}.RenderCollection(f)
	if len(body) == 0 {
		t.Error("empty feed rendered no bytes")
	}
}

func TestSplitFeedUID(t *testing.T) {
	tests := []struct {
		uid              string
		wantType, wantID string
		wantOK           bool
	}{
		{"task-TSK-001@rela", "task", "TSK-001", true}, // id contains a hyphen
		{"party-PTY-9@rela", "party", "PTY-9", true},
		{"task-x@other", "", "", false}, // wrong domain
		{"garbage", "", "", false},      // no @
		{"@rela", "", "", false},        // empty local
		{"task-@rela", "", "", false},   // empty id
	}
	for _, tc := range tests {
		typ, id, ok := splitFeedUID(tc.uid)
		if ok != tc.wantOK || typ != tc.wantType || id != tc.wantID {
			t.Errorf("splitFeedUID(%q) = (%q,%q,%v), want (%q,%q,%v)",
				tc.uid, typ, id, ok, tc.wantType, tc.wantID, tc.wantOK)
		}
	}
	// Round-trips with feedUID.
	if u := feedUID("task", "TSK-001"); u != "task-TSK-001@rela" {
		t.Errorf("feedUID = %q", u)
	}
}
