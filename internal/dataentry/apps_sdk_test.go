package dataentry

import (
	"strings"
	"testing"
)

// TestAppSDKReadiness pins the replayable-readiness contract added for
// TKT-5F9V56. The 'rela:ready' EVENT can be missed when a large script (e.g.
// _rela-editor.js) delays an app's inline code past the handshake; rela.ready
// (a Promise) and rela.whenReady(cb) are not-missable and are the supported way
// for an app to run code on connect. This froze the Today app until added, so
// guard it.
func TestAppSDKReadiness(t *testing.T) {
	src := appSDKSource()

	for _, want := range []string{
		"rela.ready",     // replayable Promise
		"rela.whenReady", // callback helper
		"readyPromise",   // backing promise
		"window.rela",    // still exposes the API object
		"'rela:ready'",   // back-compat event still dispatched
	} {
		if !strings.Contains(src, want) {
			t.Errorf("SDK source missing %q (readiness contract)", want)
		}
	}

	// The event must still be dispatched for back-compat, AND the promise must
	// resolve on the same handshake. Both happen in the handshake branch.
	if !strings.Contains(src, "resolveReady") {
		t.Error("SDK must resolve the ready promise on handshake")
	}

	// whenReady must ALWAYS return the promise (chainable) — i.e. there is a
	// `return readyPromise` outside any conditional in the helper. Guard against
	// regressing to the early-`return;` form that made whenReady(cb).then() throw.
	if !strings.Contains(src, "return readyPromise;") {
		t.Error("whenReady must always return readyPromise (chainable)")
	}
}

// TestAppSDKMethodsNotPollutedByReadiness ensures the readiness helpers are NOT
// added to the bridge method allow-list (they are local SDK helpers, not host
// calls). A method in appSDKMethods generates a send()-backed function; ready/
// whenReady/isReady must stay off it.
func TestAppSDKMethodsNotPollutedByReadiness(t *testing.T) {
	for _, m := range appSDKMethods {
		switch m {
		case "ready", "whenReady", "isReady":
			t.Errorf("%q must not be a bridge method (it's a local readiness helper)", m)
		}
	}
}
