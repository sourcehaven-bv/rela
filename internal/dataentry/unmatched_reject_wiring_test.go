package dataentry

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// `unmatched_principal: reject` only takes effect when a JWT gate is wired:
// NewRouter snapshots a.jwtGate, and attachACLRequest keys the reject decision
// on that snapshot. An operator who configures reject in a deployment without
// a gate gets an acl.yaml key that silently does nothing — believing writes
// from unknown identities are denied when they are not (TKT-M60ZF5 / issue #1274).
//
// Policy.Validate cannot catch this. It only sees acl.yaml, and "is a gate
// wired" is a property of the SERVER, decided elsewhere and later. NewRouter is
// the first place both facts are in scope.
//
// A warning, not a hard error: refusing to start would turn a wiring omission
// into an outage for a deployment that is merely no stricter than the default.
// The mistake is believing a restriction is in force, so the fix is to say so.

// lockedWriter serializes writes and reads. bytes.Buffer is not safe for
// concurrent use, and newTestAppV1 builds a real app whose background
// goroutines can log after the constructor returns — so an unsynchronized
// buffer races the assertion. Mirrors appbuild_membership_warn_test.go.
type lockedWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *lockedWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// captureWarnings redirects the default logger for one test and returns the
// accumulated output.
//
// Callers must NOT use t.Parallel(): slog.SetDefault mutates process-global
// state, so two tests capturing at once would each see the other's output.
// Today the package is safe by scheduling accident (Go runs parallel tests
// after serial ones), which is not an invariant worth relying on.
func captureWarnings(t *testing.T) *lockedWriter {
	t.Helper()
	w := &lockedWriter{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return w
}

// rejectPolicy is a policy on which reject would genuinely be live: the mode
// plus the lookup it requires. Tests that want a DIFFERENT inert cause vary one
// field from this, so what each test is isolating stays visible.
func rejectPolicy() *acl.Policy {
	return &acl.Policy{
		UnmatchedPrincipal: acl.UnmatchedReject,
		UserEntityType:     "person",
		PrincipalProperty:  "email",
	}
}

func TestUnmatchedReject_WarnsWhenNoJWTGateWired(t *testing.T) {
	buf := captureWarnings(t)

	warnUnmatchedRejectWithoutJWTGate(rejectPolicy(), false)

	got := buf.String()
	if !strings.Contains(got, "unmatched_principal: reject") {
		t.Errorf("expected a warning naming the setting, got: %s", got)
	}
	// The warning has to say the setting is INERT. "reject is configured" alone
	// reads as confirmation that it is working, which is the belief being
	// corrected.
	if !strings.Contains(got, "NO effect") {
		t.Errorf("warning must state the setting has no effect, got: %s", got)
	}
}

// The discriminating case. A wired gate means reject genuinely applies, so
// warning would be false alarm — and an operator who learns to ignore this
// warning gets nothing from it in the deployment where it matters.
func TestUnmatchedReject_SilentWhenJWTGateWired(t *testing.T) {
	buf := captureWarnings(t)

	warnUnmatchedRejectWithoutJWTGate(rejectPolicy(), true)

	if got := buf.String(); got != "" {
		t.Errorf("no warning expected when a JWT gate is wired, got: %s", got)
	}
}

// The other modes are unaffected by gate wiring, so a missing gate is not
// noteworthy for them. `provision` has its own separate warning at the point it
// would have acted; duplicating it here would double-report one condition.
func TestUnmatchedReject_SilentForOtherModes(t *testing.T) {
	for _, mode := range []string{"", acl.UnmatchedAnonymous, acl.UnmatchedProvision} {
		name := mode
		if name == "" {
			name = "<empty>"
		}
		t.Run("mode="+name, func(t *testing.T) {
			buf := captureWarnings(t)

			p := rejectPolicy()
			p.UnmatchedPrincipal = mode
			warnUnmatchedRejectWithoutJWTGate(p, false)

			if got := buf.String(); got != "" {
				t.Errorf("no warning expected for mode %q, got: %s", mode, got)
			}
		})
	}
}

// A nil policy is not a reject configuration, and must not panic on the way to
// deciding that.
func TestUnmatchedReject_NilPolicyIsSilent(t *testing.T) {
	buf := captureWarnings(t)

	warnUnmatchedRejectWithoutJWTGate(nil, false)

	if got := buf.String(); got != "" {
		t.Errorf("no warning expected for a nil policy, got: %s", got)
	}
}

// The tests above call the helper directly, which proves the predicate is right
// and NOT that anything calls it. This one goes through NewRouter — the
// composition site, and the only place the wiring fact exists. Without it the
// helper could be correct and dead, which is the same class of gap as the
// silent no-op being fixed.
// newRejectApp builds an App whose ACL is a reject policy.
//
// Built directly rather than via mustNewACL: reject requires UserEntityType +
// PrincipalProperty, and enabling that lookup in turn requires a
// PrincipalLookup that mustNewACL does not supply.
func newRejectApp(t *testing.T) *App {
	t.Helper()
	app := newTestAppV1(t)
	d, err := acl.NewDeclarative(&acl.Policy{
		Roles:              map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments:        map[string]string{"alice": "viewer"},
		UserEntityType:     "person",
		PrincipalProperty:  "email",
		UnmatchedPrincipal: acl.UnmatchedReject,
	}, acl.NewStoreGraph(app.store), app.store,
		acl.WithPrincipalLookup(acl.NewStorePrincipalLookup(app.store)))
	if err != nil {
		t.Fatalf("acl.NewDeclarative: %v", err)
	}
	app.acl = d
	return app
}

func TestUnmatchedReject_NewRouterWarnsWhenGateMissing(t *testing.T) {
	app := newRejectApp(t)
	buf := captureWarnings(t)

	// No SetJWTGate call: this is exactly the deployment the issue describes.
	_ = app.NewRouter()

	if got := buf.String(); !strings.Contains(got, "no JWT gate is wired") {
		t.Errorf("NewRouter must warn that reject is inert without a JWT gate, got: %s", got)
	}
}

// The paired case, and the one that pins the ARGUMENT NewRouter passes. Without
// it, swapping `a.jwtGate != nil` for a literal `false` leaves the suite green:
// the test above only asserts a warning appeared, not that it appeared for the
// right reason, and every other test calls the helper directly.
func TestUnmatchedReject_NewRouterSilentWhenGateWired(t *testing.T) {
	app := newRejectApp(t)
	mustSetJWTGate(t, app)
	buf := captureWarnings(t)

	_ = app.NewRouter()

	if got := buf.String(); strings.Contains(got, "unmatched_principal") {
		t.Errorf("no unmatched_principal warning expected when a gate is wired, got: %s", got)
	}
}

// The SECOND way reject is inert, and the one the first version of this warning
// missed entirely (RR-0CJ47L): reject also requires the principal_property lookup —
// with it disabled the resolver never attempts a match, so "unmatched" is not
// something that can be observed.
//
// LoadPolicy refuses this combination, but NewDeclarative does NOT call
// Validate, so any construction path that skips LoadPolicy reaches exactly this
// state. That is why the warning checks it instead of assuming it away.
func TestUnmatchedReject_WarnsWhenLookupDisabled(t *testing.T) {
	buf := captureWarnings(t)

	p := rejectPolicy()
	p.UserEntityType = "" // lookup disabled; gate IS wired
	warnUnmatchedRejectWithoutJWTGate(p, true)

	got := buf.String()
	if !strings.Contains(got, "NO effect") {
		t.Errorf("expected a warning that reject is inert, got: %s", got)
	}
	// The message must name the CAUSE. "reject does nothing" is not actionable
	// without saying which of the two things to change.
	if !strings.Contains(got, "principal_property") {
		t.Errorf("warning must name the lookup as the cause, got: %s", got)
	}
	if strings.Contains(got, "no JWT gate is wired") {
		t.Errorf("warning names the wrong cause (a gate IS wired), got: %s", got)
	}
}

// A typed-nil *acl.Declarative stored in the ACL interface is NOT nil, so the
// `d != nil` guard in NewRouter's type switch is load-bearing. Adding a call to
// d.Policy() there widened the blast radius of getting it wrong: previously a
// typed-nil failed later, per request; now NewRouter itself would panic at
// construction.
//
// Removing that guard leaves every other test in this file green, so it needs
// its own. Policy() is also nil-receiver safe now, making this defense in
// depth rather than a single point of failure.
func TestUnmatchedReject_TypedNilDeclarativeDoesNotPanic(t *testing.T) {
	app := newTestAppV1(t)
	var d *acl.Declarative
	app.acl = d // typed nil: non-nil interface, nil pointer

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewRouter panicked on a typed-nil *acl.Declarative: %v", r)
		}
	}()
	_ = app.NewRouter()
}
