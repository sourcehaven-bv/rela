package worldreader

import (
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// WorldAwareSearcher is the capability a searcher must advertise before
// it may back a world-bound surface: it declares which worlds it can
// actually serve.
//
// No searcher implements this today, and that is the point — see
// [NewSurface].
type WorldAwareSearcher interface {
	// ServesWorld reports whether this searcher's results are scoped to
	// the given world.
	ServesWorld(w store.WorldScope) bool
}

// Surface is a world-bound read surface: a resolver, its relation
// capability, and (optionally) a searcher, all fixed to ONE world at
// construction.
//
// There is no world parameter on any method, and no setter. A surface IS
// its world (Q10). Request-level world selection is deliberately not
// shipped here: it needs its own grant check, and a plumbed-but-ungated
// world parameter would be exactly the half-built thing this arc declined
// to ship.
type Surface struct {
	resolver  *Resolver
	relations *RelationReader
	searcher  WorldAwareSearcher
}

// ErrSearcherCannotServeWorld is returned when a world-bound surface is
// constructed over a searcher that cannot honor its world.
var ErrSearcherCannotServeWorld = errors.New(
	"worldreader: searcher cannot serve this surface's world")

// NewSurface binds a resolver, its relation reader, and a searcher into
// one world-scoped surface.
//
// # The searcher-constructibility criterion (RULING 3)
//
// A world-bound surface must not be CONSTRUCTIBLE with a searcher that
// cannot honor its world. This is the DEC-ZBI39P stance — structurally
// incapable rather than "defaults to safe" — and it is enforced here, at
// construction, rather than by a runtime check on the search path.
//
// The reason it cannot be a runtime check: `internal/search` is
// world-agnostic BY CONSTRUCTION. Its Query type carries no world and its
// Searcher interface has no world parameter, so there is nothing on the
// search path to test. A refusal there would be a concept introduced
// solely to reject itself. See that package's doc.
//
// So the check lives where the world actually exists — here — and it is
// deliberately strict:
//
//   - DEFAULT world: any searcher is fine. Every searcher serves the
//     default world today, so a pointerless project is unaffected.
//   - NON-DEFAULT world with NO searcher: allowed. A surface without
//     search cannot return wrongly-scoped hits.
//   - NON-DEFAULT world WITH a searcher: the searcher must implement
//     [WorldAwareSearcher] and affirm it serves this world. Nothing does
//     today, so this REFUSES — which is the intended outcome. Per-world
//     indexing is Step 5 (TKT-9KZGJO); until it lands, a world-bound
//     surface with a working-but-wrong search box must be impossible to
//     build rather than merely discouraged.
//
// The failure mode this closes: the ACL row gate CANNOT catch a
// wrongly-scoped search hit, because guard rule 1 makes the row gate
// world-independent. A draft surfacing through search would reach the
// user with nothing downstream to stop it.
func NewSurface(
	resolver *Resolver, relations *RelationReader, searcher WorldAwareSearcher,
) (*Surface, error) {
	if resolver == nil {
		return nil, errors.New("worldreader: NewSurface: resolver must be non-nil")
	}
	if relations == nil {
		return nil, errors.New("worldreader: NewSurface: relations must be non-nil")
	}

	if !resolver.Scope().IsDefaultWorld() && searcher != nil {
		if !searcher.ServesWorld(resolver.Scope()) {
			return nil, fmt.Errorf(
				"%w: build the surface without a searcher, or wait for per-world "+
					"indexing (TKT-9KZGJO) — a world-bound surface must not serve "+
					"default-world hits, and the ACL row gate cannot catch them "+
					"because it is world-independent by design (guard rule 1)",
				ErrSearcherCannotServeWorld)
		}
	}

	return &Surface{resolver: resolver, relations: relations, searcher: searcher}, nil
}

// Resolver returns the surface's resolver. The world is already bound.
func (s *Surface) Resolver() *Resolver { return s.resolver }

// Relations returns the world-scoped relation capability. Callers get
// this rather than a [store.RelationQuery], so the scope dispatch cannot
// be omitted.
func (s *Surface) Relations() *RelationReader { return s.relations }

// Searcher returns the surface's searcher, which may be nil. It is
// guaranteed to serve this surface's world — NewSurface refuses any
// other combination.
func (s *Surface) Searcher() WorldAwareSearcher { return s.searcher }
