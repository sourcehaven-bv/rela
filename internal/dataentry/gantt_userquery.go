package dataentry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/search"
)

// ganttUserQuery is the reader's own narrowing of a gantt: the free-text `q`
// and the `filter[...]` params the SPA's search box and Filters menu send, as
// on a list. It is distinct from a source's configured where: clauses, which
// the operator wrote.
//
// It runs on the redacted node set (see loadGanttNodes), so a value the
// reader cannot see is absent from what it matches against, and membership
// cannot reveal it. A hidden entity was never in the set.
type ganttUserQuery struct {
	// text is the trimmed `q`. It matches case-insensitively against the
	// entity id and its string properties, as [search.MatchTextFields] does. A
	// gantt reads no markdown bodies, so a body is not searched.
	text string
	// params are the `filter[...]` params, keyed as on the list endpoint.
	params map[string][]string
	// keep is the drilled root. It stays in the set whether or not it
	// matches, so a drill into a node the reader chose keeps answering after
	// the reader narrows the chart.
	keep string
}

// parseGanttUserQuery reads the reader's search and filters from query.
// Empty filter values mean "no constraint", as they do on a list.
func parseGanttUserQuery(query url.Values, keep string) ganttUserQuery {
	uq := ganttUserQuery{text: strings.TrimSpace(query.Get("q")), keep: keep}
	for key, values := range query {
		if !strings.HasPrefix(key, "filter[") || len(values) == 0 || values[len(values)-1] == "" {
			continue
		}
		if uq.params == nil {
			uq.params = map[string][]string{}
		}
		uq.params[key] = values
	}
	return uq
}

func (uq ganttUserQuery) empty() bool { return uq.text == "" && len(uq.params) == 0 }

// validate refuses a filter the gantt cannot place: a malformed key, or a
// property that no source type declares. A filter on a property only SOME
// source types declare is fine; it applies to those types, and the others
// are not narrowed by it.
func (uq ganttUserQuery) validate(meta *metamodel.Metamodel, g dataentryconfig.Gantt) error {
	for key := range uq.params {
		fk, err := parseFilterKey(key)
		if err != nil {
			return err
		}
		found := false
		for typeName := range g.Sources {
			if def, ok := meta.GetEntityDef(typeName); ok {
				if _, has := def.Properties[fk.property]; has {
					found = true
					break
				}
			}
		}
		if !found {
			return fmt.Errorf("%w: property %q is on no source type of this gantt", errBadFilter, fk.property)
		}
	}
	return nil
}

// applyGanttUserQuery narrows the redacted entities of one source type by the
// reader's search and filters.
//
// The filters go through the list's own gate and evaluator (gateListQuery,
// applyV1Filters), so a gantt filter means what the same filter means on a
// list, and a property the reader does not see on every row is refused the
// same way. They run on the REDACTED entities: a hidden value is absent.
func (h *ganttHandler) applyGanttUserQuery(
	ctx context.Context, s *Schema, typeName string, ents []*entity.Entity, uq ganttUserQuery,
) ([]*entity.Entity, *ganttError) {
	if uq.empty() || len(ents) == 0 {
		return ents, nil
	}
	narrowed := ents
	if len(uq.params) > 0 {
		var gerr *ganttError
		narrowed, gerr = h.filterGanttEntities(ctx, s, typeName, narrowed, uq)
		if gerr != nil {
			return nil, gerr
		}
	}
	if uq.text != "" {
		matched := make([]*entity.Entity, 0, len(narrowed))
		for _, e := range narrowed {
			if len(search.MatchTextFields(e, uq.text)) > 0 {
				matched = append(matched, e)
			}
		}
		narrowed = matched
	}
	return withGanttKeep(ents, narrowed, uq.keep), nil
}

// filterGanttEntities applies the filter params that name a property of
// typeName. A param naming a property the type lacks is not this type's.
func (h *ganttHandler) filterGanttEntities(
	ctx context.Context, s *Schema, typeName string, ents []*entity.Entity, uq ganttUserQuery,
) ([]*entity.Entity, *ganttError) {
	def, ok := s.Meta.GetEntityDef(typeName)
	if !ok {
		return ents, nil
	}
	params := map[string][]string{}
	for key, values := range uq.params {
		fk, err := parseFilterKey(key)
		if err != nil {
			return nil, ganttFilterError(err)
		}
		if _, has := def.Properties[fk.property]; has {
			params[key] = values
		}
	}
	if len(params) == 0 || h.fields == nil {
		return ents, nil
	}
	gated, err := gateListQuery(ctx, h.fields(), nil, typeName, params)
	if err != nil {
		return nil, ganttFilterError(err)
	}
	out, err := applyV1Filters(ents, gated)
	if err != nil {
		return nil, ganttFilterError(err)
	}
	return out, nil
}

// withGanttKeep adds the kept entity back when the narrowing dropped it.
func withGanttKeep(all, narrowed []*entity.Entity, keep string) []*entity.Entity {
	if keep == "" {
		return narrowed
	}
	for _, e := range narrowed {
		if e.ID == keep {
			return narrowed
		}
	}
	for _, e := range all {
		if e.ID == keep {
			return append(narrowed, e)
		}
	}
	return narrowed
}

// ganttFilterError maps a filter refusal to the gantt's error shape: a client
// error that echoes only the caller's own key, property and type.
func ganttFilterError(err error) *ganttError {
	switch {
	case errors.Is(err, errPropertyNotVisible):
		return &ganttError{http.StatusBadRequest, "property_not_visible",
			"Cannot filter on this property", err.Error()}
	case errors.Is(err, errBadFilter):
		return &ganttError{http.StatusBadRequest, "invalid_filter", "Invalid filter parameter", err.Error()}
	default:
		return &ganttError{http.StatusInternalServerError, "internal", "Failed to filter entities", ""}
	}
}
