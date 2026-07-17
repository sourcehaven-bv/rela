package acl

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// PrincipalLookup resolves the raw principal identifier to a user entity
// by property equality. It is the narrow, consumer-side contract the
// resolver needs for `principal_property` substitution — declared here
// (not next to the store) per CLAUDE.md's "interfaces at the call site"
// rule. The wiring site supplies a store-backed implementation
// ([StorePrincipalLookup]).
//
// The method returns EVERY matching entity ID so the resolver can
// distinguish the three outcomes it cares about: zero (no match — keep
// the raw principal), one (substitute), and more than one (ambiguous —
// a data-integrity failure the resolver refuses to guess through).
type PrincipalLookup interface {
	// LookupEntityByProperty returns the IDs of entities of entityType
	// whose property equals value (exact string match). A blank value
	// returns nil without querying — an empty principal never resolves.
	// Backend errors propagate so the resolver can fail-open-to-raw and
	// warn rather than silently drop a match.
	LookupEntityByProperty(ctx context.Context, entityType, property, value string) ([]string, error)
}

// storePrincipalLookup adapts a store entity reader to [PrincipalLookup].
// It scans the entities of the requested type and matches the property's
// string value. Production wiring (appbuild) constructs it over the
// store; the ACL resolver only ever sees the [PrincipalLookup] interface.
//
// The scan is O(entities of type) per lookup, run at most once per
// request (upstream of the cached member-of walk). For large user-entity
// sets operators can push the equality into a backend index (see
// docs/security.md); this adapter is the portable default.
type storePrincipalLookup struct {
	s store.EntityReader
}

// NewStorePrincipalLookup constructs a store-backed [PrincipalLookup]
// over s. Returned as the interface so callers depend on the contract,
// not the adapter.
func NewStorePrincipalLookup(s store.EntityReader) PrincipalLookup {
	return &storePrincipalLookup{s: s}
}

// LookupEntityByProperty implements [PrincipalLookup] by scanning
// entities of entityType and collecting those whose property equals
// value.
func (l *storePrincipalLookup) LookupEntityByProperty(
	ctx context.Context, entityType, property, value string,
) ([]string, error) {
	if value == "" {
		return nil, nil
	}
	var ids []string
	for e, err := range l.s.ListEntities(ctx, store.EntityQuery{Type: entityType}) {
		if err != nil {
			return nil, err
		}
		if e.GetString(property) == value {
			ids = append(ids, e.ID)
		}
	}
	return ids, nil
}
