package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// cascadeScriptRunner builds the automation-cascade script runner, granting
// bypass_acl closures a RAW read handle (admin.get_entity et al.) plus the
// audit sink that records its use (TKT-ACSBSA).
//
// The capability is inert until an action is allow_acl_bypass AND the
// cascade Mutator offers ElevatedProvider — the same two keys that gate
// elevated writes. The reader is deliberately the same raw store
// readDeps.WritePrepStore holds, named through visibility.Unrestricted so
// this ungated path shows up in the grep that enumerates them (TKT-1WV50C).
func cascadeScriptRunner(
	engine *script.Engine, readDeps lua.ReadDeps, st store.Store, sink audit.Audit,
) *script.LuaScriptRunner {
	return script.NewLuaScriptRunnerWithElevatedReads(engine, readDeps, script.ReadElevation{
		Reader:   visibility.Unrestricted(st),
		Recorder: NewElevationAuditor(sink),
	})
}

// NewElevationAuditor returns the elevated-read audit recorder over sink, or
// a genuinely nil interface when no sink is wired (which the lua side reads
// as "recording disabled").
//
// Exported so appbuildtest can build the SAME elevation bundle production
// does — a fixture that granted elevated reads without the audit trace would
// let a test pass while the wiring it stands in for has the audit gap.
//
// This wrapper exists for ONE reason: the nil conversion. audit's
// constructor returns *audit.ElevationRecorder, and assigning a nil one
// straight into lua's interface-typed ElevationRecorder field yields a TYPED
// nil, which is != nil — lua's `ElevationRecorder == nil` guard would pass
// and the first elevated read would nil-deref, killing the process on a path
// that is supposed to degrade quietly. Returning the INTERFACE type here
// makes the nil a real nil. Same trap as RR-5QQL1Z.
func NewElevationAuditor(sink audit.Audit) lua.ElevationRecorder {
	if sink == nil {
		return nil
	}
	return audit.NewElevationRecorder(sink)
}
