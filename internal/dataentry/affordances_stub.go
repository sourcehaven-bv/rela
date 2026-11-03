package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/relresolve"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
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
// resolver into NewApp; tests pass the resolver directly. Neither
// profile may be set when acl.yaml declares field rules (BUG-UO72G2):
// see [ResolverFromProfile].
type AffordanceProfile string

const (
	AffordanceProfileNone AffordanceProfile = "none"
	AffordanceProfileDemo AffordanceProfile = "demo"
)

// ErrProfileOverridesFieldRules is returned by [ResolverFromProfile] when
// RELA_AFFORDANCE_PROFILE is "none" or "demo" and acl.yaml declares field
// rules.
var ErrProfileOverridesFieldRules = errors.New("acl.yaml declares field rules")

// ResolverFromProfile returns the [FieldVerdictResolver] for the given
// profile, policy, and metamodel. Selection order (DR-M3, amended by
// BUG-UO72G2):
//
//  1. profile == "demo" or "none" and the policy declares any affordance
//     grants ([acl.Policy.HasAffordanceGrants]: role fields:, visible:,
//     options: or relations:, or a client_baselines field ceiling) →
//     an error wrapping [ErrProfileOverridesFieldRules]. The profile only
//     replaces the REST resolver; MCP and CLI reads still apply the
//     policy, so applying it would drop the field rules on REST alone.
//  2. profile == "demo" → [DemoFieldVerdictResolver] (dev / e2e fixture).
//  3. profile == "none" → [NopFieldVerdictResolver] (opt-out).
//  4. profile == "" and the policy declares any affordance grants →
//     the policy-backed resolver.
//  5. otherwise → [NopFieldVerdictResolver].
//
// An unknown profile logs a warning and falls back to step 4/5. A
// policy-backed resolver whose predicates fail to compile returns an
// error. In both error cases the caller fails startup loudly (DR-M4),
// matching the acl.yaml hard-fail posture for genuinely broken config.
func ResolverFromProfile(
	profile string, meta *metamodel.Metamodel, st store.Store,
	declarative *acl.Declarative,
) (FieldVerdictResolver, error) {
	// No declarative wired → NopACL or constant-decider ACL → no
	// affordance grants to compute. Declarative is the single source
	// for the policy (RR-WTLD); reading it through any other channel
	// invites mismatched-pair bugs.
	hasGrants := false
	if declarative != nil {
		if policy := declarative.Policy(); policy != nil {
			hasGrants = policy.HasAffordanceGrants()
		}
	}

	switch AffordanceProfile(profile) {
	case AffordanceProfileDemo, AffordanceProfileNone:
		if hasGrants {
			return nil, fmt.Errorf(
				"dataentry: %w: RELA_AFFORDANCE_PROFILE=%s would drop them on REST "+
					"while MCP and CLI still apply them; unset RELA_AFFORDANCE_PROFILE",
				ErrProfileOverridesFieldRules, profile)
		}
		if AffordanceProfile(profile) == AffordanceProfileDemo {
			return DemoFieldVerdictResolver{}, nil
		}
		return NopFieldVerdictResolver{}, nil
	case "":
		// fall through to policy-based selection
	default:
		slog.Warn("dataentry: unknown RELA_AFFORDANCE_PROFILE; using policy or 'none'",
			"value", profile,
			"allowed", []string{string(AffordanceProfileNone), string(AffordanceProfileDemo)})
	}

	if !hasGrants {
		return NopFieldVerdictResolver{}, nil
	}
	// One raw binder answers related() in both ACL `when:` grants and
	// transition `when:` (TKT-205V2N): both are authorization decisions over
	// the graph as it is.
	traversals, err := relresolve.NewStoreBinder(meta, relresolve.Ungated, st)
	if err != nil {
		return nil, fmt.Errorf("dataentry: acl traversals: %w", err)
	}
	resolver, err := affordances.New(meta, storeRelationLookup{st: st}, declarative,
		affordances.WithTraversals(traversals))
	if err != nil {
		return nil, fmt.Errorf("dataentry: compiling acl.yaml affordance predicates: %w", err)
	}
	// Wire the enum state machines so the resolver can answer
	// TransitionVerdicts (TKT-FT8J9). Compiled from the same (load-once,
	// read-only) metamodel the entitymanager compiled its enforcer from:
	// Compile is a deterministic pure transform AND meta is immutable between
	// the two calls, so the two Sets are semantically identical. Correctness
	// does not actually depend on that identity, though — the read/write
	// drift guard is the shared evalEdge, so two Sets from the same source
	// cannot disagree regardless. (Threading the entitymanager's single Set
	// through appbuild.Services is the follow-up when the SPA status control
	// wires the whole surface.)
	machines, err := statemachine.Compile(meta, statemachine.WithTraversals(traversals))
	if err != nil {
		return nil, fmt.Errorf("dataentry: compiling state machines: %w", err)
	}
	resolver.WithMachines(machines)
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

// UnconditionalRelationVerdicts always returns the zero value.
func (NopFieldVerdictResolver) UnconditionalRelationVerdicts(context.Context, *entityPkg.Entity) RelationVerdicts {
	return RelationVerdicts{}
}

// FieldVisibleOnEveryRow is always true: this resolver hides nothing.
func (NopFieldVerdictResolver) FieldVisibleOnEveryRow(context.Context, string, string) bool {
	return true
}

// SearchFieldScope is nil: this resolver hides nothing.
func (NopFieldVerdictResolver) SearchFieldScope(context.Context, string) (*search.FieldScope, error) {
	return nil, nil //nolint:nilnil // nil scope is the documented "nothing hidden"
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

// UnconditionalRelationVerdicts is RelationVerdicts: the fixture depends on
// the type only.
func (d DemoFieldVerdictResolver) UnconditionalRelationVerdicts(
	ctx context.Context, e *entityPkg.Entity,
) RelationVerdicts {
	return d.RelationVerdicts(ctx, e)
}

// FieldVisibleOnEveryRow is false for the one field the fixture hides,
// ticket.priority, whose verdict depends on the type only.
func (DemoFieldVerdictResolver) FieldVisibleOnEveryRow(_ context.Context, entityType, property string) bool {
	return entityType != "ticket" || property != "priority"
}

// SearchFieldScope hides priority on every ticket, as FieldVerdicts does.
func (DemoFieldVerdictResolver) SearchFieldScope(_ context.Context, entityType string) (*search.FieldScope, error) {
	if entityType != "ticket" {
		return nil, nil //nolint:nilnil // nil scope is the documented "nothing hidden"
	}
	return &search.FieldScope{Gated: []string{"priority"}}, nil
}
