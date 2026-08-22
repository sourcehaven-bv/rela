// Package worldreader is the RUNTIME half of content-state worlds
// (TKT-WAV8XP Step 2): it resolves an entity to the single state a world
// selects — its "prime" — and hands out capabilities that carry that
// resolution so callers cannot accidentally read around it.
//
// It is deliberately separate from [internal/worlds], which is the pure
// COMPILER (metamodel → [store.WorldScope]) and holds no store. This
// package holds a store and does the reading.
//
// # Guard rule 1: resolution is PRINCIPAL-INDEPENDENT
//
// A world resolves the same prime for every reader. Nothing in this
// package takes a principal, an ACL gate, or an [internal/acl] type, and
// that is a structural guarantee rather than a convention — see
// `guard_test.go`, which scans the package and fails on such a
// dependency.
//
// The reason is an existence oracle. Resolution must complete BEFORE the
// ACL gate is consulted: if the gate ran first, a prime the principal
// may not read would fall through to the chain's next candidate, and the
// face a reader receives would then reveal what the ACL denied them. So
// the order is fixed — resolve, then gate — and the only way to keep it
// is that the resolver cannot consult a gate at all.
//
// Consequently a denied entity yields NOTHING, never a different face.
//
// # Two mechanisms, one contract
//
// Resolution reaches the store two ways, and both are required:
//
//   - as a DECORATOR, for the read paths that go through a reader; and
//   - as a FIELD ON THE QUERY ([store.EntityQuery.World] /
//     [store.GraphQuery.World]), because `internal/visibility`'s pushdown
//     composes a GraphQuery and hands it straight to the raw store,
//     reaching past every decorator.
//
// A decorator-only design would silently degrade to the default world on
// the pushdown path — which is precisely the ACL-path fail-open found in
// PR-B review (RR-GQWRLD). `parity_test.go` pins that the two paths agree,
// including for a principal carrying a policy query.
package worldreader

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// StateReader is the narrow read surface the resolver needs: addressing
// one stored state directly. Declared here, at the call site, rather than
// taken from the store package wholesale.
type StateReader interface {
	GetEntityState(ctx context.Context, id string, p entity.Pointer) (*entity.Entity, error)
}

// Resolved is a prime plus the PROVENANCE of how it was chosen.
//
// The provenance is not decoration: "the published face" and "the default
// face, because no published state exists" are different facts about the
// same bytes, and a caller that renders a publication badge, logs an
// audit line, or decides whether to offer an edit affordance needs to
// tell them apart. Returning only the entity would force every such
// caller to re-derive the verdict, and re-deriving it requires the chain
// — which is exactly what this package exists to own.
type Resolved struct {
	// Entity is the resolved state. Never nil when Found is true.
	Entity *entity.Entity

	// Pointer is the coordinate Entity was stored at. The zero pointer
	// means the default state, whether it was reached by rule 1, by an
	// explicit chain coordinate, or by the fallback — Via distinguishes
	// those.
	Pointer entity.Pointer

	// Via records WHICH resolution rule produced this prime.
	Via Rule

	// Found is false when the world excludes this entity. The entity may
	// exist in storage; in this world it does not.
	Found bool
}

// Rule names the resolution rule that selected a prime, matching the
// three rules in the design doc and [storeutil.WorldPrimes].
type Rule int

const (
	// RuleUnscoped is rule 1: the world declares no resolution for this
	// entity's type, so it contributes its default state. This is the
	// common case in a mixed graph and costs no chain walk.
	RuleUnscoped Rule = iota
	// RuleChain is rule 2: the first chain coordinate that EXISTS.
	RuleChain
	// RuleFallbackDefault is rule 3 under `otherwise: default`: no chain
	// coordinate exists, so the default state stands in.
	RuleFallbackDefault
	// RuleExcluded is rule 3 under `otherwise: exclude`: no chain
	// coordinate exists and the entity is absent from this world.
	RuleExcluded
)

// String names the rule for logs and test failures.
func (r Rule) String() string {
	switch r {
	case RuleUnscoped:
		return "unscoped"
	case RuleChain:
		return "chain"
	case RuleFallbackDefault:
		return "fallback-default"
	case RuleExcluded:
		return "excluded"
	default:
		return fmt.Sprintf("Rule(%d)", int(r))
	}
}

// TypeCanonicalizer maps a possibly-aliased entity type name to its
// canonical name. [store.WorldScope] is keyed on CANONICAL names only —
// a store holds no metamodel and cannot resolve an alias — so a caller
// that took a type name from user input or from a stored row must
// canonicalize before the scope is consulted.
//
// Skipping this is FAIL-OPEN, not fail-closed: an alias reaches
// [store.WorldScope.For] as an unknown type, which is ok=false, which is
// rule 1 — the default state served in a world that meant to exclude it.
// That is why canonicalization happens at this boundary rather than being
// left to callers.
type TypeCanonicalizer interface {
	CanonicalType(name string) (string, bool)
}

// Resolver resolves entities to their prime under one fixed world.
//
// The world is bound at CONSTRUCTION and there is no per-call world
// parameter (Q10). A surface is built over its world; it cannot be asked
// to serve a different one. That is the DEC-ZBI39P stance — structurally
// incapable rather than defaulting to safe — and it is why request-level
// world selection is a separate ticket with its own grant check rather
// than a parameter plumbed through now.
type Resolver struct {
	reader StateReader
	scope  store.WorldScope
	canon  TypeCanonicalizer
}

// NewResolver builds a Resolver over a fixed world.
//
// Required collaborators are rejected when nil rather than silently
// substituted, per the project constructor rule: a no-op canonicalizer
// would turn every aliased type into rule 1, which is the fail-open
// direction described on [TypeCanonicalizer].
func NewResolver(reader StateReader, scope store.WorldScope, canon TypeCanonicalizer) (*Resolver, error) {
	if reader == nil {
		return nil, errors.New("worldreader: NewResolver: reader must be non-nil")
	}
	if canon == nil {
		return nil, errors.New("worldreader: NewResolver: canon must be non-nil")
	}
	return &Resolver{reader: reader, scope: scope, canon: canon}, nil
}

// Scope returns the world this resolver is bound to, for the wiring site
// to copy onto a query ([store.EntityQuery.World]) on the pushdown path.
func (r *Resolver) Scope() store.WorldScope { return r.scope }

// Resolve walks the chain for one entity and returns its prime.
//
// entityType may be an alias; it is canonicalized here (see
// [TypeCanonicalizer]).
//
// The walk is at most len(chain)+1 point reads, and chains are 1-3 in
// practice — the common case is a single call. It reads one coordinate at
// a time rather than listing the family because a point read is what
// every backend is fastest at, and because the chain short-circuits: the
// first hit wins and the rest are never fetched.
//
// A missing state is not an error. Only a real store fault is, and it is
// returned rather than swallowed — the caller (the visibility Reader)
// owns the decision to collapse faults into a clean miss, and duplicating
// that policy here would make a backend outage indistinguishable from an
// exclusion at the wrong layer.
func (r *Resolver) Resolve(ctx context.Context, entityType, id string) (Resolved, error) {
	typ := entityType
	if canonical, ok := r.canon.CanonicalType(entityType); ok {
		typ = canonical
	}

	res, scoped := r.scope.For(typ)
	if !scoped {
		// Rule 1: unscoped type contributes its default state.
		e, found, err := r.getState(ctx, id, "")
		if err != nil {
			return Resolved{}, err
		}
		if !found {
			return Resolved{Via: RuleUnscoped}, nil
		}
		return Resolved{Entity: e, Via: RuleUnscoped, Found: true}, nil
	}

	// Rule 2: the first coordinate that EXISTS wins. Chain order is the
	// entire semantic content of a world.
	for _, coord := range res.Chain {
		e, found, err := r.getState(ctx, id, coord)
		if err != nil {
			return Resolved{}, err
		}
		if found {
			return Resolved{Entity: e, Pointer: coord, Via: RuleChain, Found: true}, nil
		}
	}

	// Rule 3: no coordinate exists; the fallback decides.
	if res.Fallback != store.FallbackDefaultState {
		// `otherwise: exclude` — absence IS the publication bit.
		return Resolved{Via: RuleExcluded}, nil
	}
	e, found, err := r.getState(ctx, id, "")
	if err != nil {
		return Resolved{}, err
	}
	if !found {
		return Resolved{Via: RuleExcluded}, nil
	}
	return Resolved{Entity: e, Via: RuleFallbackDefault, Found: true}, nil
}

// getState reads one coordinate. The bool distinguishes ABSENCE (a
// coordinate the chain may skip) from a FAULT (which must surface) —
// returning a nil entity with a nil error would conflate them at the
// call site, which is the one distinction this walk turns on.
func (r *Resolver) getState(
	ctx context.Context, id string, p entity.Pointer,
) (e *entity.Entity, found bool, err error) {
	e, err = r.reader.GetEntityState(ctx, id, p)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return e, true, nil
}
