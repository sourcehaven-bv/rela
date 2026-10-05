package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// searchRelation is the optional relation context of /_search and of a
// collection list (`GET /{plural}`): the relation a
// picker is about to create, so each row can say whether that edge may be
// created from it (`linkable`). The zero value means no context.
type searchRelation struct {
	relType string
	scope   metamodel.RelationScope
}

// parseSearchRelation reads `relation` and `direction` from a collection query.
// Both absent is no context. `relation` must name a declared relation type and
// `direction` must be `incoming`: the row is the edge's source. An outgoing
// edge's source is the edited entity itself, so a per-row answer would be the
// same on every row; that direction is refused rather than answered.
//
// The edited entity (the edge's target) is not a parameter because neither
// gate the write runs reads it: the ACL subject of a relation write names only
// the source, and the affordance verdict is per source.
func parseSearchRelation(meta *metamodel.Metamodel, q url.Values) (searchRelation, *v1.WireError) {
	relType, direction := q.Get("relation"), q.Get("direction")
	if relType == "" && direction == "" {
		return searchRelation{}, nil
	}
	def, ok := meta.GetRelationDef(relType)
	if !ok {
		return searchRelation{}, &v1.WireError{Code: "invalid_relation",
			Detail: fmt.Sprintf("relation %q is not declared", relType), Path: "relation"}
	}
	if direction != string(DirectionIncoming) {
		return searchRelation{}, &v1.WireError{Code: "invalid_direction",
			Detail: "direction must be incoming when relation is given", Path: "direction"}
	}
	return searchRelation{relType: relType, scope: def.Scope}, nil
}

// active reports whether the request named a relation context.
func (rc searchRelation) active() bool { return rc.relType != "" }

// linkablePage answers [affordanceService.linkableFrom] for every row of a
// /_search page, keyed by row address. The source rows the gates evaluate are
// read in batch up front (one read per distinct face, plus one header read
// for an identity-scoped relation), so the page costs the same number of
// store reads at 10 rows as at 50.
//
// The batch only feeds relationSources: the source reads of a page-scoped
// copy of svc resolve from it, and fall back to svc's own reads for a row the
// batch missed. Every decision still runs through linkableFrom.
func (svc affordanceService) linkablePage(
	ctx context.Context, rows []*entityPkg.Entity, relType string, scope metamodel.RelationScope,
) (map[entityPkg.Ref]bool, error) {
	ids := make([]string, 0, len(rows))
	keys := make([]entityPkg.Ref, 0, len(rows))
	for _, e := range rows {
		ids = append(ids, e.ID)
		keys = append(keys, entityPkg.Ref{ID: e.ID, Face: e.Face})
	}
	var families map[string]storedFamily
	if !scope.IsContent() {
		// An identity edge has the zero tail, so a faced source is judged on
		// every face of its family the principal may read.
		var err error
		families, err = loadStoredFamilies(ctx, svc.store, ids)
		if err != nil {
			return nil, err
		}
		keys = keys[:0]
		for id, fam := range families {
			for _, f := range fam.faces {
				keys = append(keys, entityPkg.Ref{ID: id, Face: f})
			}
		}
	}
	raw, err := loadRows(ctx, svc.store, keys)
	if err != nil {
		return nil, err
	}

	page := svc
	page.sourceRow = func(ctx context.Context, ref entityPkg.Ref) (*entityPkg.Entity, bool) {
		if e, ok := raw[ref]; ok {
			return e, true
		}
		return svc.sourceRow(ctx, ref)
	}
	if families != nil {
		readable := svc.readableFamilies(ctx, families, raw)
		page.sourceFamily = func(ctx context.Context, id string) ([]*entityPkg.Entity, error) {
			if _, ok := families[id]; !ok {
				return svc.sourceFamily(ctx, id)
			}
			return readable[id], nil
		}
	}

	out := make(map[entityPkg.Ref]bool, len(rows))
	for _, e := range rows {
		out[e.Ref()] = page.linkableFrom(ctx, e, relType, scope)
	}
	return out, nil
}

// readableFamilies is the batch form of [affordanceService.sourceFamily]: the
// rows of families the principal may read, keyed by id and in face order. One
// read-gate pass covers the page, so the cost does not grow with its size.
func (svc affordanceService) readableFamilies(
	ctx context.Context, families map[string]storedFamily, raw map[entityPkg.Ref]*entityPkg.Entity,
) map[string][]*entityPkg.Entity {
	var rows []*entityPkg.Entity
	for id, fam := range families {
		for _, f := range fam.faces {
			if e, ok := raw[entityPkg.Ref{ID: id, Face: f}]; ok {
				rows = append(rows, e)
			}
		}
	}
	out := make(map[string][]*entityPkg.Entity, len(families))
	for _, e := range svc.readable(ctx, rows) {
		out[e.ID] = append(out[e.ID], e)
	}
	for _, fam := range out {
		slices.SortFunc(fam, func(a, b *entityPkg.Entity) int {
			return strings.Compare(string(a.Face), string(b.Face))
		})
	}
	return out
}

// servedLinkable is [affordanceService.linkablePage] for the served rows of a
// collection read with relation context rc. It returns a nil map when rc is
// not active, so the response carries no `linkable`. A read fault is written
// to w and reported as false.
func servedLinkable(
	w http.ResponseWriter, r *http.Request, svc affordanceService, rows []*entityPkg.Entity, rc searchRelation,
) (map[entityPkg.Ref]bool, bool) {
	if !rc.active() {
		return nil, true
	}
	linkable, err := svc.linkablePage(r.Context(), rows, rc.relType, rc.scope)
	if err != nil {
		writeListPipelineError(w, r, fmt.Errorf("%w: %w", errListLoad, err))
		return nil, false
	}
	return linkable, true
}

// linkRows pairs each serialized row with its `linkable` answer. data[i] is
// the wire form of rows[i]; a nil linkable map leaves every row without one.
func linkRows(data []v1.Entity, rows []*entityPkg.Entity, linkable map[entityPkg.Ref]bool) []v1.LinkRow {
	out := make([]v1.LinkRow, len(data))
	for i := range data {
		out[i] = v1.LinkRow{Entity: data[i]}
		if linkable != nil {
			ok := linkable[rows[i].Ref()]
			out[i].Linkable = &ok
		}
	}
	return out
}
