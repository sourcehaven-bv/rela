package script

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The TKT-D8T148 / TKT-ACSBSA two-key gate lives HERE, in Run: elevation is
// granted only when the action is allow_acl_bypass AND the per-cascade
// Mutator offers ElevatedProvider. internal/lua cannot test this — it
// receives WriteDeps already assembled. These tests inspect the deps this
// package hands the executor.

// depsCapturingExecutor records the lua.WriteDeps it was called with, which
// is exactly the elevation decision this package makes.
type depsCapturingExecutor struct{ got lua.WriteDeps }

func (d *depsCapturingExecutor) ExecuteCode(
	_ context.Context, _ string, deps lua.WriteDeps, _, _ *entity.Entity,
) error {
	d.got = deps
	return nil
}

func (d *depsCapturingExecutor) ExecuteFile(
	_ context.Context, _ string, deps lua.WriteDeps, _, _ *entity.Entity,
) error {
	d.got = deps
	return nil
}

// elevatingMutator offers the optional ElevatedProvider capability.
type elevatingMutator struct{ autocascade.Mutator }

func (e elevatingMutator) Elevated() autocascade.Mutator { return e }

// plainMutator does NOT offer ElevatedProvider — a Mutator that declines to
// grant elevation, which no flag can override.
type plainMutator struct{ autocascade.Mutator }

// errStubRead is returned by every stubReader method. The stub is never
// called by these tests — they assert which reader is HANDED OVER, not what
// it returns — so a sentinel error is the honest zero behavior; returning a
// nil entity with a nil error would model a contract no real reader has.
var errStubRead = errors.New("stubReader: not expected to be called")

// stubReader is a non-nil lua.EntityReader standing in for the raw store.
type stubReader struct{}

func (stubReader) GetEntity(context.Context, string) (*entity.Entity, error) {
	return nil, errStubRead
}
func (stubReader) ListEntities(context.Context, store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return func(func(*entity.Entity, error) bool) {}
}
func (stubReader) ListRelations(
	context.Context, store.RelationQuery,
) iter.Seq2[*entity.Relation, error] {
	return func(func(*entity.Relation, error) bool) {}
}

// stubRecorder is a non-nil lua.ElevationRecorder. Like stubReader it is
// never invoked; these tests assert what is handed over.
type stubRecorder struct{}

func (stubRecorder) RecordElevatedRead(context.Context, []string) {}

// TestRun_ElevationRequiresBothKeys is the table that pins the gate. Both
// keys are still required for ANY elevation: the operator opt-in on the
// action, and a Mutator that offers the capability.
//
// Since TKT-Y3JVFK read and write are asserted SEPARATELY, because the
// allow_acl_bypass enum grants them independently — `read` must yield a
// reader and no Mutator, `write` the reverse. What must NOT drift is the
// outer gate: neither capability may appear without both keys.
func TestRun_ElevationRequiresBothKeys(t *testing.T) {
	t.Parallel()

	elevation := ReadElevation{Reader: stubReader{}, Recorder: stubRecorder{}}
	for _, tc := range []struct {
		name        string
		allowBypass metamodel.ACLBypass
		mutator     autocascade.Mutator
		wantWrite   bool
		wantRead    bool
	}{
		{
			name:        "read+write with elevating mutator: both granted",
			allowBypass: metamodel.ACLBypassReadWrite,
			mutator:     elevatingMutator{},
			wantWrite:   true,
			wantRead:    true,
		},
		{
			// The document-render posture: reads elevate, writes do not, so
			// the surface is structurally unable to mutate.
			name:        "read only: reader granted, no elevated mutator",
			allowBypass: metamodel.ACLBypassRead,
			mutator:     elevatingMutator{},
			wantRead:    true,
		},
		{
			name:        "write only: mutator granted, no elevated reader",
			allowBypass: metamodel.ACLBypassWrite,
			mutator:     elevatingMutator{},
			wantWrite:   true,
		},
		{
			// Operator opt-in alone is not enough: a Mutator that does not
			// offer the capability cannot be forced to grant it. This holds
			// for READ elevation too, which is why the ElevatedProvider check
			// still wraps both grants.
			name:        "read+write but mutator declines: denied",
			allowBypass: metamodel.ACLBypassReadWrite,
			mutator:     plainMutator{},
		},
		{
			name:        "read but mutator declines: denied",
			allowBypass: metamodel.ACLBypassRead,
			mutator:     plainMutator{},
		},
		{
			// The mirror image, and the more dangerous direction: a Mutator
			// that CAN elevate must not do so for an ordinary action.
			name:    "mutator can elevate but value unset: denied",
			mutator: elevatingMutator{},
		},
		{
			name:    "neither key: denied",
			mutator: plainMutator{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			exec := &depsCapturingExecutor{}
			r := NewLuaScriptRunnerWithElevatedReads(exec, lua.ReadDeps{}, elevation)

			err := r.Run(context.Background(), autocascade.ScriptAction{
				Code:           "print('x')",
				AllowACLBypass: string(tc.allowBypass),
			}, tc.mutator)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}

			gotWrite := exec.got.ElevatedManager != nil
			gotRead := exec.got.ElevatedReader != nil
			if gotWrite != tc.wantWrite {
				t.Errorf("elevated WRITE granted = %v, want %v", gotWrite, tc.wantWrite)
			}
			if gotRead != tc.wantRead {
				t.Errorf("elevated READ granted = %v, want %v -- read elevation still "+
					"requires BOTH keys (the operator value and an offering Mutator); "+
					"only WHICH capability it grants is now selectable", gotRead, tc.wantRead)
			}
			// The recorder must accompany the reader on every row that grants
			// one. Granting the capability without the audit trace is the exact
			// gap TKT-ACSBSA closes, and it would be invisible in behavior.
			// It rides along on write-only rows too (harmless: with no reader
			// there is nothing to record), so assert against the outer gate.
			wantRec := tc.wantRead || tc.wantWrite
			if gotRec := exec.got.ElevationRecorder != nil; gotRec != wantRec {
				t.Errorf("elevation RECORDER passed = %v, want %v -- the audit sink "+
					"must travel with the elevated reader, or elevated reads happen "+
					"untraced", gotRec, wantRec)
			}
		})
	}
}

// TestRun_NoElevatedReaderWhenNoneSupplied pins that Run hands over only
// what the WIRING SITE gave it. A runner built without a reader must not
// synthesize one from readDeps.
//
// The specific temptation this guarded against is now gone: readDeps used
// to carry an always-present raw WritePrepStore that a future refactor
// could have reached for, granting read elevation at every wiring site
// rather than the ones that opted in. TKT-80EWGM removed that field, so
// there is no raw handle in readDeps left to synthesize from. The
// assertion is kept because the RULE — elevation comes from the wiring
// site, never from ambient deps — outlives the particular field.
func TestRun_NoElevatedReaderWhenNoneSupplied(t *testing.T) {
	t.Parallel()
	exec := &depsCapturingExecutor{}
	// NewLuaScriptRunner is the no-read-elevation constructor.
	r := NewLuaScriptRunner(exec, lua.ReadDeps{})

	err := r.Run(context.Background(), autocascade.ScriptAction{
		Code:           "print('x')",
		AllowACLBypass: string(metamodel.ACLBypassReadWrite),
	}, elevatingMutator{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if exec.got.ElevatedReader != nil {
		t.Error("read elevation was granted by a runner that was never given a " +
			"reader -- the capability must come from the wiring site, not be " +
			"synthesized from the read deps")
	}
	// Write elevation is unaffected: the two are independent capabilities.
	if exec.got.ElevatedManager == nil {
		t.Error("withholding read elevation also disabled WRITE elevation")
	}
}

// TestACLBypassConstantsMatchMetamodel pins the string mirror in autocascade
// to the metamodel constants it duplicates.
//
// autocascade may not import metamodel (it is deliberately schema-agnostic —
// see .go-arch-lint.yml), so it re-declares the three values as plain strings
// with a comment saying they MUST match. This package imports both, so it is
// the one place that can turn that comment into a check.
//
// Without it, a typo in either place silently disables elevation on the
// cascade path: bypass.Enabled() would be false for a value the operator
// legitimately wrote. Fail-closed, but silently, which is the worst shape for
// a capability gate.
func TestACLBypassConstantsMatchMetamodel(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		cascade string
		metaVal metamodel.ACLBypass
	}{
		{"read", autocascade.ACLBypassRead, metamodel.ACLBypassRead},
		{"write", autocascade.ACLBypassWrite, metamodel.ACLBypassWrite},
		{"read+write", autocascade.ACLBypassReadWrite, metamodel.ACLBypassReadWrite},
	} {
		if tc.cascade != string(tc.metaVal) {
			t.Errorf("%s: autocascade has %q, metamodel has %q — the duplicated "+
				"constants have drifted, which silently disables elevation for that value",
				tc.name, tc.cascade, tc.metaVal)
		}
	}
}
