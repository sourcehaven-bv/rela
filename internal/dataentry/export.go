package dataentry

import (
	"context"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"sort"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/transform"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// exportHandler owns the view-export routes (transform list, entity export, list
// export) and their markdown-rendering helpers. Extracted from App (keeps App
// under its plimsoll method cap); constructed once in NewApp with closures over
// the App collaborators it needs. The ACL-coupled reads flow through the same
// seams the rest of dataentry uses (reader / visibleReader / gateReadOrNotFound /
// scopedSortedEntities), so export can never widen past an authorized view.
type exportHandler struct {
	meta            func() *metamodel.Metamodel
	cfg             func() *Config
	reader          entityReader
	documents       *documentService
	scopedEntities  func(ctx context.Context, typeName string, query map[string][]string) ([]*entityPkg.Entity, error)
	findListForType func(entityType string) string

	// visReader is the row-gating + field-redacting read seam (DEC-ZBI39P):
	// entity export reads through Get (which owns the stored-type check,
	// RR-SRZK6X) and list-export rows through Filter, so a hidden field can
	// never reach the markdown handed to a transform. redactor is the same
	// field-verdict source exposed directly, for redacting already-gated
	// neighbor entities before title derivation (visibility.Redact).
	visReader visibility.Reader
	redactor  visibility.FieldRedactor

	// visibleReader remains for the batched neighbor-ID gate
	// (visibleRelationIDs) shared with the serializer paths.
	visibleReader visibleReader

	// engine is the SHARED transform engine. Sharing is load-bearing, not an
	// optimisation: the engine owns the bounded worker pool that caps concurrent
	// converter processes, so a per-request engine would give every request its
	// own pool and bound nothing. The engine holds no registry — each request
	// passes the current one to Run — so a metamodel live-reload needs no
	// rebuild.
	engine *transform.Engine
}

// newExportHandler builds the export handler with closures over the App
// collaborators it needs. Called from both NewApp and the test app builder so
// the wiring lives in one place.
func newExportHandler(app *App) (*exportHandler, error) {
	redactor := app.redactor()
	visReader, err := visibility.NewPolicyReader(ctxRowGate{}, redactor, app.store)
	if err != nil {
		return nil, fmt.Errorf("dataentry: newExportHandler: %w", err)
	}
	return &exportHandler{
		meta:           app.Meta,
		cfg:            func() *Config { return app.State().Cfg },
		reader:         app.reader,
		visibleReader:  app.visibleReader,
		visReader:      visReader,
		redactor:       redactor,
		documents:      app.documents,
		scopedEntities: app.scopedSortedEntities,
		findListForType: func(entityType string) string {
			s := app.State()
			return app.findListByEntityType(s, s.Cfg.Navigation, entityType)
		},
		engine: transform.NewEngine(),
	}, nil
}

// transformInfo is the wire shape for GET /api/v1/_transforms: the export
// formats a client can offer, derived from the metamodel `transforms:` registry
// (markdown-input transforms only).
type transformInfo struct {
	Name     string `json:"name"`
	Produces string `json:"produces"`
}

// probeTransforms checks, at startup, that every registered transform's command
// binary is resolvable on PATH, warning (never failing) for any that are
// missing — so an operator learns a typo or an uninstalled converter at boot
// rather than on the first export. Mirrors probeAttachmentCommands.
func (h *exportHandler) probeTransforms() {
	for name, perr := range h.engine.Probe(transform.RegistryFromMetamodel(h.meta())) {
		if perr != nil {
			slog.Warn("transforms: configured command not found", "transform", name, "err", perr)
		}
	}
}

// handleV1Transforms serves GET /api/v1/_transforms — the list of registered
// export formats. It is public metadata (which formats exist), carries no
// entity data, and drives the SPA "Export as" menu.
func (h *exportHandler) handleV1Transforms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	reg := transform.RegistryFromMetamodel(h.meta())
	list := make([]transformInfo, 0, len(reg))
	for _, nd := range reg.FromMarkdown() {
		list = append(list, transformInfo{Name: nd.Name, Produces: nd.Def.Produces})
	}
	writeV1JSON(w, http.StatusOK, list)
}

// handleV1ExportEntity serves GET /api/v1/{plural}/{id}/_export?transform=<name>.
//
// It is a READ affordance downstream of the same ACL gate as the entity view:
// the entity is resolved via visibleReader.getVisible (404 on deny,
// indistinguishable from a real miss), then rendered to markdown and converted
// by the named transform. A request may only choose a registered transform name
// — never a command, flag, or path. The response is hardened like an attachment
// download (nosniff + sandbox CSP + no-store + sanitized filename) because a
// transform emits attacker-influenceable bytes (a PDF/DOCX built from user
// content).
func (h *exportHandler) handleV1ExportEntity(w http.ResponseWriter, r *http.Request, typeName, entityID string) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	ctx := r.Context()

	name, reg, ok := h.resolveTransform(w, r)
	if !ok {
		return
	}

	// ACL gate BEFORE any render (same as handleV1GetEntity): a deny is an
	// indistinguishable 404, and the render never runs for a hidden entity.
	// visReader.Get also owns the stored-type check (RR-SRZK6X) and returns
	// a FIELD-REDACTED copy — the renderer below can never see a property
	// the caller's `visible:` policy hides (the #1188 IB-review finding).
	entity, found, err := h.visReader.Get(ctx, typeName, entityID)
	if err != nil {
		writeGateError(w, r, err)
		return
	}
	if !found {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}

	// A per-type `export_render:` view config (RR-BM0KIJ: config-selected,
	// never request-selected — no query param chooses the script) renders via
	// the operator's Lua document instead of the built-in entity renderer. It
	// routes through the SAME document machinery handleV1Documents uses, so it
	// can never become an unauthorized Lua-on-read surface (RR-8C23IL).
	// No override configured → built-in renderer.
	renderer, ok := h.exportRenderer(w, r, typeName, entityID, entity)
	if !ok {
		return // exportRenderer already wrote the error/404
	}

	h.convertAndWrite(w, r, reg, name, renderer, entityID, "entity", entityID)
}

// resolveTransform parses ?transform=<name> and validates it against the
// current registry, writing the 400/404 response itself on failure. The
// returned registry is the one the name was validated against; pass it to the
// engine so validation and execution cannot see different registries.
func (h *exportHandler) resolveTransform(
	w http.ResponseWriter, r *http.Request,
) (name string, reg transform.Registry, ok bool) {
	name = r.URL.Query().Get("transform")
	if name == "" {
		writeV1Error(w, r, http.StatusBadRequest, "missing_transform",
			"A transform is required", "pass ?transform=<name>")
		return "", nil, false
	}
	reg = transform.RegistryFromMetamodel(h.meta())
	if _, exists := reg[name]; !exists {
		writeV1Error(w, r, http.StatusNotFound, "unknown_transform",
			"Unknown transform", "no such export format is configured")
		return "", nil, false
	}
	return name, reg, true
}

// convertAndWrite runs renderer through the shared engine and writes the
// hardened download response. Any failure maps to a logged 500: a transform
// failure (missing binary, non-zero exit, timeout) is a server/config problem,
// not caller input — don't leak the command line. logAttrs adds handler-specific
// log context ("entity", id / "type", name).
func (h *exportHandler) convertAndWrite(
	w http.ResponseWriter, r *http.Request,
	reg transform.Registry, name string, renderer transform.Renderer, baseName string,
	logAttrs ...any,
) {
	res, err := h.engine.Run(r.Context(), reg, name, renderer)
	if err != nil {
		slog.Warn("dataentry: transform export failed",
			append([]any{"err", err, "transform", name}, logAttrs...)...)
		writeV1Error(w, r, http.StatusInternalServerError, "export_failed",
			"Export failed", "check server logs")
		return
	}
	writeExportResponse(w, res, exportFilename(baseName, res.Produces))
}

// exportRenderer selects the markdown renderer for an entity export. When the
// entity type configures an `export_render:` Lua script (in EntityViews), the
// export routes through that script — the per-type render OVERRIDE — so
// exporting an entity of that type automatically uses the operator's custom
// document instead of the built-in property renderer. Otherwise the built-in
// [transform.EntityRenderer] is used.
//
// The entity was already resolved through the ACL read gate (getVisible on
// typeName) in the caller, so the override runs only for an entity the caller
// may read; the script is a fixed config value (not request input) and receives
// the already-validated entityID. On any failure it writes the response and
// returns ok=false.
func (h *exportHandler) exportRenderer(
	w http.ResponseWriter, r *http.Request, typeName, entityID string, entity *entityPkg.Entity,
) (transform.Renderer, bool) {
	script := h.exportRenderScriptFor(typeName)
	if script == "" {
		return transform.EntityRenderer{
			Entity:    entity,
			Meta:      h.meta(),
			Relations: h.entityRelationGroups(r.Context(), entity),
		}, true
	}

	// Defense-in-depth: the entityID reaches the document cache filename and (for
	// a future command override) an sh -c {id}. It is already an existing entity
	// id (getVisible matched it), but validate it the same way handleV1Documents
	// does before any render.
	if !isSafePathSegment(entityID) {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_entity", "Invalid entity id", "")
		return nil, false
	}

	// A render config for the type's override script. ConfigID is a synthetic,
	// per-type identity ("export:<type>") so it never collides with a documents:
	// entry in the singleflight/cache key.
	renderCfg := documentRenderConfig{ConfigID: "export:" + typeName, Script: script}
	return transform.RendererFunc(func(ctx context.Context) ([]byte, error) {
		md, err := h.documents.RenderMarkdown(ctx, entityID, renderCfg)
		if err != nil {
			return nil, err
		}
		return []byte(md), nil
	}), true
}

// exportRenderScriptFor returns the configured per-type export render override
// script (the entity type's view.ExportRender), or "" when the type uses the
// built-in renderer.
func (h *exportHandler) exportRenderScriptFor(typeName string) string {
	if v, ok := findViewByEntityType(h.cfg().Views, typeName); ok {
		return v.ExportRender
	}
	return ""
}

// writeExportResponse writes converted bytes as a hardened forced download
// (the shared header block attachment downloads use), plus no-store because the
// bytes are per-request and may embed user data.
func writeExportResponse(w http.ResponseWriter, res transform.Result, filename string) {
	setHardenedDownloadHeaders(w.Header(), res.Produces, filename)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(res.Data); err != nil {
		slog.Warn("dataentry: streaming export failed", "err", err)
	}
}

// exportFilename builds a download name from the (sanitized-downstream) entity
// id and an extension inferred from the produced content-type. Falls back to a
// bare id when no extension is known.
func exportFilename(entityID, produces string) string {
	if exts, err := mime.ExtensionsByType(produces); err == nil && len(exts) > 0 {
		return entityID + exts[0]
	}
	return entityID
}

// entityRelationGroups resolves an entity's outgoing and incoming relations into
// display groups (relation label + VISIBLE neighbor titles) for the entity
// renderer. Neighbor visibility is gated through visibleRelationIDs so a hidden
// neighbor's title never leaks into the export (the same gate the list/serializer
// paths apply). Grouped by relation display label, in label order.
func (h *exportHandler) entityRelationGroups(ctx context.Context, e *entityPkg.Entity) []transform.RelationGroup {
	meta := h.meta()
	outgoing := h.reader.outgoingRelations(ctx, e.ID)
	incoming := h.reader.incomingRelations(ctx, e.ID)

	neighborIDs := neighborIDsOf(outgoing, incoming)
	visible := visibleRelationIDs(ctx, h.reader, h.visibleReader, neighborIDs)

	// label -> ordered neighbor titles.
	byLabel := map[string][]string{}
	addNeighbor := func(label, neighborID string) {
		if !visible[neighborID] {
			return
		}
		title := neighborID
		if node, ok := h.reader.getEntity(ctx, neighborID); ok {
			// Redact BEFORE deriving the title: a visible neighbor whose
			// display property is hidden must render as its ID, never the
			// hidden value (the RR-5N4K35 title-leak class).
			title = transform.DisplayTitle(meta, visibility.Redact(ctx, h.redactor, node))
		}
		byLabel[label] = append(byLabel[label], title)
	}

	for _, rel := range outgoing {
		addNeighbor(relationDisplayLabel(meta, rel.Type, false), rel.To)
	}
	for _, rel := range incoming {
		addNeighbor(relationDisplayLabel(meta, rel.Type, true), rel.From)
	}

	labels := make([]string, 0, len(byLabel))
	for l := range byLabel {
		labels = append(labels, l)
	}
	sort.Strings(labels)

	groups := make([]transform.RelationGroup, 0, len(labels))
	for _, l := range labels {
		groups = append(groups, transform.RelationGroup{Label: l, Neighbors: byLabel[l]})
	}
	return groups
}

// relationDisplayLabel returns a human label for a relation type in the given
// direction: the relation's Label (outgoing) or its inverse label (incoming),
// falling back to the raw type / inverse key.
func relationDisplayLabel(meta *metamodel.Metamodel, relType string, incoming bool) string {
	def, ok := meta.GetRelationDef(relType)
	if !ok {
		return relType
	}
	if incoming {
		if def.Inverse != nil && def.Inverse.Label != "" {
			return def.Inverse.Label
		}
		return inverseRelationKey(relType, *def)
	}
	if def.Label != "" {
		return def.Label
	}
	return relType
}
