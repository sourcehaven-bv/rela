package store

import (
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// ResolutionRule names which of the three world-resolution rules selected an
// entity's prime. It is the ONE vocabulary for that fact: search, worldreader
// and the data-entry wire all alias it rather than mirroring it, so a wire
// value and a log line about the same resolution read alike by construction.
type ResolutionRule int

const (
	// ResolutionUnscoped is rule 1: the world declares no resolution for the
	// entity's type, so it contributes its default state.
	ResolutionUnscoped ResolutionRule = iota
	// ResolutionChain is rule 2: the first chain coordinate that EXISTS.
	ResolutionChain
	// ResolutionFallbackDefault is rule 3 under `otherwise: default`: no
	// chain coordinate exists, so the default state stands in.
	ResolutionFallbackDefault
	// ResolutionExcluded is rule 3 under `otherwise: exclude`: no chain
	// coordinate exists and the entity is absent from this world.
	// [ResolveWorldPrimes] never reports it — an excluded id is simply
	// missing from its result — but readers that answer one id at a time
	// need the word.
	ResolutionExcluded
)

// String names the rule for the wire, logs and test failures.
func (r ResolutionRule) String() string {
	switch r {
	case ResolutionUnscoped:
		return "unscoped"
	case ResolutionChain:
		return "chain"
	case ResolutionFallbackDefault:
		return "fallback-default"
	case ResolutionExcluded:
		return "excluded"
	default:
		return fmt.Sprintf("ResolutionRule(%d)", int(r))
	}
}

// ParseResolutionRule is the inverse of [ResolutionRule.String] for the
// wire vocabulary. An unknown name is [ResolutionUnscoped], the rule that
// applies when no resolution was recorded.
func ParseResolutionRule(name string) ResolutionRule {
	switch name {
	case ResolutionChain.String():
		return ResolutionChain
	case ResolutionFallbackDefault.String():
		return ResolutionFallbackDefault
	case ResolutionExcluded.String():
		return ResolutionExcluded
	default:
		return ResolutionUnscoped
	}
}

// WorldCandidate is one stored state row offered to [ResolveWorldPrimes]:
// the bare id, its type, and its face. Backends and indexes pass their own
// metadata rather than a loaded entity, so resolution costs no I/O.
type WorldCandidate struct {
	ID   string
	Type string
	Face entity.Face
}

// WorldResolved is the verdict for one entity: which face the world serves,
// and the provenance a caller must be able to label it with. "The published
// face" and "the default face, because no published face exists" are
// different facts about the same bytes, and only Via and ChainPosition tell
// them apart.
type WorldResolved struct {
	Face entity.Face
	Via  ResolutionRule
	// ChainPosition is the 0-based index of Face in the type's chain.
	// Meaningful only when Via is [ResolutionChain]; position 0 is the
	// world's first choice, anything later is a stand-in.
	ChainPosition int
}

// ResolveWorldPrimes selects, per bare id, the single state row that world w
// resolves to — the "prime" (design doc §4.1) — with its provenance. An id
// absent from the result contributes NOTHING to this world.
//
// This is THE implementation of the three rules for every Go caller:
// [storeutil.WorldPrimes] (fs, mem, sqlite listings), the search resolvers,
// and the data-entry provenance all delegate here. The only other copy is
// the SQL form in pgstore, which cannot share Go and is pinned to this one by
// the storetest conformance suite.
//
// The three rules:
//
//   - rule 1: the type has no resolution in w (absent from the scope) — the
//     DEFAULT state, matching the pre-worlds behavior. Absence is NOT the zero
//     TypeResolution, which excludes; see [WorldScope].
//   - rule 2: the first coordinate in the chain that EXISTS for this id.
//   - rule 3: no chain coordinate exists — the chain's Fallback decides,
//     either the default state or nothing at all.
//
// Candidates must be the rows of WHOLE families: the fallback verdict is a
// decision about ABSENCE, so an id whose candidates are only partly present
// can resolve to the wrong prime rather than merely a stale one. Decisions
// are made AFTER the whole candidate set is seen, never streaming per row.
func ResolveWorldPrimes(w WorldScope, candidates []WorldCandidate) map[string]WorldResolved {
	if len(candidates) == 0 {
		return nil
	}

	type family struct {
		typ         string
		bestRank    int
		bestFace    entity.Face
		haveBest    bool
		haveDefault bool
	}
	// The family's type comes from whichever row arrives first, and the
	// default row's type wins when present: every backend rejects a state
	// whose type differs from its family's on write, so a family is
	// single-typed and the choice is arbitrary-but-equal — except for a
	// hand-edited fsstore tree, where the answer must not depend on the
	// order candidates arrived in.
	fams := make(map[string]*family, len(candidates))
	for _, c := range candidates {
		f, ok := fams[c.ID]
		if !ok {
			f = &family{typ: c.Type}
			fams[c.ID] = f
		}
		if c.Face.IsDefault() {
			f.haveDefault = true
			f.typ = c.Type
		}
		res, scoped := w.For(c.Type)
		if !scoped {
			continue // rule 1: decided below from haveDefault
		}
		if rank := slices.Index(res.Chain, c.Face); rank >= 0 {
			if !f.haveBest || rank < f.bestRank {
				f.bestRank, f.bestFace, f.haveBest = rank, c.Face, true
			}
		}
	}

	out := make(map[string]WorldResolved, len(fams))
	for id, f := range fams {
		res, scoped := w.For(f.typ)
		switch {
		case !scoped:
			if f.haveDefault {
				out[id] = WorldResolved{Via: ResolutionUnscoped}
			}
		case f.haveBest:
			out[id] = WorldResolved{Face: f.bestFace, Via: ResolutionChain, ChainPosition: f.bestRank}
		case res.Fallback == FallbackDefaultState && f.haveDefault:
			out[id] = WorldResolved{Via: ResolutionFallbackDefault}
		default:
			// `otherwise: exclude` — absence IS the publication bit.
		}
	}
	return out
}

// ResolutionAt names the rule that produced a face stored at p for an entity
// of entityType under scope, and its chain position when rule 2 applied. It
// answers for a face ALREADY chosen (by a backend's ranking, say), where the
// family is not in hand; it is a total mapping, not a walk.
//
// entityType must be canonical: [WorldScope] is keyed on canonical names
// only, and an alias reads as an unknown type, which is rule 1.
func ResolutionAt(scope WorldScope, entityType string, p entity.Face) (rule ResolutionRule, position int) {
	res, scoped := scope.For(entityType)
	if !scoped {
		return ResolutionUnscoped, 0
	}
	if i := slices.Index(res.Chain, p); i >= 0 {
		return ResolutionChain, i
	}
	return ResolutionFallbackDefault, 0
}
