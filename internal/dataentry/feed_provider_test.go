package dataentry

import (
	"context"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/calfeed"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
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
					"title":     {Type: metamodel.PropertyTypeString},
					"due":       {Type: metamodel.PropertyTypeDate},
					"starts_at": {Type: metamodel.PropertyTypeDatetime},
					"ends_at":   {Type: metamodel.PropertyTypeDatetime},
					"status":    {Type: metamodel.PropertyTypeString},
					"schedule":  {Type: metamodel.PropertyTypeString},
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
	// nil redactor: these tests exercise mapping and filtering with no ACL
	// policy, which is the no-policy parity case (nothing hidden).
	return newTestFeedWithRedactor(t, cfg, src, nil)
}

func newTestFeedWithRedactor(
	t *testing.T, cfg dataentryconfig.Feed, src entitySource, r visibility.FieldRedactor,
) *declarativeFeed {
	t.Helper()
	d, err := newDeclarativeFeed("cal", cfg, feedTestMeta(), src, testLinker, r)
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
	if e.UID != "task--TSK-1@rela" {
		t.Errorf("UID = %q, want task--TSK-1@rela", e.UID)
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

func TestDeclarativeFeed_DatetimeSourceIsTimed(t *testing.T) {
	mod := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	src := fakeSource{byType: map[string][]*entity.Entity{
		"task": {{
			ID: "TSK-1", Type: "task", UpdatedAt: mod,
			Properties: map[string]any{
				"title":     "Standup",
				"starts_at": "2026-07-13T14:30:00Z",
				"ends_at":   "2026-07-13T15:00:00Z",
			},
		}},
	}}
	cfg := dataentryconfig.Feed{Sources: []dataentryconfig.FeedSource{
		{EntityType: "task", Date: "starts_at", EndDate: "ends_at", Summary: "title"},
	}}
	d := newTestFeed(t, cfg, src)

	events, _, err := d.List(context.Background(), feedListOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	e := events[0]
	if !e.Timed {
		t.Error("a datetime source must produce a timed event")
	}
	// Start/End carry the full time-of-day, not a truncated day.
	if want := time.Date(2026, 7, 13, 14, 30, 0, 0, time.UTC); !e.Start.Equal(want) {
		t.Errorf("start = %v, want %v (time-of-day preserved)", e.Start, want)
	}
	if want := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC); !e.End.Equal(want) {
		t.Errorf("end = %v, want %v", e.End, want)
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
	if !uids["task--TSK-1@rela"] || !uids["party--PTY-1@rela"] {
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
	ev, ok, err := d.Get(ctx, "task--TSK-1@rela")
	if err != nil || !ok {
		t.Fatalf("Get TSK-1: ok=%v err=%v", ok, err)
	}
	if ev.Summary != "A" {
		t.Errorf("Get summary = %q", ev.Summary)
	}
	// Filtered-out entity → not in feed.
	if _, ok, _ := d.Get(ctx, "task--TSK-2@rela"); ok {
		t.Error("Get returned a filtered-out (done) entity")
	}
	// Unknown uid → not in feed.
	if _, ok, _ := d.Get(ctx, "task--NOPE@rela"); ok {
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

func TestDeclarativeFeed_GetRejectsTypeMismatch(t *testing.T) {
	// A source of type "task", but the store (fake here mimicking the production
	// by-id lookup) returns an entity of a different type for the same id.
	// Get must reject the mismatch rather than map it under the task source.
	mod := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	wrongType := &entity.Entity{ID: "TSK-1", Type: "party", UpdatedAt: mod,
		Properties: map[string]any{"name": "not a task", "on": "2026-07-10"}}
	src := byIDSource{e: wrongType}
	cfg := dataentryconfig.Feed{Sources: []dataentryconfig.FeedSource{{EntityType: "task", Date: "due", Summary: "title"}}}
	d := newTestFeed(t, cfg, src)

	if _, ok, err := d.Get(context.Background(), "task--TSK-1@rela"); ok || err != nil {
		t.Errorf("Get should reject a type-mismatched entity: ok=%v err=%v", ok, err)
	}
}

// byIDSource mimics the production reader: getEntity resolves by id regardless
// of the requested type (the type is only used for the ACL gate upstream).
type byIDSource struct{ e *entity.Entity }

func (s byIDSource) listType(_ context.Context, t string) ([]*entity.Entity, error) {
	if s.e != nil && s.e.Type == t {
		return []*entity.Entity{s.e}, nil
	}
	return nil, nil
}

func (s byIDSource) getEntity(_ context.Context, _, id string) (*entity.Entity, bool, error) {
	if s.e != nil && s.e.ID == id {
		return s.e, true, nil // returns whatever type it has, ignoring the requested type
	}
	return nil, false, nil
}

func TestDeclarativeFeed_MalformedPropertyRRuleDropped(t *testing.T) {
	mod := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	// schedule holds a non-RRULE string; the event should render with no rrule
	// rather than emit a broken RRULE that breaks the whole feed.
	e := mkTask("TSK-1", "X", "todo", "2026-07-10", mod)
	e.Properties["schedule"] = "not a rule at all"
	src := fakeSource{byType: map[string][]*entity.Entity{"task": {e}}}
	cfg := dataentryconfig.Feed{Sources: []dataentryconfig.FeedSource{
		{EntityType: "task", Date: "due", Summary: "title", Rrule: "schedule"},
	}}
	d := newTestFeed(t, cfg, src)
	events, _, _ := d.List(context.Background(), feedListOpts{})
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	if events[0].RRule != "" {
		t.Errorf("malformed property rrule should be dropped, got %q", events[0].RRule)
	}

	// A well-formed property rule is kept.
	e.Properties["schedule"] = "FREQ=WEEKLY"
	events, _, _ = d.List(context.Background(), feedListOpts{})
	if events[0].RRule != "FREQ=WEEKLY" {
		t.Errorf("valid property rrule dropped: %q", events[0].RRule)
	}
}

func TestDeclarativeFeed_UnparseableDateSkipped(t *testing.T) {
	// An entity whose date property holds garbage is skipped (not a hard error) —
	// pins the deliberate //nolint:nilerr tolerate-bad-data path.
	mod := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	good := mkTask("TSK-1", "Good", "todo", "2026-07-10", mod)
	bad := mkTask("TSK-2", "Bad date", "todo", "not-a-date", mod)
	src := fakeSource{byType: map[string][]*entity.Entity{"task": {good, bad}}}
	cfg := dataentryconfig.Feed{Sources: []dataentryconfig.FeedSource{{EntityType: "task", Date: "due", Summary: "title"}}}
	d := newTestFeed(t, cfg, src)

	events, _, err := d.List(context.Background(), feedListOpts{})
	if err != nil {
		t.Fatalf("a bad date should skip the entity, not error: %v", err)
	}
	if len(events) != 1 || events[0].Summary != "Good" {
		t.Errorf("want only the good event, got %+v", events)
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
		{"task--TSK-001@rela", "task", "TSK-001", true}, // id contains a hyphen
		{"party--PTY-9@rela", "party", "PTY-9", true},
		// Hyphenated entity types MUST round-trip (a single-hyphen separator
		// would mis-split these — the reason "--" is the separator).
		{"test-case--TC-1@rela", "test-case", "TC-1", true},
		{"review-response--RR-3@rela", "review-response", "RR-3", true},
		{"doc-task--DOC-9@rela", "doc-task", "DOC-9", true},
		{"task--x@other", "", "", false},   // wrong domain
		{"garbage", "", "", false},         // no @
		{"@rela", "", "", false},           // empty local
		{"task--@rela", "", "", false},     // empty id
		{"task-TSK-1@rela", "", "", false}, // single hyphen: not our format
	}
	for _, tc := range tests {
		typ, id, ok := splitFeedUID(tc.uid)
		if ok != tc.wantOK || typ != tc.wantType || id != tc.wantID {
			t.Errorf("splitFeedUID(%q) = (%q,%q,%v), want (%q,%q,%v)",
				tc.uid, typ, id, ok, tc.wantType, tc.wantID, tc.wantOK)
		}
	}
	// Round-trips with feedUID, including hyphenated types.
	for _, tc := range []struct{ typ, id string }{
		{"task", "TSK-001"},
		{"test-case", "TC-1"},
		{"review-response", "RR-3"},
	} {
		typ, id, ok := splitFeedUID(feedUID(tc.typ, tc.id))
		if !ok || typ != tc.typ || id != tc.id {
			t.Errorf("round-trip feedUID(%q,%q) = (%q,%q,%v)", tc.typ, tc.id, typ, id, ok)
		}
	}
}

// feedFakeRedactor hides a fixed set of property names.
type feedFakeRedactor struct{ hide []string }

func (f feedFakeRedactor) HiddenProperties(context.Context, *entity.Entity) map[string]struct{} {
	out := map[string]struct{}{}
	for _, h := range f.hide {
		out[h] = struct{}{}
	}
	return out
}

// TestDeclarativeFeed_RedactsHiddenProperties pins that field-level `visible:`
// ACL reaches the ICS feed.
//
// listType gates ROWS — an entity the principal cannot read never reaches the
// mapper. It does not touch FIELDS, and docs/acl-security.md commits to
// redaction on "every HTTP read shape". The feed was one that skipped it: a
// source mapping `description: notes` served that property verbatim to a
// principal whose role redacts it.
func TestDeclarativeFeed_RedactsHiddenProperties(t *testing.T) {
	cfg := dataentryconfig.Feed{Sources: []dataentryconfig.FeedSource{{
		EntityType: "task", Date: "due", Summary: "title", Description: "notes",
	}}}
	task := mkTask("TSK-1", "Buy milk", "todo", "2026-08-12", time.Now())
	task.Properties["notes"] = "classified"
	src := fakeSource{byType: map[string][]*entity.Entity{"task": {task}}}

	// Precondition: with nothing hidden the value IS rendered, so the assertion
	// below cannot pass for an unrelated reason.
	open, _, err := newTestFeed(t, cfg, src).List(t.Context(), feedListOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(open) != 1 || open[0].Description != "classified" {
		t.Fatalf("precondition: description not rendered at all: %+v", open)
	}

	d := newTestFeedWithRedactor(t, cfg, src, feedFakeRedactor{hide: []string{"notes"}})
	got, _, err := d.List(t.Context(), feedListOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 event, got %d", len(got))
	}
	if got[0].Description != "" {
		t.Errorf("a `visible:`-hidden property reached the feed: %q", got[0].Description)
	}
	if got[0].Summary != "Buy milk" {
		t.Errorf("a visible property was dropped: %q", got[0].Summary)
	}
}

// TestDeclarativeFeed_RedactionDoesNotChangeMembership pins the ORDER: the
// `where:` filter runs against the raw entity, before redaction.
//
// Redacting first would make a hidden property read as empty inside the filter,
// so the same feed would contain different EVENTS for different readers — an
// entity silently dropping off the calendar because a field the reader cannot
// see was filtered on. Which entities a feed selects is operator-authored; what
// their fields say is the reader's business.
func TestDeclarativeFeed_RedactionDoesNotChangeMembership(t *testing.T) {
	cfg := dataentryconfig.Feed{Sources: []dataentryconfig.FeedSource{{
		EntityType: "task", Date: "due", Summary: "title",
		Where: []string{"status = todo"},
	}}}
	src := fakeSource{byType: map[string][]*entity.Entity{"task": {
		mkTask("TSK-1", "Buy milk", "todo", "2026-08-12", time.Now()),
	}}}

	// `status` is the filtered property AND hidden from this reader.
	d := newTestFeedWithRedactor(t, cfg, src, feedFakeRedactor{hide: []string{"status"}})
	got, _, err := d.List(t.Context(), feedListOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("hiding the filtered property changed feed membership: got %d events, want 1", len(got))
	}
}

// TestDeclarativeFeed_RedactionCopies pins that redaction never mutates the
// shared store entity. The snapshot is process-wide, so an in-place delete would
// redact for every other reader — including write-prep reads, where a missing
// property is an ERASURE.
func TestDeclarativeFeed_RedactionCopies(t *testing.T) {
	task := mkTask("TSK-1", "Buy milk", "todo", "2026-08-12", time.Now())
	task.Properties["notes"] = "classified"
	cfg := dataentryconfig.Feed{Sources: []dataentryconfig.FeedSource{{
		EntityType: "task", Date: "due", Summary: "title", Description: "notes",
	}}}
	src := fakeSource{byType: map[string][]*entity.Entity{"task": {task}}}

	d := newTestFeedWithRedactor(t, cfg, src, feedFakeRedactor{hide: []string{"notes"}})
	if _, _, err := d.List(t.Context(), feedListOpts{}); err != nil {
		t.Fatalf("List: %v", err)
	}
	if task.Properties["notes"] != "classified" {
		t.Error("redaction mutated the shared store entity")
	}
}
