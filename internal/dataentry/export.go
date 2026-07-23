package dataentry

import (
	"context"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"sort"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/transform"
)

// transformInfo is the wire shape for GET /api/v1/_transforms: the export
// formats a client can offer, derived from the metamodel `transforms:` registry
// (markdown-input transforms only).
type transformInfo struct {
	Name     string `json:"name"`
	Produces string `json:"produces"`
}

// handleV1Transforms serves GET /api/v1/_transforms — the list of registered
// export formats. It is public metadata (which formats exist), carries no
// entity data, and drives the SPA "Export as" menu.
func (a *App) handleV1Transforms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	reg := transform.RegistryFromMetamodel(a.Meta())
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
func (a *App) handleV1ExportEntity(w http.ResponseWriter, r *http.Request, typeName, entityID string) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	ctx := r.Context()

	name := r.URL.Query().Get("transform")
	if name == "" {
		writeV1Error(w, r, http.StatusBadRequest, "missing_transform",
			"A transform is required", "pass ?transform=<name>")
		return
	}

	reg := transform.RegistryFromMetamodel(a.Meta())
	if _, ok := reg[name]; !ok {
		writeV1Error(w, r, http.StatusNotFound, "unknown_transform",
			"Unknown transform", "no such export format is configured")
		return
	}

	// ACL gate BEFORE any render (same as handleV1GetEntity): a deny is an
	// indistinguishable 404, and the render never runs for a hidden entity.
	entity, found, err := a.visibleReader.getVisible(ctx, typeName, entityID)
	if err != nil {
		writeGateError(w, r, err)
		return
	}
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}

	renderer := transform.EntityRenderer{
		Entity:    entity,
		Meta:      a.Meta(),
		Relations: a.entityRelationGroups(ctx, entity),
	}

	eng, err := transform.NewEngine(reg)
	if err != nil {
		slog.Warn("dataentry: build transform engine failed", "err", err)
		writeV1Error(w, r, http.StatusInternalServerError, "export_failed",
			"Export failed", "check server logs")
		return
	}
	res, err := eng.Run(ctx, name, renderer)
	if err != nil {
		var unknown transform.UnknownTransformError
		if errors.As(err, &unknown) {
			writeV1Error(w, r, http.StatusNotFound, "unknown_transform", "Unknown transform", "")
			return
		}
		// A transform failure (missing binary, non-zero exit, timeout) is a
		// server/config problem, not caller input. Don't leak the command line.
		slog.Warn("dataentry: transform export failed",
			"err", err, "entity", entityID, "transform", name)
		writeV1Error(w, r, http.StatusInternalServerError, "export_failed",
			"Export failed", "check server logs")
		return
	}

	writeExportResponse(w, res, exportFilename(entityID, res.Produces))
}

// writeExportResponse writes converted bytes as a hardened forced download.
// Mirrors handleV1GetAttachment: force download, block sniffing, sandbox active
// content, and never cache (the bytes are per-request and may embed user data).
func writeExportResponse(w http.ResponseWriter, res transform.Result, filename string) {
	hdr := w.Header()
	hdr.Set("Content-Type", res.Produces)
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	hdr.Set("Cache-Control", "no-store")
	hdr.Set("Content-Disposition", `attachment; filename="`+safeAttachmentFilename(filename)+`"`)
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
func (a *App) entityRelationGroups(ctx context.Context, e *entityPkg.Entity) []transform.RelationGroup {
	meta := a.Meta()
	outgoing := a.reader.outgoingRelations(ctx, e.ID)
	incoming := a.reader.incomingRelations(ctx, e.ID)

	neighborIDs := neighborIDsOf(outgoing, incoming)
	visible := visibleRelationIDs(ctx, a.reader, a.visibleReader, neighborIDs)

	// label -> ordered neighbor titles.
	byLabel := map[string][]string{}
	addNeighbor := func(label, neighborID string) {
		if !visible[neighborID] {
			return
		}
		title := neighborID
		if node, ok := a.reader.getEntity(ctx, neighborID); ok {
			title = displayTitleOrTitle(meta, node)
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

// displayTitleOrTitle resolves an entity's human title: metamodel DisplayTitle
// (whose fallback is the id), then the raw title property, then the id.
func displayTitleOrTitle(meta *metamodel.Metamodel, e *entityPkg.Entity) string {
	if t := meta.DisplayTitle(e.ID, e.Type, e.Properties); t != "" && t != e.ID {
		return t
	}
	if t := e.Title(); t != "" {
		return t
	}
	return e.ID
}
