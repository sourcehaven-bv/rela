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
		// TKT-3I5U: ?dry_run=true evaluates affordances + soft validation
		// against the candidate WITHOUT persisting, so the create form can
		// gate fields / options / hidden as the user types. Read-shaped:
		// dispatched before handleV1CreateEntity, and never persists.
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
	// The served page only: rows paged out are never judged.
	linkable, ok := servedLinkable(w, r, h.affordances(), entities, relCtx)
	if !ok {
		return
	}

	// Bodies are opt-in for a collection (rowcontent.go): one read per
	// distinct face on the page, never for the rows that were paged out.
	if wantContent(query) {
		if cErr := loadRowContent(r.Context(), h.services().Store, entities); cErr != nil {
			writeListPipelineError(w, r, fmt.Errorf("%w: %w", errListLoad, cErr))
			return
		}
	}

	// Check if includes are requested (for relation columns)
	includes := query.Get("include")
	wantIncludes := includes != ""

	// Load each row's edges once. List rows carry BOTH outgoing edges (keyed
	// by relation type) and incoming edges (keyed by the relation's inverse
	// name) so that `direction: incoming` relation columns have a wire key to
	// resolve against. The SPA computes the same inverse key and reads the
	// source entities from the ?include=* peers. See MECHANISM.md.
	//
	// Each row's links come from THAT ROW'S OWN FACE, in every world
	// including the default one — see [servedFacePageEdges].
	outgoingByRow, incomingByRow, visibleNeighbors, werr := servedFacePageEdges(
		r.Context(), h.reader(), h.worldNeighbors(), h.visible(), entities)
	if werr != nil {
		// Infrastructure failure, not an empty page. See the same arm on the
		// single-entity GET.
		writeGateError(w, r, werr)
		return
	}

	// Build response - always include relations for relation column support
	data := make([]v1.Entity, 0, len(entities))
	var included map[string]v1.Entity // nil when none; both encoders omit it
	pageCtx := primeVerdicts(r.Context(), h.fields(), entities)
	for i, e := range entities {
		v1Entity := h.serializer().forWireRelated(
			pageCtx, e,
			outgoingByRow[i],
			incomingByRow[i],
			visibleNeighbors,
			state.Meta, plural,
		)
		// Face provenance per ROW (BUG-3). Under a chain a row that fell back
		// to a later candidate is byte-identical to a first-choice hit — same
		// id, same title — and only `_world` separates them. Without it a
		// picker or list cannot say which face a row is, which is the whole
		// distinction content states exist to make visible.
		//
		// nil under the default world, so an ordinary list response is
		// unchanged: every row resolves to its default face there, and a block
		// on every row would be noise that also implies a world was applied.
		// Same choice loadViewEntities' provenanceFor makes, for the same
		// reason (see [viewWorld.provenanceFor]).
		if !worldScopeFrom(r.Context()).IsTrivial() {
			v1Entity.World = worldProvenance(r.Context(), e)
		}
		data = append(data, v1Entity)
	}

	// Resolve includes for the whole page at once (TASK-550RD).
	if wantIncludes {
		included = h.resolveV1Includes(r.Context(), entities, includes)
	}

	if !h.rowExtras(w, r, query, entities, data) {
		return
	}

	resp := v1.ListResponse{
		Data: data,
		Meta: v1.ListMeta{
			Total:   total,
			Page:    page,
			PerPage: perPage,
			HasMore: end < total,
			// Relation order is shown only when it was applied, so the
			// client never has to re-derive the condition.
			RelationOrder: ordering.wire(),
		},
		// A list's toolbar buttons ride on the collection map (toolbaractions.go).
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

	// If includes were requested, add them to response
	if len(included) > 0 {
		// For list responses with includes, we need a different response structure
		// Encode as JSON with additional "included" field
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

//archplan:auth=authz
func (h *entityReadHandler) handleV1GetEntity(
	w http.ResponseWriter, r *http.Request, typeName, plural, entityID string,
) {
	ctx := r.Context()

	// The path segment is an ADDRESS — `ID` or `ID@face` (TKT-SLFURL). The
	// resolver applies PermitsRead BEFORE the store read, so a hidden id and
	// a nonexistent id spend the same MatchingFaces roundtrip — otherwise the
	// timing difference (in-memory lookup ~1µs vs. DB roundtrip ~1ms) is an
	// id-enumeration side channel that defeats the indistinguishable-404-body
	// invariant (RR-NGMI). Every miss, including a malformed address, a
	// denied face and a stored type other than the route's, is the same 404.
	//
	// A miss on an id a rename or conversion retired redirects a reader of
	// the successor there ([redirectRetired]); every other miss stays the
	// uniform 404. Only this GET redirects: a write to an old id must not
	// land on an entity the caller did not name.
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
	// Serializing evaluates the grant traversals several times (strip,
	// `_fields`, `_relations`); priming answers them once.
	ctx = primeVerdicts(ctx, h.fields(), []*entityPkg.Entity{entity})

	query := r.URL.Query()

	// Single per-entity serialization: strips hidden + attaches
	// `_fields` / `_relations` per docs/data-entry/api-reference.md.
	//
	// The edges OF THE FACE BEING SERVED, in every world including the
	// default one — see [servedFaceEdges]. Each link resolves through the
	// SAME world as the entry, per neighbor, independently (RULING 12): a
	// published view links to published faces, and a link whose head has no
	// face in this world is ABSENT rather than pointing at a 404.
	//
	// There is deliberately no default-world arm reaching the bare-id
	// reader. That arm returned the UNION of every face's content-scoped
	// edges, so a draft carried the published face's links too — the
	// mixed-face response that reads as correct because the entity looks
	// right and only its links are wrong.
	outgoing, visibleNeighbors, werr := h.faceEdges().read(ctx, entity)
	if werr != nil {
		// A neighbor-resolution fault is an infrastructure failure, not an
		// empty link set. Rendering it as "this world links to nothing"
		// would hide an outage behind a page that looks like a
		// correctly-sparse published face (RR-4TFZNL).
		writeGateError(w, r, werr)
		return
	}
	result := h.serializer().forWireScoped(ctx, entity, outgoing, visibleNeighbors, meta, plural)
	result.Versions = fieldVersionsOf(&result, meta)
	// Roll-ups after the field version tokens and outside the ETag: they are
	// computed from OTHER entities under the caller's grants, which neither
	// covers, and the response is no-store.
	h.rollups().add(ctx, entity.Type, []rollupRow{{ref: entity.Ref(), props: result.Properties}},
		worldScopeFrom(ctx), nil)

	// Face provenance (TKT-WRLDAPI item 2). Attached HERE rather than inside
	// forWire, even though forWire is the shared per-entity serializer,
	// because this is the only path whose Face is the STORE's answer to a
	// world query. Its other callers would produce a label the code cannot
	// back: the history handler serializes a snapshot built by
	// entityPkg.New(), whose Face is always zero, so a historical version
	// of a PUBLISHED face would report the default coordinate. An affordance
	// map that lies is a trap for every future consumer (Ruling 11); so is a
	// provenance block.
	//
	// An EXPLICIT address was not resolved by the world at all, so it is
	// labeled by [addressedProvenance] rather than by the chain position it
	// happens to hold — see that function for why.
	if isExplicitAddress(entityID) {
		result.World = addressedProvenance(ctx, entity)
	} else {
		result.World = worldProvenance(ctx, entity)
	}

	// Handle includes for related entities
	if includes := query.Get("include"); includes != "" {
		result.Included = h.resolveV1Includes(ctx, []*entityPkg.Entity{entity}, includes)
	}

	// Resolve the entity-ID code spans in the body so an edit form can render
	// them as titled links, the way the read surface already does
	// (`rewriteEntityRefToken`). Same call the view handler makes, so the two
	// surfaces cannot disagree about a title.
	//
	// It runs on the SERVED content, after the read gate and the redactor, so
	// a reference inside a body the caller could not read never gets scanned.
	// collectMentions then applies the gate a second time to each REFERENCED
	// entity, which is what keeps a title the caller may not see out of the
	// response (BUG-R9EHKV).
	//
	// Not folded into the ETag: mentions turn on OTHER entities' titles and on
	// the caller's own grants, neither of which this validator ever covered.
	// The response is `no-store` anyway, so a stale shared copy is not
	// reachable.
	result.Mentions = collectMentions(ctx, h.viewReader(), meta, entity.Content)

	// The advisory lease, after the read gate above and after the field
	// version tokens, so it moves neither. It is not in the ETag either: a
	// claim would then fail every If-Match the holder's client sends.
	result.Lease = h.leases().entityLease(ctx, entity.ID)

	// ETag for caching (visible-only path; deny-path above emits no ETag).
	//
	// The edges computed for the BODY above are reused rather than re-read.
	// Under a world, re-deriving them costs a second head-resolution query,
	// two more relation queries and a second ACL pass per request — the
	// RR-FRK1 duplication this PR is careful to avoid one function over — and
	// it also opens a window in which the validator could describe a
	// different edge set than the body it is validating.
	etag := servedETag(ctx, h.affordances(), entity, outgoing)
	w.Header().Set("ETag", etag)

	// Check If-None-Match. When leases are wired the GET never answers 304:
	// the ETag does not cover `_lease`, so a copy cached before a claim or a
	// release would carry the wrong lease. The ETag is still sent, unchanged
	// by the lease, so If-Match keeps working. A type with roll-ups never
	// answers 304 either, for the same reason: they change with OTHER
	// entities, which the ETag does not cover.
	if h.leases() == nil && len(meta.RollupNames(entity.Type)) == 0 &&
		r.Header.Get("If-None-Match") == etag {

		w.WriteHeader(http.StatusNotModified)
		return
	}

	writeV1JSON(w, http.StatusOK, result)
}

//archplan:auth=authz
func (h *entityReadHandler) handleV1EntityRelations(w http.ResponseWriter, r *http.Request, typeName, entityID string) {
	// The path segment is an ADDRESS — `ID` or `ID@face` (BUG-VFHUWO); the
	// face selects which tail's edges are served.
	//
	// ACL gate (TKT-VQGN CRIT-2): /relations on a hidden entity 404s
	// indistinguishably. Without the gate the endpoint confirms
	// existence (200 vs 404) AND leaks the full neighbor-id set —
	// closing one channel via /include filter while leaving this open
	// would defeat the per-entity-response invariant. The face gate matters
	// even though the response carries no body: a content-scoped relation
	// belongs to ONE face, so serving a withheld face's edges discloses its
	// structure (TKT-O7R2A1).
	entity, found := readAddressedOr404(w, r, h.visible(), typeName, entityID)
	if !found {
		return
	}
	ref := entity.Ref()
	s := h.schema()

	// Outgoing edges at the ADDRESSED TAIL: a content-scoped edge belongs to
	// one face of its source, so an unfiltered read returns the union of every
	// face's edges and presents another face's links as this one's. Incoming
	// edges point at the entity, since heads are faceless; one whose tail is a
	// face is served with that face named, if the caller may read it.
	outgoing := h.reader().outgoingRelationsOnFace(r.Context(), ref)
	incoming, err := readableIncoming(r.Context(), h.reader(), h.visible(), ref.ID)
	if err != nil {
		writeGateError(w, r, err)
		return
	}

	// Gate hidden neighbors (BUG-ABXMAV / RR-HJV8CP). Without this, a hidden
	// peer's `type` (via the ungated entityType read) and edge `meta` leak past
	// the source-only read gate. Mirror the list path (handleV1ListEntities):
	// compute the visible neighbor set in one type-batched pass and drop any
	// edge whose peer is not visible BEFORE reading its type. A visible neighbor
	// stays; a hidden one is dropped (no over-filtering).
	// TODO(BUG-ABXMAV): the "gate a set of neighbor edges" step is now
	// replicated here, in handleV1GetRelationType, and in the list path — a
	// shared chokepoint (P3) would collapse the three; deferred to avoid
	// churning the list path in a security fix.
	// Incoming edges are already gated, by ACL alone (readableIncoming).
	visibleNeighbors := visibleRelationIDs(r.Context(), h.visible(),
		neighborIDsOf(outgoing, nil))

	relations := make(map[string][]map[string]any)

	// Track the sort property per group, derived from the relation type's
	// Orderable mode. Empty string disables sorting for that group.
	groupSortProp := make(map[string]string)

	// pendingStrips defers per-edge relation-meta redaction (TKT-B1F5Q1) until
	// AFTER sortRelationGroup runs (see relationMetaStrip). Each strip also carries
	// the relation type so the redaction resolves the right grant block.
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
			// The relation grant lives on the SOURCE entity type (edge.From for an
			// incoming edge — the peer). Resolved fail-closed at strip time.
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

	// Sort each group by its managed order property; missing values last;
	// ties stable on insertion order.
	for groupName, prop := range groupSortProp {
		if prop == "" {
			continue
		}
		sortRelationGroup(relations[groupName], prop)
	}

	// Redact hidden relation meta AFTER sorting, through the shared chokepoint.
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

//archplan:auth=authz
func (h *entityReadHandler) handleV1GetRelationType(
	w http.ResponseWriter, r *http.Request, typeName, entityID, relType string,
) {
	// An ADDRESS, gated as in handleV1EntityRelations (BUG-VFHUWO).
	entity, found := readAddressedOr404(w, r, h.visible(), typeName, entityID)
	if !found {
		return
	}
	ref := entity.Ref()

	incoming := r.URL.Query().Get("direction") == string(DirectionIncoming)

	// Outgoing at the addressed tail, incoming at the entity: see
	// handleV1EntityRelations.
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

	// Gate hidden neighbors (BUG-ABXMAV / RR-HJV8CP): same rationale as
	// handleV1EntityRelations. Collect this relation-type's peers and compute
	// the visible set in one batched pass, then skip any hidden peer BEFORE
	// reading its type (so its type/meta never reach the wire). See the
	// shared-chokepoint TODO in handleV1EntityRelations.
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
		// Already gated, by ACL alone (readableIncoming).
		visibleNeighbors = make(map[string]bool, len(peerIDs))
		for _, id := range peerIDs {
			visibleNeighbors[id] = true
		}
	} else {
		visibleNeighbors = visibleRelationIDs(r.Context(), h.visible(), peerIDs)
	}

	relations, pendingStrips := h.buildRelationTypeRows(r.Context(), entity, edges, relType, incoming, visibleNeighbors)

	// Apply orderable sort when the type declares the relevant side.
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

	// Redact hidden relation meta after sorting (see relationMetaStrip).
	for _, ps := range pendingStrips {
		h.affordances().redactRelationMetaStrip(r.Context(), ps, entity, relType)
	}

	writeV1JSON(w, http.StatusOK, relations)
}

func (h *entityReadHandler) handleV1RelationTarget(
	w http.ResponseWriter, r *http.Request, typeName, entityID, relType, targetID string,
) {
	// The path segment is an ADDRESS (BUG-VFHUWO); every arm resolves it
	// through the visible reader before acting on a row.
	switch r.Method {
	case http.MethodGet:
		// Single-relation body read (RR-SYNCR1): meta + content +
		// _redacted + a relation-level ETag, dual-endpoint gated.
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

	// A `type` parameter also joins the query as a `type:` clause, so a
	// free-text search is scoped BEFORE its relevance cap rather than
	// filtered after it, where the cap could have spent every slot on
	// other types. Only a declared type name is spliced in; any other value
	// cannot match a row, which the exact filter below still ensures.
	typeFilter := r.URL.Query().Get("type")
	if _, declared := meta.Entities[typeFilter]; declared {
		query = "type:" + typeFilter + " " + query
	}

	// The read is gated (TKT-BA8BSX): only entities the request principal
	// may read come back, and gate/load/search failures surface instead of
	// silently truncating. The limit lowers the relevance cap where that
	// cannot change which rows survive the filters (TKT-T2I7P2, see
	// freeTextLimit), and a structured query reads only the rows it serves
	// (TKT-D3RAB0E, see searchPage). total counts the matches before the
	// cut.
	entities, total, err := h.queries().searchPage(r.Context(), query,
		queryOptions{limit: limit, exactType: typeFilter})
	if err != nil {
		writeListPipelineError(w, r, err)
		return
	}

	// Search rows are content-free (rowcontent.go); bodies are opt-in and
	// bounded by the search result cap.
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
		// includeRelations stays false on search results: the read gate
		// covers root entities only, and a relation map would expose
		// {ID, Title} of related entities this principal may not read.
		// Flipping this requires per-target gating first (RR-QO01XY) —
		// TestACLSearch_VisibleHitRelatedToHidden pins the invariant.
		row := h.serializer().forWireRelated(pageCtx, e, nil, nil, nil, meta, plural)
		// Same provenance a list row carries, and nil in the default world
		// for the same reason (see handleV1ListEntities).
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
			Page:    1,
			PerPage: len(data),
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
//
// Candidate collection is world-aware (TKT-WRLDAPI item 4): under a
// non-default world the neighbors come from the world-scoped seam, so an
// included peer is THAT world's face of the neighbor and a neighbor with no
// face in this world is absent. Under the default world it is the ungated
// reader, byte-identical to before.
//
// Everything after collection — the batched ACL gate, nested recursion,
// serialization — is shared by both paths deliberately. The world decides
// WHICH entities are candidates; it does not change how they are gated or
// rendered, and forking that would be two visibility rules to keep in step.
func (h *entityReadHandler) resolveV1Includes(
	ctx context.Context, rows []*entityPkg.Entity, includes string,
) map[string]v1.Entity {
	if len(rows) == 0 {
		return nil
	}
	s := h.schema()
	included := make(map[string]v1.Entity)

	// Collect candidate target entities first; filter via the ACL
	// gate per-type (batched), then serialize the survivors. The
	// two-phase shape exists to satisfy RR-M84L (the include channel
	// must respect the per-entity visibility rule) AND RR-FRK1 (a
	// hub entity with 50 neighbors must not cost 50 GraphCount
	// round-trips).
	//
	// nested pairs a target id with the remaining nested-include expression
	// (e.g. "implements.requires" → "requires" against the implements
	// target). Recursion happens after the visibility filter so hidden
	// neighbors don't trigger hidden nested probes.
	candidates, nested, err := includeCandidates(ctx, h.reader(), h.worldNeighbors(), rows, includes)
	if err != nil {
		// The include channel is a best-effort affordance (see
		// filterVisibleIncludes): the authoritative read is the explicit GET.
		// A world-resolution fault drops the block and logs rather than
		// failing the whole response — but it must not be silent, or
		// an outage reads as "this entity has no neighbors in this world".
		slog.Warn("dataentry: resolving world-scoped includes failed; include block omitted for every row",
			"first_entity", rows[0].ID, "rows", len(rows), "include", includes, "err", err)
		return nil
	}

	// One entry per id, the LAST candidate winning — the map assignment the
	// per-row loop made. A neighbor shared by many rows is serialized once.
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
		// Per-neighbor provenance (QA F-6b). Each included target was resolved
		// through the world INDEPENDENTLY of its source, so one entity's
		// neighbors can mix a chain hit with a stand-in — and a client cannot
		// tell which without being told. The detail view badges exactly this:
		// "the link leads somewhere English", which the payload previously had
		// no way to say.
		//
		// Same helper the entity root and list rows use, so the three surfaces
		// cannot disagree about what a face resolution was.
		wired.World = worldProvenance(ctx, target)
		included[target.ID] = wired
	}

	// Nested includes: one batch per distinct expression, over the visible
	// targets that carry it.
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
//
// Both branches end gated: the list pipeline resolves the read scope
// internally, and executeQuery gates the search pipeline internally
// since TKT-BA8BSX (it routes through search.VisibleSearcher; the
// external readableSubset patch this branch used in the interim is
// retired). Without the gate a denied principal reads hidden
// cardinality from Total and harvests hidden {ID, Type} pairs from
// prev/next — the exact leak shape TKT-VMD8 closes on the list path
// (CRIT finding, TKT-VMD8 review).
//
// A pile scope is the pile's readable items in the request's world, newest
// first, exactly the rows GET /_piles/{id} lists. Another user's pile is
// [piles.ErrNotFound], like a missing one.
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
