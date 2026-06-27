package dataentry

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// AffordanceProfile names a verdict-source preset. "none" is the
// permissive default; "demo" is a fixture against the ticket type
// exercising every affordance code path. Absent an explicit profile,
// a policy carrying affordance grants selects the policy-backed
// resolver.
//
// The env var RELA_AFFORDANCE_PROFILE selects the override at startup.
// cmd/rela-server and cmd/rela-desktop parse the var and pass the
// resolver into NewApp; tests pass the resolver directly.
type AffordanceProfile string

const (
	AffordanceProfileNone AffordanceProfile = "none"
	AffordanceProfileDemo AffordanceProfile = "demo"
)

// ResolverFromProfile returns the [FieldVerdictResolver] for the given
// profile, policy, and metamodel. Selection order (DR-M3):
//
//  1. profile == "demo" → [DemoFieldVerdictResolver] (hard override,
//     for dev / e2e fixtures even when a policy is present).
//  2. profile == "none" → [NopFieldVerdictResolver] (hard opt-out).
//  3. profile == "" and the policy declares any affordance grants →
//     the policy-backed resolver.
//  4. otherwise → [NopFieldVerdictResolver].
//
// An unknown profile logs a warning and falls back to step 3/4. A
// policy-backed resolver whose predicates fail to compile returns an
// error — the caller fails startup loudly (DR-M4), matching the
// acl.yaml hard-fail posture for genuinely broken config.
func ResolverFromProfile(
	profile string, meta *metamodel.Metamodel, st store.Store,
	declarative *acl.Declarative,
) (FieldVerdictResolver, error) {
	switch AffordanceProfile(profile) {
	case AffordanceProfileDemo:
		return DemoFieldVerdictResolver{}, nil
	case AffordanceProfileNone:
		return NopFieldVerdictResolver{}, nil
	case "":
		// fall through to policy-based selection
	default:
		slog.Warn("dataentry: unknown RELA_AFFORDANCE_PROFILE; using policy or 'none'",
			"value", profile,
			"allowed", []string{string(AffordanceProfileNone), string(AffordanceProfileDemo)})
	}

	// No declarative wired → NopACL or constant-decider ACL → no
	// affordance grants to compute. Declarative is the single source
	// for the policy (RR-WTLD); reading it through any other channel
	// invites mismatched-pair bugs.
	if declarative == nil {
		return NopFieldVerdictResolver{}, nil
	}
	policy := declarative.Policy()
	if policy == nil || !policy.HasAffordanceGrants() {
		return NopFieldVerdictResolver{}, nil
	}
	resolver, err := affordances.New(meta, storeRelationLookup{st: st}, declarative)
	if err != nil {
		return nil, fmt.Errorf("dataentry: compiling acl.yaml affordance predicates: %w", err)
	}
	return &policyResolver{inner: resolver}, nil
}

// ResolverServices is the slice of [appbuild.Services] that
// [ResolverFromServices] needs. Declared here at the call site so the
// entry points pass `svc` directly without each re-spelling the
// env-read + accessors.
//
// ACLDeclarative may return nil when no policy is wired (NopACL); the
// resolver constructor handles that case by selecting NopFieldVerdictResolver.
type ResolverServices interface {
	ACLDeclarative() *acl.Declarative
	Meta() *metamodel.Metamodel
	Store() store.Store
}

// ResolverFromServices builds the affordance resolver for an entry
// point, reading RELA_AFFORDANCE_PROFILE from the environment and the
// metamodel / store / declarative from svc. Both cmd entry points call
// this; they differ only in how they handle the returned error
// (rela-server exits, rela-desktop surfaces it to the UI), so error
// handling stays at the call site.
func ResolverFromServices(svc ResolverServices) (FieldVerdictResolver, error) {
	return ResolverFromProfile(
		os.Getenv("RELA_AFFORDANCE_PROFILE"),
		svc.Meta(), svc.Store(), svc.ACLDeclarative())
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
