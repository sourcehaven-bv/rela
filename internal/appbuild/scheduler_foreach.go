package appbuild

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ScheduledForEachEntities resolves the bounded recipient/subject selection
// through the scheduler identity's existing visible reader.
func (s *Services) ScheduledForEachEntities(
	ctx context.Context, entityType string, where []string, limit int,
) (ids []string, dropped int, err error) {
	if s == nil || s.meta == nil {
		return nil, 0, errors.New("appbuild: scheduled for_each has no metamodel")
	}
	def, ok := s.meta.GetEntityDef(entityType)
	if !ok {
		return nil, 0, fmt.Errorf("unknown entity type %q", entityType)
	}
	filters, err := filter.ParseAll(where)
	if err != nil {
		return nil, 0, fmt.Errorf("parse filters: %w", err)
	}
	deps := s.ScheduledLuaWriteDeps()
	if deps.VisibleReader == nil {
		return nil, 0, errors.New("appbuild: scheduled for_each has no visible reader")
	}
	ids = make([]string, 0, limit)
	for e, listErr := range deps.VisibleReader.ListEntities(ctx, store.EntityQuery{Type: entityType}) {
		if listErr != nil {
			return nil, 0, listErr
		}
		matched, matchErr := filter.MatchAll(filter.Record{
			ID: e.ID, Type: e.Type, Properties: e.Properties, ModifiedAt: e.UpdatedAt,
		}, filters, def, s.meta)
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
// durable payload.
func (s *Services) ScheduledForEachPrincipal(ctx context.Context, entityID string) (string, error) {
	if s == nil || s.store == nil {
		return "", errors.New("appbuild: scheduled for_each has no store")
	}
	e, err := s.store.GetEntity(ctx, entityID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	if s.aclPolicy == nil || s.aclPolicy.UserEntityType == "" || e.Type != s.aclPolicy.UserEntityType {
		return "", nil
	}
	return e.ID, nil
}
