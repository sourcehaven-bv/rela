package dataentry

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRealtime is a realtimeReloader whose next Reload returns err.
type fakeRealtime struct {
	err   error
	calls int
}

func (f *fakeRealtime) Reload(context.Context) error {
	f.calls++
	return f.err
}

func TestSetRealtime_RejectsNil(t *testing.T) {
	app, _ := setupReloadTestApp(t)
	if err := SetRealtime(app, nil); err == nil {
		t.Fatal("SetRealtime accepted a nil holder")
	}
}

// A rejected realtime.yaml reload keeps the previous config and sends a
// config-error frame naming realtime.yaml, without the host path. An
// accepted one sends nothing: no browser consumes the realtime config yet.
func TestReloadRealtime_Broadcasts(t *testing.T) {
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
	select {
	case ev := <-ch:
		t.Fatalf("accepted reload broadcast %q", ev.Name)
	default:
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
