package main

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
)

// Without -mcp, wireRemoteMCP does nothing at all — no handler is built, and
// crucially SetRemoteMCP is never called, so an upgraded server that does not
// opt in cannot fail to start because of MCP (AC #2).
func TestWireRemoteMCP_DisabledIsANoop(t *testing.T) {
	// A zero App would panic if wireRemoteMCP touched it; that it does not is
	// the assertion. No JWT gate is configured either, so if the flag were
	// ignored this would return the SetRemoteMCP refusal instead of nil.
	if err := wireRemoteMCP(nil, nil, &serverFlags{remoteMCP: false}); err != nil {
		t.Fatalf("wireRemoteMCP with -mcp unset = %v, want nil (and no App access)", err)
	}
}

// AC #6 at the wiring layer: -mcp without the JWT identity flags must stop
// startup. The refusal itself lives in dataentry.SetRemoteMCP (and is tested
// there); this pins that rela-server actually surfaces it rather than logging
// and continuing, which would serve a CSRF-exempt endpoint with no
// authentication.
func TestWireRemoteMCP_WithoutJWTGateIsRefused(t *testing.T) {
	app := newBareApp(t)

	err := wireRemoteMCP(app, nil, &serverFlags{remoteMCP: true})
	if err == nil {
		t.Fatal("wireRemoteMCP() error = nil, want the missing-JWT refusal " +
			"propagated to the caller so startup can exit")
	}
	if !strings.Contains(err.Error(), "verified JWT identity") {
		t.Errorf("error = %q, want it to name the missing requirement", err)
	}
}

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

// newBareApp builds the minimum App that SetRemoteMCP can be called against.
// It needs no store or metamodel: the JWT-gate check runs before the factory,
// which is exactly what these tests exercise.
func newBareApp(t *testing.T) *dataentry.App {
	t.Helper()
	return &dataentry.App{}
}
