package appbuild

import (
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/elementnaive"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// Elements builds the gated element range read that POST /api/v1/_reconcile
// serves (TKT-VP02WHG, FEAT-CMVZQFI): the backend's range reader under the
// caller's row gate, field policy and roll-up names.
//
// The row gate is the policy's [visibility.DeclarativeGate] in the default
// world, which checks on every call that the request edge bound an
// acl.Request; without a policy every row is readable
// ([visibility.AllowAllReadQueries]), as on every other read. The field
// policy is the one the gated readers redact with, so a view hash covers
// exactly the values the caller would read.
//
// Every failure is returned. A deployment that cannot build it must not
// serve the endpoint, rather than serve it ungated.
func Elements(s *Services) (*visibility.Elements, error) {
	if s == nil {
		return nil, errors.New("appbuild: Elements: services are required")
	}
	ranges, err := elementRangesFor(s.store)
	if err != nil {
		return nil, fmt.Errorf("appbuild: element ranges: %w", err)
	}
	var reads visibility.ElementReads = visibility.AllowAllReadQueries{}
	if s.aclDeclarative != nil {
		gate, gErr := visibility.NewDeclarativeGate(s.aclDeclarative, s.worlds.DefaultWorld())
		if gErr != nil {
			return nil, fmt.Errorf("appbuild: element ranges: %w", gErr)
		}
		reads = gate
	}
	var policy visibility.ElementPolicy = visibility.NopRedactor{}
	if s.fieldReads != nil {
		policy = s.fieldReads
	}
	rollups := gatedRollups(s)
	if rollups == nil {
		// gatedRollups logs why. Without the namer a roll-up value stored
		// on a row would be hashed into the view; refuse instead.
		return nil, errors.New("appbuild: element ranges: roll-ups are unavailable")
	}
	return visibility.NewElements(visibility.ElementDeps{
		Ranges: ranges, Rows: s.store, Reads: reads, Policy: policy, Rollups: rollups,
	})
}

// naiveRanges returns the reference element range reader over st, as an
// interface that is nil exactly when err is set.
func naiveRanges(st store.Store) (visibility.ElementRangeReader, error) {
	r, err := elementnaive.New(st)
	if err != nil {
		return nil, err
	}
	return r, nil
}
