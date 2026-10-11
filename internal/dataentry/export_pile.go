package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/transform"
)

// exportPile answers GET /_piles/{id}/_export?transform=<name>: the pile's
// readable items as a table, converted by a registered transform.
//
// It is downstream of the pile read, never a wider one: the rows are
// exactly the ones GET /_piles/{id} lists, already gated and redacted once
// by the pile resolver. The download is hardened like every export.
func (h *pilesHandler) exportPile(w http.ResponseWriter, r *http.Request, id string) {
	ctx := r.Context()
	p, err := h.svc.Get(ctx, id)
	if err != nil {
		writePilesError(w, r, err)
		return
	}
	export := h.export()
	name, reg, ok := export.resolveTransform(w, r)
	if !ok {
		return
	}
	// `piles.export:` narrows the registry. Config is not a secret, but the
	// same unknown_transform answer keeps one meaning for "not offered here".
	if pc := h.cfg().Piles; pc != nil && len(pc.Export) > 0 && !slices.Contains(pc.Export, name) {
		writeV1Error(w, r, http.StatusNotFound, "unknown_transform",
			"Unknown transform", "no such export format is configured")
		return
	}
	rows, err := h.readableItems(ctx, p)
	if err != nil {
		writeGateError(w, r, err)
		return
	}
	renderer := pileTableRenderer(h.meta(), pileEntities(rows))
	export.convertAndWrite(w, r, reg, name, renderer, "pile-"+p.ID, "pile", p.ID)
}

// pileTableRenderer emits the pile as a markdown table of address, type and
// title.
//
// It does not reuse listTableRenderer: that renders a list's columns of ONE
// type, and its cell lookup has no column for the entity type, which a pile
// of mixed types needs. A pile has no relation columns, so the batched
// neighbor resolution listTableRenderer exists for has nothing to do here.
func pileTableRenderer(meta *metamodel.Metamodel, rows []*entityPkg.Entity) transform.Renderer {
	return transform.RendererFunc(func(context.Context) ([]byte, error) {
		var b strings.Builder
		b.WriteString("| ID | Type | Title |\n| --- | --- | --- |\n")
		for _, e := range rows {
			fmt.Fprintf(&b, "| %s | %s | %s |\n",
				transform.EscapeInline(entityPkg.FormatStateRef(e.ID, e.Face)),
				transform.EscapeInline(typeLabel(meta, e.Type)),
				transform.EscapeInline(safeDisplayTitle(meta, e)))
		}
		return []byte(b.String()), nil
	})
}

// typeLabel is the declared label of an entity type, or its name.
func typeLabel(meta *metamodel.Metamodel, typ string) string {
	if def, ok := meta.Entities[typ]; ok && def.Label != "" {
		return def.Label
	}
	return typ
}
