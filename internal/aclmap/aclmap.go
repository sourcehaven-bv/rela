// Package aclmap materializes slices of the effective-access relation —
// who can perform a verb on an entity, and by which route — from the
// declarative ACL policy plus the graph. It is the read-only reporting
// side of authorization: it answers questions the per-request resolver
// (internal/acl) can only answer one principal at a time, by enumerating
// the principal universe and asking the resolver about each.
//
// This package holds the UC3 slice — WhoCan on a single entity (see
// FEAT-RCQ6SJ / TKT-9089I6). The per-principal map, drift, and
// conformance modes are separate follow-ups that will share this
// enumeration.
//
// Provenance depth is TERMINAL FACTS in this slice: each route names the
// kind plus the group / ancestor / relation by name (enough to act on —
// e.g. delete the named edge to revoke), not the full hop-by-hop chain.
// The route JSON is shaped as a list of route objects so the later
// full-chain work can add a hops field without a breaking change.
package aclmap

import (
	"context"
	"errors"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// EntitySource is the narrow read access the engine needs: fetch one
// entity (existence gate + type lookup), list entities of a type
// (resolvable user principals), and list relations of a type (to find
// role-relation edge sources — principals granted a role by a graph
// edge). Declared here, at the consumer, rather than depending on the
// full store.Store; *store.Store satisfies it.
type EntitySource interface {
	GetEntity(ctx context.Context, id string) (*entity.Entity, error)
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
	ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
}

// Resolver is the per-principal ACL access the engine needs. Satisfied
// by *acl.Declarative. Declared consumer-side so the engine binds only
// to the calls it makes.
type Resolver interface {
	ForPrincipal(p principal.Principal) (*acl.Request, error)
	ResolvePrincipal(ctx context.Context, rawUser string) (string, error)
	EveryoneGrants(verb acl.Verb, entityType string) acl.EveryoneGrant
	Policy() *acl.Policy
}

// Engine answers who-can queries. Construct with New; it holds only the
// two narrow collaborators.
type Engine struct {
	src      EntitySource
	resolver Resolver
}

// New constructs an Engine. Both collaborators are required; a nil
// either is a programmer error (constructors-reject-nil).
func New(src EntitySource, resolver Resolver) (*Engine, error) {
	if src == nil {
		return nil, errors.New("aclmap: New: EntitySource must be non-nil")
	}
	if resolver == nil {
		return nil, errors.New("aclmap: New: Resolver must be non-nil")
	}
	return &Engine{src: src, resolver: resolver}, nil
}

// Route is one way a principal (or everyone) is granted the verb, at
// terminal-fact depth. Kind is the acl.SourceKind string; Role is the
// role the route confers; Group / Ancestor / Relation name the terminal
// entities of the route and are populated per kind (empty otherwise),
// mirroring acl.Source. A later change may add a Hops field for the
// full chain without breaking this shape.
type Route struct {
	Kind     string `json:"kind"`
	Role     string `json:"role"`
	Group    string `json:"group,omitempty"`
	Ancestor string `json:"ancestor,omitempty"`
	Relation string `json:"relation,omitempty"`
}

// PrincipalAccess is one principal's grant of the verb on the entity,
// with every route that grants it. Principal is the effective identity
// the resolver used (a resolved entity ID when principal_property
// resolved it, else the raw key). Raw is set only when it differs from
// Principal, so the caller can show both.
type PrincipalAccess struct {
	Principal string  `json:"principal"`
	Raw       string  `json:"raw,omitempty"`
	Routes    []Route `json:"routes"`
}

// Everyone describes a global grant of the verb via the built-in
// everyone role, when present. Reported once (not per principal).
type Everyone struct {
	Granted  bool `json:"granted"`
	Wildcard bool `json:"wildcard"`
}

// WhoCanResult is the answer to "who can <verb> <entity>". SchemaVersion
// is bumped on any breaking change to this shape so snapshot/diff
// consumers can detect format changes. Everyone is the global everyone
// grant; Principals is every enumerated principal with a non-everyone
// route, sorted by principal ID.
type WhoCanResult struct {
	SchemaVersion int               `json:"schema_version"`
	Verb          string            `json:"verb"`
	Entity        string            `json:"entity"`
	EntityType    string            `json:"entity_type"`
	Everyone      Everyone          `json:"everyone"`
	Principals    []PrincipalAccess `json:"principals"`
}

// schemaVersion is the WhoCanResult wire version. Bump on breaking
// changes (field removal/rename/semantic change); additive fields do
// not require a bump.
const schemaVersion = 1

// ErrEntityNotFound is returned by WhoCan when the entity does not
// exist. Reporting readers for a non-existent entity would be a
// misleading attestation (a typo'd ID must error, not return the
// global-only reader set).
var ErrEntityNotFound = errors.New("aclmap: entity not found")
