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
// A world resolves the same prime for every reader OVER THE SAME
// CANDIDATES. Nothing in this package takes a principal, an ACL gate, or an
// [internal/acl] type, and that is a structural guarantee rather than a
// convention — see `guard_test.go`, which scans the package and fails on
// such a dependency.
//
// What a principal DOES change is the candidate set. The contract for the
// whole read path is filter-first: the ACL trims the graph to the faces the
// reader may see (a `type@face` grant compiles to [store.EntityQuery.FaceIn]),
// and the world then ranks what is left. A reader granted only
// `policy@published` under `select: [review, published]` is served the
// published face — the world is a view onto the part of the graph that is
// visible to them, not onto the whole graph. The single-entity path and the
// list path both run that one query, so they cannot disagree.
//
// The resolver itself still never consults a gate: it cannot, and that is
// what keeps "same candidates, same prime" true.
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
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

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

	// Face is the coordinate Entity was stored at. The zero face
	// means the default state, whether it was reached by rule 1, by an
	// explicit chain coordinate, or by the fallback — Via distinguishes
	// those.
	Face entity.Face

	// Via records WHICH resolution rule produced this prime.
	Via Rule

	// Found is false when the world excludes this entity. The entity may
	// exist in storage; in this world it does not.
	Found bool
}

// Rule is [store.ResolutionRule]: the resolution rule that selected a prime.
// One vocabulary for the whole tree, aliased rather than mirrored.
type Rule = store.ResolutionRule

const (
	// RuleUnscoped is rule 1: the world declares no resolution for this
	// entity's type, so it contributes its default state.
	RuleUnscoped = store.ResolutionUnscoped
	// RuleChain is rule 2: the first chain coordinate that EXISTS.
	RuleChain = store.ResolutionChain
	// RuleFallbackDefault is rule 3 under `otherwise: default`.
	RuleFallbackDefault = store.ResolutionFallbackDefault
	// RuleExcluded is rule 3 under `otherwise: exclude`: the entity is
	// absent from this world.
	RuleExcluded = store.ResolutionExcluded
)
