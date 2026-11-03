package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// setTestACL installs a as the ACL of a test app. Call it before NewRouter:
// the router and the handlers it builds may capture the ACL at construction,
// so a later swap is not guaranteed to reach every route.
//
// Tests set the ACL through this helper rather than assigning app.acl, so the
// field has one writer in the test suite and can change shape without
// touching every test. Two sites stay raw on purpose: the wiring assignment in
// rebindApp (test_helpers_test.go), which mirrors NewApp, and struct literals
// that build an App directly (newAppFromParts, newHandlerTestApp,
// newPostgresHookApp, list_incoming_relation_test.go), which set
// fieldResolver.
//
// a is stored exactly as given: a typed nil stays a non-nil interface, which
// some fail-closed tests depend on.
func setTestACL(tb testing.TB, app *App, a acl.ACL) {
	tb.Helper()
	if app == nil {
		tb.Fatal("setTestACL: app is nil")
		return
	}
	app.acl = a
}

// setTestResolver installs r as the field verdict resolver of a test app.
// Call it before NewRouter, for the reason given on [setTestACL], which also
// lists the sites that stay raw.
func setTestResolver(tb testing.TB, app *App, r FieldVerdictResolver) {
	tb.Helper()
	if app == nil {
		tb.Fatal("setTestResolver: app is nil")
		return
	}
	app.fieldResolver = r
}
