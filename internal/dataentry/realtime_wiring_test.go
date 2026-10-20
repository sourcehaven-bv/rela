package dataentry

import (
	"context"
	"errors"
	"path/filepath"
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
// is still accepted; browsers get a config-error frame for data-entry.yaml,
// then refresh.
func TestReloadRealtime_ChannelsDrift(t *testing.T) {
	app, _ := setupReloadTestApp(t)
	fake := &fakeRealtime{schema: roomSchema{types: []string{"room"}}}
	app.State().Cfg.Channels = channelsCfg(dataentryconfig.FormField{Property: "body"})
	if err := SetRealtime(app, fake); err != nil {
		t.Fatal(err)
	}
	fake.schema = roomSchema{}
	ch := app.broker.subscribe()
	defer app.broker.unsubscribe(ch)
	if err := reloadRealtime(app); err != nil {
		t.Fatalf("reloadRealtime = %v; the realtime.yaml change is in force", err)
	}
	ev := nextEvent(t, ch)
	if ev.Name != "config-error" || !strings.Contains(ev.Data, `"file":"data-entry.yaml"`) {
		t.Fatalf("event = %s %s; want a config-error for data-entry.yaml", ev.Name, ev.Data)
	}
	if ev := nextEvent(t, ch); ev.Name != "refresh" {
		t.Fatalf("second event = %q; want refresh", ev.Name)
	}
}
