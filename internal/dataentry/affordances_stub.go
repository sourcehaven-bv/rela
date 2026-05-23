package dataentry

import (
	"context"
	"log/slog"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
)

// AffordanceProfile names a verdict-source preset. v1 ships two:
// "none" (permissive default) and "demo" (a fixture against the
// ticket type that exercises every affordance code path).
//
// The env var RELA_AFFORDANCE_PROFILE selects between them at
// startup. cmd/rela-server and cmd/rela-desktop parse the var and
// pass the resolver into NewApp; tests pass the resolver directly.
type AffordanceProfile string

const (
	AffordanceProfileNone AffordanceProfile = "none"
	AffordanceProfileDemo AffordanceProfile = "demo"
)

// ResolverFromProfile returns the [FieldVerdictResolver] for the named
// profile. An empty string or "none" yields [NopFieldVerdictResolver].
// An unknown profile name logs a warning and falls back to none —
// never panics, never errors. Operators see the warning at startup
// and can correct the env var.
func ResolverFromProfile(profile string) FieldVerdictResolver {
	switch AffordanceProfile(profile) {
	case "", AffordanceProfileNone:
		return NopFieldVerdictResolver{}
	case AffordanceProfileDemo:
		return DemoFieldVerdictResolver{}
	default:
		slog.Warn("dataentry: unknown RELA_AFFORDANCE_PROFILE; using 'none'",
			"value", profile,
			"allowed", []string{string(AffordanceProfileNone), string(AffordanceProfileDemo)})
		return NopFieldVerdictResolver{}
	}
}

// NopFieldVerdictResolver returns empty verdicts for every entity.
// computeFields / computeRelations interpret empty verdicts as "no
// deviations from default" and emit sparse `_fields: {}` and
// `_relations: {}` on the wire. The SPA renders unchanged.
type NopFieldVerdictResolver struct{}

// FieldVerdicts always returns the zero value.
func (NopFieldVerdictResolver) FieldVerdicts(context.Context, *entityPkg.Entity) FieldVerdicts {
	return FieldVerdicts{}
}

// RelationVerdicts always returns the zero value.
func (NopFieldVerdictResolver) RelationVerdicts(context.Context, *entityPkg.Entity) RelationVerdicts {
	return RelationVerdicts{}
}

// DemoFieldVerdictResolver applies a fixed fixture against the
// "ticket" entity type. The fixture is hand-picked to exercise every
// affordance code path:
//
//   - kind: read-only (writable=false)
//   - priority: hidden (visible=false)
//   - effort: option-filtered ({l: false, xl: false})
//   - status: option-filtered ({done: false})
//   - affects relation: not creatable
//   - implements relation: not removable
//   - has-planning relation: meta-field "note" not writable (the
//     metamodel doesn't currently declare relation-meta on this type,
//     so the verdict is still emitted and the contract tests rely on
//     a test-fixture metamodel that adds the meta field)
//
// Other entity types receive empty verdicts. Intended for dev /
// manual-testing use only — the predicate ticket replaces this with
// a policy-driven resolver.
type DemoFieldVerdictResolver struct{}

// FieldVerdicts returns the demo fixture for ticket entities and the
// zero value for every other type.
func (DemoFieldVerdictResolver) FieldVerdicts(_ context.Context, e *entityPkg.Entity) FieldVerdicts {
	if e == nil || e.Type != "ticket" {
		return FieldVerdicts{}
	}
	return FieldVerdicts{
		Writable: map[string]bool{
			"kind": false,
		},
		Visible: map[string]bool{
			"priority": false,
		},
		Options: map[string]map[string]bool{
			"effort": {
				"l":  false,
				"xl": false,
			},
			"status": {
				"done": false,
			},
		},
	}
}

// RelationVerdicts returns the demo relation fixture for ticket
// entities and the zero value for every other type.
func (DemoFieldVerdictResolver) RelationVerdicts(_ context.Context, e *entityPkg.Entity) RelationVerdicts {
	if e == nil || e.Type != "ticket" {
		return RelationVerdicts{}
	}
	return RelationVerdicts{
		Types: map[string]RelationVerdict{
			"affects": {
				Creatable: false,
				Removable: true,
			},
			"implements": {
				Creatable: true,
				Removable: false,
			},
			"has-planning": {
				Creatable: true,
				Removable: true,
				Fields: map[string]bool{
					"note": false,
				},
			},
		},
	}
}
