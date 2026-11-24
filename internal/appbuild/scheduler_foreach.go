package appbuild

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/readtime"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ScheduledForEachEntities resolves the bounded recipient/subject selection
// through the scheduler identity's existing visible reader.
func (w *SchedulerWorkspace) ScheduledForEachEntities(
	ctx context.Context, entityType string, where []string, limit int,
) (ids []string, dropped int, err error) {
	if w == nil || w.meta == nil {
		return nil, 0, errors.New("appbuild: scheduled for_each has no metamodel")
	}
	def, ok := w.meta.GetEntityDef(entityType)
	if !ok {
		return nil, 0, fmt.Errorf("unknown entity type %q", entityType)
	}
	filters, err := filter.ParseAll(where)
	if err != nil {
		return nil, 0, fmt.Errorf("parse filters: %w", err)
	}
	filters = filter.BindDateVariables(filters, readtime.Or(ctx, w.clock))
	deps := w.ScheduledLuaWriteDeps()
	if deps.VisibleReader == nil {
		return nil, 0, errors.New("appbuild: scheduled for_each has no visible reader")
	}
	ids = make([]string, 0, limit)
	q := store.EntityQuery{Type: entityType, Faces: store.InWorld(w.worlds.DefaultWorld())}
	for e, listErr := range deps.VisibleReader.ListEntities(ctx, q) {
		if listErr != nil {
			return nil, 0, listErr
		}
		matched, matchErr := filter.MatchAll(filter.Record{
			ID: e.ID, Type: e.Type, Properties: e.Properties, ModifiedAt: e.UpdatedAt,
		}, filters, def, w.meta)
		if matchErr != nil {
			return nil, 0, matchErr
		}
		if !matched {
			continue
		}
		if len(ids) < limit {
			ids = append(ids, e.ID)
		} else {
			dropped++
		}
	}
	return ids, dropped, nil
}

// ScheduledForEachPrincipal maps a selected user entity to the principal used
// by ACL. The entity ID is the effective principal after the normal
// principal_property resolution; no raw identity or role is trusted from the
// durable payload. The roles the IdP webhook stored for the user ride on the
// principal as asserted roles (TKT-IZL6NX8), read now rather than when the
// run was queued, so a revocation applies to the next child.
func (w *SchedulerWorkspace) ScheduledForEachPrincipal(
	ctx context.Context, entityID string,
) (principal.Principal, error) {
	if w == nil || w.store == nil {
		return principal.Principal{}, errors.New("appbuild: scheduled for_each has no store")
	}
	// A principal is an entity, not one of its faces, so the type check
	// reads the family's headers: a faced user type has no zero-face row
	// (DEC-NPZICR).
	headers, err := store.FamilyHeaders(ctx, w.store, entityID)
	if err != nil {
		return principal.Principal{}, err
	}
	var typ string
	if len(headers) > 0 {
		typ = headers[0].Type
	}
	if typ == "" || w.aclPolicy == nil || w.aclPolicy.UserEntityType == "" || typ != w.aclPolicy.UserEntityType {
		return principal.Principal{}, nil
	}
	roles, err := storedAssertedRoles(ctx, w.idpRoles, w.aclPolicy, w.store, entityID, headers)
	if err != nil {
		return principal.Principal{}, fmt.Errorf("stored asserted roles: %w", err)
	}
	return forEachPrincipal(entityID, roles), nil
}
