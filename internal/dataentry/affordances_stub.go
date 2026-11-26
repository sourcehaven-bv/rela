package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/flowcfg/transitions"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/relresolve"
	"github.com/Sourcehaven-BV/rela/internal/search"
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

// ResolverFromProfile returns the [FieldVerdictResolver] for profile, policy
// and metamodel:
//
//  1. "demo" or "none" with affordance grants or a ceiling → an error wrapping
//     [ErrProfileOverridesFieldRules], since MCP and CLI would still apply them.
//  2. "demo" → [DemoFieldVerdictResolver]; "none" → [NopFieldVerdictResolver].
//  3. "" with grants or a ceiling → the policy-backed resolver.
//  4. otherwise → [NopFieldVerdictResolver].
//
// An unknown profile warns and falls back to 3/4. ceiling is the flow edit
// ceiling or nil; it needs a declarative. Compile errors fail startup.
func ResolverFromProfile(
	profile string, meta *metamodel.Metamodel, st store.Store,
	declarative *acl.Declarative, ceiling affordances.FlowEditCeiling,
) (FieldVerdictResolver, error) {
	hasGrants := false
	if declarative != nil {
		if policy := declarative.Policy(); policy != nil {
			hasGrants = policy.HasAffordanceGrants()
		}
	}
	if ceiling != nil {
		if declarative == nil {
			return nil, errors.New("dataentry: flows.yaml outside_edits needs the acl.yaml policy, " +
				"and none is wired")
		}
		hasGrants = true
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
	default:
		slog.Warn("dataentry: unknown RELA_AFFORDANCE_PROFILE; using policy or 'none'",
			"value", profile,
			"allowed", []string{string(AffordanceProfileNone), string(AffordanceProfileDemo)})
	}

	if !hasGrants {
		return NopFieldVerdictResolver{}, nil
	}
	traversals, err := relresolve.NewStoreBinder(meta, relresolve.Ungated, st)
	if err != nil {
		return nil, fmt.Errorf("dataentry: acl traversals: %w", err)
	}
	opts := []affordances.Option{affordances.WithTraversals(traversals)}
	if ceiling != nil {
		opts = append(opts, affordances.WithFlowEdits(ceiling))
	}
	resolver, err := affordances.New(meta, storeRelationLookup{st: st}, declarative, opts...)
	if err != nil {
		return nil, fmt.Errorf("dataentry: compiling acl.yaml affordance predicates: %w", err)
	}
	machines, err := transitions.CompileTransitions(meta, transitions.WithTransitionTraversals(traversals))
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
// handling stays at the call site. ceiling is appbuild.FlowEdits(svc), passed
// beside svc because Services is at its plimsoll cap.
func ResolverFromServices(svc ResolverServices, ceiling affordances.FlowEditCeiling) (FieldVerdictResolver, error) {
	return ResolverFromProfile(
		os.Getenv("RELA_AFFORDANCE_PROFILE"),
		svc.Meta(), svc.Store(), svc.ACLDeclarative(), ceiling)
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
