package dataentrywire

import "testing"

// The no-op watcher must satisfy the MCP Watcher contract without doing
// anything: on stateless HTTP there is no session to push
// resources/list_changed to, so a real filesystem watcher would burn an
// inotify handle feeding a callback whose notification can never be sent.
//
// Calling every method here is the point — a future Watcher method added
// without an implementation would fail to compile, and a panicking stub would
// fail this test rather than a request in production.
func TestNoopWatcher_SatisfiesTheContract(t *testing.T) {
	var w noopWatcher

	called := false
	if err := w.Start(func() { called = true }); err != nil {
		t.Errorf("Start() = %v, want nil", err)
	}
	w.Pause()
	w.Resume()
	w.Stop()

	if called {
		t.Error("the onChange callback fired; the no-op watcher must never " +
			"invoke it — there is no transport to deliver the notification on")
	}
}
