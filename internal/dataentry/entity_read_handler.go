package dataentry

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// entityReadHandler owns the v1 entity read surface: the collection and
// single-entity routes under /{plural}, their relation sub-resources,
// `?include=` expansion, the entity ETag, /_search and /_position. It also
// dispatches the write verbs on those routes to the write handler, so one
// type answers every method on an entity URL. Extracted from App to take
// that cluster off the god-object.
//
// Every body it emits has passed the row-level read gate and been resolved
// through the request's world; a hidden entity id answers the uniform 404
// ([entityNotFoundTitle]). The list rows come from [listPipeline], which
// applies the ACL scope before the search, the filters and the sort.
//
// The collaborators are closures over App rather than captured values: tests
// and the wiring setters reassign App.reader, App.visibleReader,
// App.faceEdges, App.serializer, App.affordances, App.write and others after
// construction, and a captured copy would go stale (the schemaHandler idiom).
type entityReadHandler struct {
	// schema is the reloadable snapshot.
	schema func() *Schema
	// services returns the read bundle over the app's current store.
	services func() Services
	// reader is the ungated read seam: edges and peer types, always behind
	// a visible-reader gate on the entity they hang off.
	reader func() entityReader
	// visible is the row-gated reader every addressed read resolves through.
	visible func() visibleReader
	// viewReader is the row-gating, field-redacting reader the mentions and
	// the retired-id redirect read through (DEC-ZBI39P).
	viewReader func() visibility.Reader
	// worldNeighbors resolves links under the request's world. Nil until
	// [SetWorldNeighbors].
	worldNeighbors func() *worldNeighbors
	// faceEdges is the gated read of a served face's edges.
	faceEdges func() faceEdgeReader
	// serializer renders an entity into its wire shape.
	serializer func() entitySerializer
	// affordances computes the action maps and redacts relation meta.
	affordances func() affordanceService
	// fields answers the field verdicts that prime a page.
	fields func() FieldVerdictResolver
	// queries runs /_search and the search scope of /_position.
	queries func() *queryService
	// leases fills `_lease`. Nil when leases are not wired.
	leases func() *leasesHandler
	// piles answers the pile scope of /_position.
	piles func() *pilesHandler
	// rollups computes the roll-ups of a single-entity GET.
	rollups func() rollupSource
	// lists is the shared list pipeline.
	lists func() *listPipeline
	// write takes the write verbs on the entity routes.
	write func() entityWriteRoutes
	// rowExtras is serveListRowExtras bound to the app: owners, comment
	// counts and busy flags on list rows.
	rowExtras func(w http.ResponseWriter, r *http.Request, query map[string][]string,
		rows []*entityPkg.Entity, data []v1.Entity) bool
	// owners is serveOwners bound to the app.
	owners func(w http.ResponseWriter, r *http.Request, rows []*entityPkg.Entity, data []v1.Entity) bool
	// relationTarget is handleV1GetRelationTarget bound to the app.
	relationTarget func(w http.ResponseWriter, r *http.Request, typeName, addr, relType, targetID string)
	// storePosition and searchPosition answer /_position from the store
	// when the scope allows it (storePosition, searchStorePosition).
	storePosition  func(w http.ResponseWriter, r *http.Request, scope ScopeDescriptor, id string) (*v1.Position, bool)
	searchPosition func(w http.ResponseWriter, r *http.Request, scope ScopeDescriptor, id string) (*v1.Position, bool)
}

// entityWriteRoutes is the set of write verbs the entity routes forward to.
// It is wider than the usual one to three methods because it is a dispatch
// table: each method is one route, and the read handler calls nothing else on
// the write side.
type entityWriteRoutes interface {
	handleV1CreateEntity(w http.ResponseWriter, r *http.Request, typeName, plural string)
	handleV1DryRunCreate(w http.ResponseWriter, r *http.Request, typeName, plural string)
	handleV1UpdateEntity(w http.ResponseWriter, r *http.Request, typeName, plural, entityID string)
	handleV1DeleteEntity(w http.ResponseWriter, r *http.Request, typeName, plural, entityID string)
	handleV1CloneEntity(w http.ResponseWriter, r *http.Request, typeName, entityID string)
	handleV1CreateRelation(w http.ResponseWriter, r *http.Request, typeName, entityID, relType string)
	handleV1UpdateRelation(w http.ResponseWriter, r *http.Request, typeName, entityID, relType, targetID string)
	handleV1DeleteRelation(w http.ResponseWriter, r *http.Request, typeName, entityID, relType, targetID string)
}

// newEntityReadHandler wires the handler to app's live collaborators.
func newEntityReadHandler(app *App) *entityReadHandler {
	return &entityReadHandler{
		schema:         app.State,
		services:       app.Services,
		reader:         func() entityReader { return app.reader },
		visible:        func() visibleReader { return app.visibleReader },
		viewReader:     func() visibility.Reader { return app.viewReader },
		worldNeighbors: func() *worldNeighbors { return app.worldNeighbors },
		faceEdges:      func() faceEdgeReader { return app.faceEdges },
		serializer:     func() entitySerializer { return app.serializer },
		affordances:    func() affordanceService { return app.affordances },
		fields:         func() FieldVerdictResolver { return app.fieldResolver },
		queries:        func() *queryService { return app.queries },
		leases:         func() *leasesHandler { return app.leases },
		piles:          func() *pilesHandler { return app.piles },
		rollups:        func() rollupSource { return newRollupSource(app) },
		lists:          func() *listPipeline { return app.lists },
		write:          func() entityWriteRoutes { return app.write },
		rowExtras: func(w http.ResponseWriter, r *http.Request, query map[string][]string,
			rows []*entityPkg.Entity, data []v1.Entity,
		) bool {
			return serveListRowExtras(w, r, app, query, rows, data)
		},
		owners: func(w http.ResponseWriter, r *http.Request, rows []*entityPkg.Entity, data []v1.Entity) bool {
			return serveOwners(w, r, app, rows, data)
		},
		relationTarget: func(w http.ResponseWriter, r *http.Request, typeName, addr, relType, targetID string) {
			handleV1GetRelationTarget(app, w, r, typeName, addr, relType, targetID)
		},
		storePosition: func(
			w http.ResponseWriter, r *http.Request, scope ScopeDescriptor, id string,
		) (*v1.Position, bool) {
			return storePosition(app, w, r, scope, id)
		},
		searchPosition: func(
			w http.ResponseWriter, r *http.Request, scope ScopeDescriptor, id string,
		) (*v1.Position, bool) {
			return searchStorePosition(app, w, r, scope, id)
		},
	}
}

func (h *entityReadHandler) handleV1EntityCollection(w http.ResponseWriter, r *http.Request, typeName, plural string) {
	switch r.Method {
	case http.MethodGet:
		h.handleV1ListEntities(w, r, typeName, plural)
	case http.MethodPost:
		if r.URL.Query().Get("dry_run") == "true" {
			h.write().handleV1DryRunCreate(w, r, typeName, plural)
			return
		}
		h.write().handleV1CreateEntity(w, r, typeName, plural)
	case http.MethodOptions:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
	default:
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

//archplan:auth=required
func (h *entityReadHandler) handleV1ListEntities(w http.ResponseWriter, r *http.Request, typeName, plural string) {
	query, state := r.URL.Query(), h.schema()
	page, perPage := parseV1Pagination(query)
	relCtx, relErr := parseSearchRelation(state.Meta, query)
	if relErr != nil {
		writeV1Error(w, r, http.StatusBadRequest, relErr.Code, relErr.Detail, relErr.Path)
		return
	}
	entities, total, ordering, err := h.lists().listPage(r.Context(), typeName, query, page, perPage)
	if err != nil {
		writeListPipelineError(w, r, err)
		return
	}
	end := (page-1)*perPage + len(entities)
	linkable, ok := servedLinkable(w, r, h.affordances(), entities, relCtx)
	if !ok {
		return
	}

	if wantContent(query) {
		if cErr := loadRowContent(r.Context(), h.services().Store, entities); cErr != nil {
			writeListPipelineError(w, r, fmt.Errorf("%w: %w", errListLoad, cErr))
			return
		}
	}

	includes := query.Get("include")
	wantIncludes := includes != ""

	outgoingByRow, incomingByRow, visibleNeighbors, werr := servedFacePageEdges(
		r.Context(), h.reader(), h.worldNeighbors(), h.visible(), entities)
	if werr != nil {
		writeGateError(w, r, werr)
		return
	}

	data := make([]v1.Entity, 0, len(entities))
	var included map[string]v1.Entity
	pageCtx, rowFields := primeRowFields(r.Context(), h.fields(), entities, query)
	for i, e := range entities {
		v1Entity := h.serializer().forWireRelated(
			pageCtx, e,
			outgoingByRow[i],
			incomingByRow[i],
			visibleNeighbors,
			state.Meta, plural,
		)
		attachRowFields(pageCtx, h.affordances(), e, &v1Entity, rowFields)
		if !worldScopeFrom(r.Context()).IsTrivial() {
			v1Entity.World = worldProvenance(r.Context(), e)
		}
		data = append(data, v1Entity)
	}

	if wantIncludes {
		included = h.resolveV1Includes(r.Context(), entities, includes)
	}

	if !h.rowExtras(w, r, query, entities, data) {
		return
	}

	resp := v1.ListResponse{
		Data: data,
		Meta: v1.ListMeta{
			Total: total, Page: page, PerPage: perPage, HasMore: end < total,
			RelationOrder: ordering.wire(),
			ManualOrder:   ordering == nil && manualOrderApplies(r.Context(), h.lists(), state, query),
		},
		Actions: addToolbarActions(r.Context(), state.Cfg, typeName,
			h.affordances().computeCollectionActions(r.Context(), typeName)),
	}

	setListHeaders(w, r, page, perPage, total, plural)

	if linkable != nil {
		writeV1JSON(w, http.StatusOK, v1.LinkListResponse{
			Data: linkRows(resp.Data, entities, linkable), Meta: resp.Meta,
			Included: included, Actions: resp.Actions,
		})
		return
	}

	if len(included) > 0 {
		type listWithIncludes struct {
			Data     []v1.Entity          `json:"data"`
			Meta     v1.ListMeta          `json:"meta"`
			Included map[string]v1.Entity `json:"included,omitempty"`
			Actions  map[string]bool      `json:"_actions,omitempty"`
		}
		writeV1JSON(w, http.StatusOK, listWithIncludes{
			Data:     resp.Data,
			Meta:     resp.Meta,
			Included: included,
			Actions:  resp.Actions,
		})
		return
	}

	writeV1JSON(w, http.StatusOK, resp)
}

func (h *entityReadHandler) handleV1SingleEntity(
	w http.ResponseWriter, r *http.Request, typeName, plural, entityID string,
) {
	switch r.Method {
	case http.MethodGet:
		h.handleV1GetEntity(w, r, typeName, plural, entityID)
	case http.MethodPatch:
		h.write().handleV1UpdateEntity(w, r, typeName, plural, entityID)
	case http.MethodDelete:
		h.write().handleV1DeleteEntity(w, r, typeName, plural, entityID)
	case http.MethodOptions:
		w.Header().Set("Allow", "GET, PATCH, DELETE, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
	default:
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

// handleV1GetEntity serves one entity. Every miss (malformed address, denied
// face, wrong type, hidden row) costs the same lookup and returns the same
// 404, so timing and body cannot enumerate ids. Only this GET redirects a
// retired id; writes never follow a redirect.
//
//archplan:auth=authz
func (h *entityReadHandler) handleV1GetEntity(
	w http.ResponseWriter, r *http.Request, typeName, plural, entityID string,
) {
	ctx := r.Context()

	meta := h.schema().Meta
	entity, found := readAddressedOr(w, r, h.visible(), typeName, entityID, func() {
		if id := retiredRedirectID(entityID); id != "" && redirectRetired(w, r, h.viewReader(), id, pluralOf(meta)) {
			return
		}
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
	})
	if !found {
		return
	}
	ctx = primeVerdicts(ctx, h.fields(), []*entityPkg.Entity{entity})

	query := r.URL.Query()

	outgoing, visibleNeighbors, werr := h.faceEdges().read(ctx, entity)
	if werr != nil {
		writeGateError(w, r, werr)
		return
	}
	result := h.serializer().forWireScoped(ctx, entity, outgoing, visibleNeighbors, meta, plural)
	result.Versions = fieldVersionsOf(&result, meta)
	h.rollups().add(ctx, entity.Type, []rollupRow{{ref: entity.Ref(), props: result.Properties}},
		worldScopeFrom(ctx), nil)

	if isExplicitAddress(entityID) {
		result.World = addressedProvenance(ctx, entity)
	} else {
		result.World = worldProvenance(ctx, entity)
	}

	if includes := query.Get("include"); includes != "" {
		result.Included = h.resolveV1Includes(ctx, []*entityPkg.Entity{entity}, includes)
	}

	result.Mentions = collectMentions(ctx, h.viewReader(), meta, entity.Content)

	result.Lease = h.leases().entityLease(ctx, entity.ID)

	etag := servedETag(ctx, h.affordances(), entity, outgoing)
	w.Header().Set("ETag", etag)

	if h.leases() == nil && len(meta.RollupNames(entity.Type)) == 0 &&
		r.Header.Get("If-None-Match") == etag {

		w.WriteHeader(http.StatusNotModified)
		return
	}

	writeV1JSON(w, http.StatusOK, result)
}

// handleV1EntityRelations serves the edges of the addressed face. A hidden
// entity or withheld face is an indistinguishable 404, and edges to hidden
// neighbors are dropped before their type is read, so neither existence nor
// neighbor ids leak.
//
//archplan:auth=authz
func (h *entityReadHandler) handleV1EntityRelations(w http.ResponseWriter, r *http.Request, typeName, entityID string) {
	entity, found := readAddressedOr404(w, r, h.visible(), typeName, entityID)
	if !found {
		return
	}
	ref := entity.Ref()
	s := h.schema()

	outgoing := h.reader().outgoingRelationsOnFace(r.Context(), ref)
	incoming, err := readableIncoming(r.Context(), h.reader(), h.visible(), ref.ID)
	if err != nil {
		writeGateError(w, r, err)
		return
	}

	visibleNeighbors := visibleRelationIDs(r.Context(), h.visible(),
		neighborIDsOf(outgoing, nil))

	relations := make(map[string][]map[string]any)

	groupSortProp := make(map[string]string)

	type typedStrip struct {
		relationMetaStrip
		relType string
	}
	var pendingStrips []typedStrip

	for _, edge := range outgoing {
		if !visibleNeighbors[edge.To] {
			continue
		}
		rel := map[string]any{
			"id":        edge.To,
			"type":      h.reader().entityType(r.Context(), edge.To),
			"direction": "outgoing",
		}
		if len(edge.Properties) > 0 {
			rel["meta"] = edge.Properties
			pendingStrips = append(pendingStrips, typedStrip{
				relationMetaStrip: relationMetaStrip{rel: rel}, relType: edge.Type,
			})
		}
		relations[edge.Type] = append(relations[edge.Type], rel)
		if relDef, ok := s.Meta.Relations[edge.Type]; ok {
			if p := relDef.OutgoingOrderProperty(); p != "" {
				groupSortProp[edge.Type] = p
			}
		}
	}

	for _, edge := range incoming {
		relDef, ok := s.Meta.Relations[edge.Type]
		if !ok {
			continue
		}
		inverseName := inverseRelationKey(edge.Type, relDef)
		rel := map[string]any{
			"id":        edge.From,
			"type":      h.reader().entityType(r.Context(), edge.From),
			"direction": "incoming",
		}
		h.markIncomingFace(r.Context(), rel, entity, edge)
		if len(edge.Properties) > 0 {
			rel["meta"] = edge.Properties
			pendingStrips = append(pendingStrips, typedStrip{
				relationMetaStrip: relationMetaStrip{
					rel: rel, incoming: true, peer: entityPkg.Ref{ID: edge.From, Face: edge.FromFace},
				},
				relType: edge.Type,
			})
		}
		relations[inverseName] = append(relations[inverseName], rel)
		if p := relDef.IncomingOrderProperty(); p != "" {
			groupSortProp[inverseName] = p
		}
	}

	for groupName, prop := range groupSortProp {
		if prop == "" {
			continue
		}
		sortRelationGroup(relations[groupName], prop)
	}

	for _, ps := range pendingStrips {
		h.affordances().redactRelationMetaStrip(r.Context(), ps.relationMetaStrip, entity, ps.relType)
	}

	writeV1JSON(w, http.StatusOK, relations)
}

// buildRelationTypeRows builds the single-relation-type wire rows (id/type[/meta])
// for the visible edges of relType in the given direction, plus the deferred meta
// strips. An incoming row is marked with its source face
// ([entityReadHandler.markIncomingFace]).
// It is the shared build step for handleV1GetRelationType; the caller
// sorts then applies affordanceService.redactRelationMetaStrip.
func (h *entityReadHandler) buildRelationTypeRows(
	ctx context.Context, pathEntity *entityPkg.Entity, edges []*entityPkg.Relation,
	relType string, incoming bool, visibleNeighbors map[string]bool,
) (rows []map[string]any, strips []relationMetaStrip) {
	reader := h.reader()
	rows = make([]map[string]any, 0, len(edges))
	for _, edge := range edges {
		if edge.Type != relType {
			continue
		}
		peerID := edge.To
		if incoming {
			peerID = edge.From
		}
		if !visibleNeighbors[peerID] {
			continue
		}
		rel := map[string]any{"id": peerID, "type": reader.entityType(ctx, peerID)}
		if incoming {
			h.markIncomingFace(ctx, rel, pathEntity, edge)
		}
		if len(edge.Properties) > 0 {
			rel["meta"] = edge.Properties
			s := relationMetaStrip{rel: rel, incoming: incoming}
			if incoming {
				s.peer = entityPkg.Ref{ID: edge.From, Face: edge.FromFace}
			}
			strips = append(strips, s)
		}
		rows = append(rows, rel)
	}
	return rows, strips
}

func (h *entityReadHandler) handleV1EntityRelationType(
	w http.ResponseWriter, r *http.Request, typeName, entityID, relType string,
) {
	switch r.Method {
	case http.MethodGet:
		h.handleV1GetRelationType(w, r, typeName, entityID, relType)
	case http.MethodPost:
		h.write().handleV1CreateRelation(w, r, typeName, entityID, relType)
	default:
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

// handleV1GetRelationType serves the addressed face's edges of one relation
// type. It is gated like [entityReadHandler.handleV1EntityRelations]: hidden
// neighbors are dropped before their type is read, and relation meta is
// redacted after sorting.
//
//archplan:auth=authz
func (h *entityReadHandler) handleV1GetRelationType(
	w http.ResponseWriter, r *http.Request, typeName, entityID, relType string,
) {
	entity, found := readAddressedOr404(w, r, h.visible(), typeName, entityID)
	if !found {
		return
	}
	ref := entity.Ref()

	incoming := r.URL.Query().Get("direction") == string(DirectionIncoming)

	meta := h.schema().Meta
	var edges []*entityPkg.Relation
	if incoming {
		var err error
		if edges, err = readableIncoming(r.Context(), h.reader(), h.visible(), ref.ID); err != nil {
			writeGateError(w, r, err)
			return
		}
	} else {
		edges = h.reader().outgoingRelationsOnFace(r.Context(), ref)
	}

	var peerIDs []string
	for _, edge := range edges {
		if edge.Type != relType {
			continue
		}
		peerID := edge.To
		if incoming {
			peerID = edge.From
		}
		peerIDs = append(peerIDs, peerID)
	}
	var visibleNeighbors map[string]bool
	if incoming {
		visibleNeighbors = make(map[string]bool, len(peerIDs))
		for _, id := range peerIDs {
			visibleNeighbors[id] = true
		}
	} else {
		visibleNeighbors = visibleRelationIDs(r.Context(), h.visible(), peerIDs)
	}

	relations, pendingStrips := h.buildRelationTypeRows(r.Context(), entity, edges, relType, incoming, visibleNeighbors)

	if relDef, ok := meta.Relations[relType]; ok {
		var prop string
		if incoming {
			prop = relDef.IncomingOrderProperty()
		} else {
			prop = relDef.OutgoingOrderProperty()
		}
		if prop != "" {
			sortRelationGroup(relations, prop)
		}
	}

	for _, ps := range pendingStrips {
		h.affordances().redactRelationMetaStrip(r.Context(), ps, entity, relType)
	}

	writeV1JSON(w, http.StatusOK, relations)
}

func (h *entityReadHandler) handleV1RelationTarget(
	w http.ResponseWriter, r *http.Request, typeName, entityID, relType, targetID string,
) {
	switch r.Method {
	case http.MethodGet:
		h.relationTarget(w, r, typeName, entityID, relType, targetID)
	case http.MethodPatch:
		h.write().handleV1UpdateRelation(w, r, typeName, entityID, relType, targetID)
	case http.MethodDelete:
		h.write().handleV1DeleteRelation(w, r, typeName, entityID, relType, targetID)
	default:
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
	}
}

//archplan:auth=authz
func (h *entityReadHandler) handleV1EntityAction(
	w http.ResponseWriter, r *http.Request, typeName, entityID, action string,
) {
	if r.Method != http.MethodPost {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	switch action {
	case "clone":
		h.write().handleV1CloneEntity(w, r, typeName, entityID)
	default:
		writeV1Error(w, r, http.StatusNotFound, "unknown_action", "Unknown action", "")
	}
}

// handleV1Search serves free-text search, gated to what the request principal
// may read. A `type` parameter scopes the query before the relevance cap, so
// other types cannot use up the slots.
//
//archplan:auth=required
func (h *entityReadHandler) handleV1Search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	limit, ok := parseSearchLimit(r.URL.Query().Get("limit"))
	if !ok {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_limit",
			fmt.Sprintf("limit must be an integer between 1 and %d", maxSearchLimit), "")
		return
	}
	page, ok := parseSearchPage(r.URL.Query().Get("page"))
	if !ok {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_page",
			fmt.Sprintf("page must be an integer between 1 and %d", maxSearchPage), "")
		return
	}
	perPage := limit
	if perPage == 0 {
		perPage = maxSearchResults
	}
	meta := h.schema().Meta
	relCtx, werr := parseSearchRelation(meta, r.URL.Query())
	if werr != nil {
		writeV1Error(w, r, http.StatusBadRequest, werr.Code, werr.Detail, werr.Path)
		return
	}
	query := r.URL.Query().Get("q")
	if query == "" {
		writeV1JSON(w, http.StatusOK, v1.LinkListResponse{Data: []v1.LinkRow{}, Meta: v1.ListMeta{}})
		return
	}

	typeFilter := r.URL.Query().Get("type")
	if _, declared := meta.Entities[typeFilter]; declared {
		query = "type:" + typeFilter + " " + query
	}

	offset := (page - 1) * perPage
	entities, total, capped, err := h.queries().searchPage(r.Context(), query,
		queryOptions{limit: perPage, exactType: typeFilter, offset: offset})
	if err != nil {
		writeListPipelineError(w, r, err)
		return
	}

	if wantContent(r.URL.Query()) {
		if cErr := loadRowContent(r.Context(), h.services().Store, entities); cErr != nil {
			writeListPipelineError(w, r, fmt.Errorf("%w: %w", errListLoad, cErr))
			return
		}
	}

	linkable, ok := servedLinkable(w, r, h.affordances(), entities, relCtx)
	if !ok {
		return
	}

	data := make([]v1.Entity, 0, len(entities))
	pageCtx := primeVerdicts(r.Context(), h.fields(), entities)
	for _, e := range entities {
		entityDef := meta.Entities[e.Type]
		plural := entityDef.GetPlural(e.Type)
		row := h.serializer().forWireRelated(pageCtx, e, nil, nil, nil, meta, plural)
		if !worldScopeFrom(r.Context()).IsTrivial() {
			row.World = worldProvenance(r.Context(), e)
		}
		data = append(data, row)
	}
	if !h.owners(w, r, entities, data) {
		return
	}

	resp := v1.LinkListResponse{
		Data: linkRows(data, entities, linkable),
		Meta: v1.ListMeta{
			Total:   total,
			Page:    page,
			PerPage: perPage,
			HasMore: offset+len(data) < total,
			Capped:  capped,
		},
	}

	writeV1JSON(w, http.StatusOK, resp)
}

// resolveV1Includes expands `?include=` for a set of rows into the
// `included` map. The single-entity GET passes one row; the list passes its
// served page.
//
// The page is resolved in ONE batch per include level (TASK-550RD): one edge
// read and one head resolution for all rows, one gate pass per entity type,
// then one recursive call per nested expression. Resolving row by row made
// the query count grow with the page — the Atlas task list with `include=*`
// cost about 8,000 queries per poll.
func (h *entityReadHandler) resolveV1Includes(
	ctx context.Context, rows []*entityPkg.Entity, includes string,
) map[string]v1.Entity {
	if len(rows) == 0 {
		return nil
	}
	s := h.schema()
	included := make(map[string]v1.Entity)

	candidates, nested, err := includeCandidates(ctx, h.reader(), h.worldNeighbors(), rows, includes)
	if err != nil {
		slog.Warn("dataentry: resolving world-scoped includes failed; include block omitted for every row",
			"first_entity", rows[0].ID, "rows", len(rows), "include", includes, "err", err)
		return nil
	}

	visible := h.filterVisibleIncludes(ctx, candidates)
	byID := make(map[string]*entityPkg.Entity, len(visible))
	order := make([]string, 0, len(visible))
	for _, target := range visible {
		if _, dup := byID[target.ID]; !dup {
			order = append(order, target.ID)
		}
		byID[target.ID] = target
	}
	targets := make([]*entityPkg.Entity, 0, len(order))
	for _, id := range order {
		targets = append(targets, byID[id])
	}

	ctx = primeVerdicts(ctx, h.fields(), targets)
	for _, target := range targets {
		entityDef := s.Meta.Entities[target.Type]
		plural := entityDef.GetPlural(target.Type)
		wired := h.serializer().forWireRelated(ctx, target, nil, nil, nil, s.Meta, plural)
		wired.World = worldProvenance(ctx, target)
		included[target.ID] = wired
	}

	var exprs []string
	byExpr := make(map[string][]*entityPkg.Entity)
	for _, n := range nested {
		target, ok := byID[n.id]
		if !ok {
			continue
		}
		if _, known := byExpr[n.expr]; !known {
			exprs = append(exprs, n.expr)
		}
		byExpr[n.expr] = append(byExpr[n.expr], target)
	}
	for _, expr := range exprs {
		maps.Copy(included, h.resolveV1Includes(ctx, byExpr[expr], expr))
	}
	return included
}

// filterVisibleIncludes drops any candidate the principal cannot read,
// batched by entity type. For each distinct type ONE gate call
// (ReadableFacesMany over every candidate of that type) — turning a
// worst case of O(N) per-id probes into O(distinct-types). RR-FRK1.
//
// On gate error: drop the whole type's candidates (fail-closed) and
// log loud so operators see the underlying failure rather than just
// "the include block is empty." The include channel is a "best
// effort" affordance; the explicit GET / list endpoints are the
// authoritative read surface.
func (h *entityReadHandler) filterVisibleIncludes(
	ctx context.Context, candidates []*entityPkg.Entity,
) []*entityPkg.Entity {
	return h.visible().filterVisible(ctx, candidates)
}

// computeEntityETag reads the entity's edges itself. Callers that ALREADY hold
// the edges they served should use [servedETag] instead — see the
// duplication note at the single-entity GET's call site.
func (h *entityReadHandler) computeEntityETag(ctx context.Context, e *entityPkg.Entity) string {
	edges, err := h.faceEdges().edges(ctx, e)
	if err != nil {
		return etagUnresolved(ctx, h.affordances(), e)
	}
	return servedETag(ctx, h.affordances(), e, edges)
}

// resolveScope produces the fully ordered entity set a scope refers to,
// dispatching on Source. A list scope runs the shared list pipeline
// (single-type, property-sorted). A search scope runs executeQuery — the same
// relevance-ordered, possibly-mixed-type pipeline the search view uses — then
// narrows to Type when one was supplied. Routing search through executeQuery
// (not scopedSortedEntities) is what makes prev/next correct across mixed-type
// search results: position is found within the exact set the user saw.
func (h *entityReadHandler) resolveScope(ctx context.Context, scope ScopeDescriptor) ([]*entityPkg.Entity, error) {
	switch scope.Source {
	case "pile":
		return h.piles().scopeEntities(ctx, scope.Pile)
	case "search":
		entities, err := h.queries().executeQuery(ctx, scope.Q)
		if err != nil {
			return nil, err
		}
		if scope.Type != "" {
			filtered := entities[:0]
			for _, e := range entities {
				if e.Type == scope.Type {
					filtered = append(filtered, e)
				}
			}
			entities = filtered
		}
		return entities, nil
	default:
		query := scope.toQuery()
		rows, err := h.lists().scopedSortedEntities(ctx, scope.Type, query)
		if err != nil {
			return nil, err
		}
		return h.lists().scopeListCondition(ctx, scope.Type, query, rows)
	}
}

// handleV1EntityPosition resolves an entity's position within a scope. It
// reproduces the scope's ordered set via resolveScope (the list pipeline for
// source=list, the search pipeline for source=search) then locates the id,
// returning {prev, next, current, total}. This supersedes the old client-side
// approach where useScopeNavigation fetched per_page=1000 and scanned the
// array — which silently truncated once the set exceeded the pagination cap
// (issue #844).
//
//	GET /api/v1/_position?id=<entityID>&scope=<urlencoded-json>
//	→ 200 {prev,next,current,total} | 400 bad scope | 404 id not in scope
//
//archplan:auth=authz
func (h *entityReadHandler) handleV1EntityPosition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	query := r.URL.Query()

	id := strings.TrimSpace(query.Get("id"))
	if id == "" {
		writeV1Error(w, r, http.StatusBadRequest, "bad_request", "id is required", "")
		return
	}

	scope, ok, reason := scopeFromParam(query.Get("scope"), h.schema().Meta)
	if !ok {
		writeV1Error(w, r, http.StatusBadRequest, "bad_scope", "Invalid scope", reason)
		return
	}

	if pos, handled := h.storePosition(w, r, scope, id); handled {
		if pos != nil {
			writeV1JSON(w, http.StatusOK, *pos)
		}
		return
	}
	if pos, handled := h.searchPosition(w, r, scope, id); handled {
		if pos != nil {
			writeV1JSON(w, http.StatusOK, *pos)
		}
		return
	}

	entities, err := h.resolveScope(r.Context(), scope)
	if err != nil {
		if scope.Source == "pile" {
			writePilesError(w, r, err)
			return
		}
		writeListPipelineError(w, r, err)
		return
	}

	idx := scopeIndex(scope, entities, id)
	if idx == -1 {
		writeV1Error(w, r, http.StatusNotFound, "not_in_scope", "Entity not found in scope", "")
		return
	}

	pos := v1.Position{Current: idx + 1, Total: len(entities)}
	if idx > 0 {
		pos.Prev = scopePositionRef(scope, entities[idx-1])
	}
	if idx < len(entities)-1 {
		pos.Next = scopePositionRef(scope, entities[idx+1])
	}

	writeV1JSON(w, http.StatusOK, pos)
}

// markIncomingFace adds `face` and `editable` to the wire row of an incoming
// edge whose tail is a face. Such an edge belongs to that face of its source,
// so a client addresses it as `ID@face`; editable reports whether the
// principal may remove it. The server re-authorizes every write.
func (h *entityReadHandler) markIncomingFace(
	ctx context.Context, row map[string]any, pathEntity *entityPkg.Entity, edge *entityPkg.Relation,
) {
	if edge.FromFace.IsImplicit() {
		return
	}
	row["face"] = string(edge.FromFace)
	row["editable"] = false
	req := translateRelationDelete(h.schema().Meta, edge.Type, h.reader().entityType(ctx, edge.From),
		edge.From, edge.FromFace)
	if !h.affordances().acl().AuthorizeWrite(ctx, req).Allow {
		return
	}
	sources, err := h.affordances().relationSources(ctx, pathEntity,
		entityPkg.Ref{ID: edge.From, Face: edge.FromFace}, string(DirectionIncoming), edge.Type)
	if err != nil {
		return
	}
	_, denial := h.affordances().relationOpDenial(ctx, sources, edge.Type, RelationOpRemove)
	row["editable"] = denial == nil
}
