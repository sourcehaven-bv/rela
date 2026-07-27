package aclmap

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// CanResult is the answer to "can principal P perform verb on entity E".
// It is the single-cell case of both [WhoCanResult] (fix the entity) and
// [MapPrincipalResult] (fix the principal): one principal, one verb, one
// entity. Allowed is the yes/no the caller turns into an exit code; Access
// carries the deciding route(s) when Allowed, so a "yes" is explained the
// same way who-can/map explain a grant.
//
// A grant via the built-in everyone role (which [WhoCan] reports globally,
// not per principal) still makes Allowed true and is recorded as the
// Everyone flag — a spot-check must answer "can this principal?" including
// the case where the reason is "everyone can".
type CanResult struct {
	SchemaVersion int    `json:"schema_version"`
	Principal     string `json:"principal"`
	Raw           string `json:"raw,omitempty"`
	Verb          string `json:"verb"`
	Entity        string `json:"entity"`
	EntityType    string `json:"entity_type"`
	Allowed       bool   `json:"allowed"`
	// Everyone is true when the grant is (at least partly) via the built-in
	// everyone role. It can be true with no Routes: everyone grants access
	// that AccessRoutes deliberately omits.
	Everyone bool `json:"everyone"`
	// Routes are the non-everyone route(s) that grant the verb, empty when
	// the only grant is the everyone role (or when denied).
	Routes []Route `json:"routes,omitempty"`
}

// Can reports whether principal rawPrincipal may perform verb on the
// entity entityID, with the deciding route(s). It gates on entity
// existence first (a missing entity errors with [ErrEntityNotFound]
// rather than a misleading deny), resolves the entity's type from the
// store, then asks the resolver for this one principal's routes — the
// same read path who-can and map drive (AccessRoutes gates read on
// PermitsRead), so a "no" is never a false negative.
//
// The everyone role is folded into the decision here (unlike WhoCan,
// which reports it globally): a spot-check for one principal must say
// "yes" when everyone can, even if that principal holds no personal
// route.
func (e *Engine) Can(ctx context.Context, rawPrincipal string, verb acl.Verb, entityID string) (*CanResult, error) {
	if !verb.Valid() {
		return nil, fmt.Errorf("aclmap: unknown verb %q", verb)
	}

	ent, err := e.src.GetEntity(ctx, entityID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrEntityNotFound, entityID)
		}
		return nil, fmt.Errorf("aclmap: load entity %q: %w", entityID, err)
	}
	entityType := ent.Type

	result := &CanResult{
		SchemaVersion: schemaVersion,
		Verb:          string(verb),
		Entity:        entityID,
		EntityType:    entityType,
	}

	// The everyone role grants every principal; AccessRoutes omits it, so
	// consult the policy directly. A wildcard ("*") everyone grant counts.
	if eg := e.resolver.EveryoneGrants(verb, entityType); eg.Granted {
		result.Everyone = true
		result.Allowed = true
	}

	access, ok, err := e.accessFor(ctx, rawPrincipal, verb, entityType, entityID)
	if err != nil {
		return nil, err
	}
	if ok {
		result.Allowed = true
		result.Principal = access.Principal
		result.Raw = access.Raw
		result.Routes = access.Routes
	}
	if result.Principal == "" {
		// Denied, or allowed only via everyone: accessFor returned no row, so
		// record the resolved identity for the report from a bare resolve.
		user, rawShown, rErr := e.resolveEffective(ctx, rawPrincipal)
		if rErr != nil {
			return nil, rErr
		}
		result.Principal = user
		result.Raw = rawShown
	}
	return result, nil
}
