package appbuild

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// TestNewElevationAuditor_NilSinkYieldsNilInterface is the whole reason this
// wrapper exists (TKT-ACSBSA).
//
// audit.NewElevationRecorder returns a CONCRETE pointer, so `return nil`
// from it lands in lua's interface-typed ElevationRecorder field as a TYPED
// nil — which is != nil, so the `ElevationRecorder == nil` guard passes and
// the first elevated read nil-derefs. That kills the process on the path
// that is supposed to degrade quietly (no audit sink configured). Verified
// as a real failure mode in RR-5QQL1Z, not reasoned about.
//
// The assertion goes through lua.WriteDeps — the actual field the value
// lands in — rather than checking the return value directly. That detail IS
// the test: `NewElevationAuditor(nil) != nil` is false even for a
// concrete-face signature (a nil *T compares equal to nil), so a direct
// check silently passes against the very mutation it should catch. Boxing
// into the interface first is what makes the typed nil observable, and it is
// what production does.
func TestNewElevationAuditor_NilSinkYieldsNilInterface(t *testing.T) {
	t.Parallel()

	deps := lua.WriteDeps{ElevationRecorder: NewElevationAuditor(nil)}

	// Exactly the guard Runtime.recordElevatedReads performs.
	if deps.ElevationRecorder != nil {
		t.Errorf("ElevationRecorder = %#v after wiring a nil sink, want nil -- "+
			"a typed nil passes lua's `ElevationRecorder == nil` guard and "+
			"nil-derefs on the first elevated read", deps.ElevationRecorder)
	}
}

// TestNewElevationAuditor_RealSinkIsWired is the other half: a real sink
// must produce a usable recorder, or elevated reads would go untraced while
// the nil-handling above looked correct.
func TestNewElevationAuditor_RealSinkIsWired(t *testing.T) {
	t.Parallel()

	deps := lua.WriteDeps{ElevationRecorder: NewElevationAuditor(&audit.Memory{})}
	if deps.ElevationRecorder == nil {
		t.Error("a real audit sink produced no recorder -- elevated reads would " +
			"happen with no trace")
	}
}
