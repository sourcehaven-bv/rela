package dataentry

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// fakeRealtime is a realtimeReloader whose next Reload returns err and
// whose config in force is schema (nil: realtime off).
type fakeRealtime struct {
	err    error
	calls  int
	schema dataentryconfig.ChannelSchema
}

func (f *fakeRealtime) Reload(context.Context) error {
	f.calls++
	return f.err
}

func (f *fakeRealtime) ChannelSchema() dataentryconfig.ChannelSchema { return f.schema }

// roomSchema is a realtime config with channel type room, whose message
// type text has the required property body.
type roomSchema struct{ types []string }

func (s roomSchema) ChannelTypeNames() []string { return s.types }
func (roomSchema) MessageTypeNames(string) []string {
	return []string{"text"}
}

func (roomSchema) MessageProperties(_, mt string) []dataentryconfig.ChannelProperty {
	if mt != "text" {
		return nil
	}
	return []dataentryconfig.ChannelProperty{{Name: "body", Type: "markdown", Required: true}}
}

// BusyFacts: every declared type has a session and attaches to tickets.
func (s roomSchema) BusyFacts(ct string) (dataentryconfig.BusyFacts, bool) {
	if !slices.Contains(s.types, ct) {
		return dataentryconfig.BusyFacts{}, false
	}
	return dataentryconfig.BusyFacts{AttachTo: []string{"ticket"}, HasSession: true, HasWorkingState: true}, true
}

// channelsCfg presents room's text messages with field.
func channelsCfg(field dataentryconfig.FormField) map[string]dataentryconfig.ChannelPresentation {
	return map[string]dataentryconfig.ChannelPresentation{"room": {Messages: map[string]dataentryconfig.MessagePresentation{
		"text": {Fields: []dataentryconfig.FormField{field}},
	}}}
}

// nextEvent returns the next broadcast event, failing when there is none.
func nextEvent(t *testing.T, ch chan sseEvent) sseEvent {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	default:
		t.Fatal("no event broadcast")
	}
	return sseEvent{}
}

func TestSetRealtime_RejectsNil(t *testing.T) {
	app, _ := setupReloadTestApp(t)
	if err := SetRealtime(app, nil); err == nil {
		t.Fatal("SetRealtime accepted a nil holder")
	}
}

// A rejected realtime.yaml reload keeps the previous config and sends a
// config-error frame naming realtime.yaml, without the host path. An
// accepted one sends refresh, so the SPA reloads the channel descriptor.
func TestReloadRealtime_BroadcastsRefresh(t *testing.T) {
	root := t.TempDir()
	app, _ := setupReloadTestAppAt(t, root)
	fake := &fakeRealtime{}
	if err := SetRealtime(app, fake); err != nil {
		t.Fatal(err)
	}
	ch := app.broker.subscribe()
	defer app.broker.unsubscribe(ch)

	if err := reloadRealtime(app); err != nil {
		t.Fatalf("accepted reload returned %v", err)
	}
	if ev := nextEvent(t, ch); ev.Name != "refresh" {
		t.Fatalf("accepted reload broadcast %q; want refresh", ev.Name)
	}

	fake.err = errors.New("read realtime.yaml: open " + filepath.Join(root, "realtime.yaml") + ": denied")
	var ce *configError
	if err := reloadRealtime(app); !errors.As(err, &ce) {
		t.Fatalf("rejected reload returned %v; want a *configError", err)
	}
	select {
	case ev := <-ch:
		if ev.Name != "config-error" || !strings.Contains(ev.Data, `"file":"realtime.yaml"`) {
			t.Errorf("event = %s %s; want config-error for realtime.yaml", ev.Name, ev.Data)
		}
		if strings.Contains(ev.Data, root) {
			t.Errorf("config-error frame carries the project root: %s", ev.Data)
		}
	default:
		t.Fatal("no event broadcast")
	}
	if fake.calls != 2 {
		t.Errorf("Reload called %d times, want 2", fake.calls)
	}
}

// Without a holder there is nothing to reload or watch.
func TestReloadRealtime_NotWired(t *testing.T) {
	app, _ := setupReloadTestApp(t)
	if err := reloadRealtime(app); err != nil {
		t.Fatal(err)
	}
	if err := watchRealtime(app, nil); err != nil {
		t.Fatal(err)
	}
}

// SetRealtime refuses a channels: section that names what realtime.yaml
// does not declare, and accepts one with realtime off.
func TestSetRealtime_ChecksChannels(t *testing.T) {
	app, _ := setupReloadTestApp(t)
	app.State().Cfg.Channels = channelsCfg(dataentryconfig.FormField{Property: "body", Hidden: true})
	err := SetRealtime(app, &fakeRealtime{schema: roomSchema{types: []string{"room"}}})
	if err == nil || !strings.Contains(err.Error(), "cannot be hidden") {
		t.Fatalf("SetRealtime = %v; want the hidden required property refused", err)
	}
	if app.realtime != nil {
		t.Fatal("a refused holder was installed")
	}
	if err := SetRealtime(app, &fakeRealtime{}); err != nil {
		t.Fatalf("realtime off: %v; want a warning only", err)
	}
}

// A data-entry.yaml reload whose channels: section names an unknown
// channel type is refused with a config-error frame.
func TestReloadConfig_RefusesInvalidChannels(t *testing.T) {
	app, fs := setupReloadTestApp(t)
	if err := SetRealtime(app, &fakeRealtime{schema: roomSchema{types: []string{"room"}}}); err != nil {
		t.Fatal(err)
	}
	_ = fs.WriteFile(app.paths.Root+"/"+ConfigFile,
		[]byte(testReloadConfigYAML+"channels:\n  hall: {}\n"), 0o644)
	ch := app.broker.subscribe()
	defer app.broker.unsubscribe(ch)
	var ce *configError
	if err := app.reloadConfig(); !errors.As(err, &ce) {
		t.Fatalf("reloadConfig = %v; want a *configError", err)
	}
	ev := nextEvent(t, ch)
	if ev.Name != "config-error" || !strings.Contains(ev.Data, `declares no channel type`) {
		t.Fatalf("event = %s %s", ev.Name, ev.Data)
	}
	if app.Cfg().Channels != nil {
		t.Fatal("the refused config was published")
	}
}

// A realtime.yaml reload that drops a channel type data-entry.yaml presents
// is still accepted; browsers get a config-error frame for data-entry.yaml
// that says realtime.yaml was applied, then refresh.
func TestReloadRealtime_ChannelsDrift(t *testing.T) {
	app, fs := setupReloadTestApp(t)
	fake := &fakeRealtime{schema: roomSchema{types: []string{"room"}}}
	if err := SetRealtime(app, fake); err != nil {
		t.Fatal(err)
	}
	_ = fs.WriteFile(app.paths.Root+"/"+ConfigFile,
		[]byte(testReloadConfigYAML+"channels:\n  room: {}\n"), 0o644)
	if err := app.reloadConfig(); err != nil {
		t.Fatal(err)
	}
	fake.schema = roomSchema{}
	ch := app.broker.subscribe()
	defer app.broker.unsubscribe(ch)
	if err := reloadRealtime(app); err != nil {
		t.Fatalf("reloadRealtime = %v; the realtime.yaml change is in force", err)
	}
	ev := nextEvent(t, ch)
	if ev.Name != "config-error" || !strings.Contains(ev.Data, `"file":"data-entry.yaml"`) ||
		!strings.Contains(ev.Data, `"applied":"realtime.yaml"`) {

		t.Fatalf("event = %s %s; want a config-error for data-entry.yaml with realtime.yaml applied", ev.Name, ev.Data)
	}
	if ev := nextEvent(t, ch); ev.Name != "refresh" {
		t.Fatalf("second event = %q; want refresh", ev.Name)
	}
}

// An operator who saves data-entry.yaml before realtime.yaml gets the
// data-entry.yaml change refused first (it names a channel type realtime.yaml
// does not declare yet), and in force once realtime.yaml declares it: the
// realtime reload re-reads data-entry.yaml from disk.
func TestReloadRealtime_RereadsDataEntryConfig(t *testing.T) {
	app, fs := setupReloadTestApp(t)
	fake := &fakeRealtime{schema: roomSchema{}}
	if err := SetRealtime(app, fake); err != nil {
		t.Fatal(err)
	}
	_ = fs.WriteFile(app.paths.Root+"/"+ConfigFile,
		[]byte(testReloadConfigYAML+"channels:\n  room: {}\n"), 0o644)
	if err := app.reloadConfig(); err == nil {
		t.Fatal("reloadConfig accepted a channel type realtime.yaml does not declare")
	}
	if app.Cfg().Channels != nil {
		t.Fatal("the refused config was published")
	}

	fake.schema = roomSchema{types: []string{"room"}}
	ch := app.broker.subscribe()
	defer app.broker.unsubscribe(ch)
	if err := reloadRealtime(app); err != nil {
		t.Fatal(err)
	}
	if _, ok := app.Cfg().Channels["room"]; !ok {
		t.Fatalf("channels = %v; want the on-disk data-entry.yaml in force", app.Cfg().Channels)
	}
	if ev := nextEvent(t, ch); ev.Name != "refresh" {
		t.Fatalf("event = %s %s; want refresh only", ev.Name, ev.Data)
	}
	select {
	case ev := <-ch:
		t.Fatalf("extra event %s %s", ev.Name, ev.Data)
	default:
	}
}

// busyKanban is a ticket board whose card shows the busy state of
// channelType (TKT-21G6JS).
func busyKanban(channelType string) map[string]dataentryconfig.Kanban {
	return map[string]dataentryconfig.Kanban{"board": {
		EntityType: "ticket", ColumnProperty: "status",
		Card: dataentryconfig.KanbanCard{Fields: []dataentryconfig.KanbanCardField{{Busy: channelType}}},
	}}
}

// SetRealtime checks a kanban busy field without a channels: section: it
// refuses an undeclared channel type, and with realtime off it logs a
// warning and accepts the config.
func TestSetRealtime_ChecksBusyField(t *testing.T) {
	app, _ := setupReloadTestApp(t)
	app.State().Cfg.Kanbans = busyKanban("nope")
	err := SetRealtime(app, &fakeRealtime{schema: roomSchema{types: []string{"run"}}})
	if err == nil || !strings.Contains(err.Error(), `declares no channel type "nope"`) {
		t.Fatalf("SetRealtime = %v; want the undeclared busy type refused", err)
	}

	logs := captureWarnings(t)
	app.State().Cfg.Kanbans = busyKanban("run")
	if err := SetRealtime(app, &fakeRealtime{}); err != nil {
		t.Fatalf("realtime off: %v; want a warning only", err)
	}
	if got := logs.String(); !strings.Contains(got, "this busy field shows nothing") {
		t.Fatalf("logs = %s; want the realtime-off warning for the busy field", got)
	}
}

// busyKanbanYAML is a data-entry.yaml kanban block whose card shows the
// busy state of channelType.
func busyKanbanYAML(channelType string) string {
	return `kanbans:
  board:
    entity_type: ticket
    title: Board
    column_property: status
    columns:
      - value: open
        label: Open
    card:
      title: title
      fields:
        - busy: ` + channelType + "\n"
}

// A data-entry.yaml reload checks a busy field against realtime.yaml as
// boot does (AC 8 of TKT-21G6JS): an undeclared channel type is refused
// and the last good config stays in force, and a declared one is applied.
func TestReloadConfig_ChecksBusyField(t *testing.T) {
	// A kanban needs an enum column property.
	meta := strings.Replace(testReloadMetamodelYAML, "      status:\n        type: string\n",
		"      status:\n        type: enum\n        values: [open, done]\n", 1)
	if meta == testReloadMetamodelYAML {
		t.Fatal("the schema has no status property to turn into an enum")
	}
	app, fs := setupReloadTestAppWith(t, "/project", meta)
	if err := SetRealtime(app, &fakeRealtime{schema: roomSchema{types: []string{"run"}}}); err != nil {
		t.Fatal(err)
	}
	path := app.paths.Root + "/" + ConfigFile
	_ = fs.WriteFile(path, []byte(testReloadConfigYAML+busyKanbanYAML("nope")), 0o644)
	ch := app.broker.subscribe()
	defer app.broker.unsubscribe(ch)
	var ce *configError
	if err := app.reloadConfig(); !errors.As(err, &ce) {
		t.Fatalf("reloadConfig = %v; want a *configError", err)
	}
	if ev := nextEvent(t, ch); ev.Name != "config-error" || !strings.Contains(ev.Data, `declares no channel type`) {
		t.Fatalf("event = %s %s", ev.Name, ev.Data)
	}
	if _, ok := app.Cfg().Kanbans["board"]; ok {
		t.Fatal("the refused config was published")
	}

	_ = fs.WriteFile(path, []byte(testReloadConfigYAML+busyKanbanYAML("run")), 0o644)
	if err := app.reloadConfig(); err != nil {
		t.Fatalf("reloadConfig = %v; want the declared busy type accepted", err)
	}
	if got := app.Cfg().Kanbans["board"].Card.Fields; len(got) != 1 || got[0].Busy != "run" {
		t.Fatalf("board card fields = %+v; want the busy field in force", got)
	}
}
