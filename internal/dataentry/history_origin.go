package dataentry

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// originWire renders a version's provenance for the API, or nil for a direct
// edit.
//
// A direct edit is an OMITTED key — not a null and not `"kind": "manual"`.
// The absence IS the signal, and the version already carries `principal`
// naming the human who made it, so a reader distinguishes
// "edith@example.com · data-entry" (typed by hand) from the same principal
// with an `origin` block (produced by a copy) without a redundant marker.
//
// visibleSource is the already-gated source label ("" when the reader may not
// know the source exists); see gateOriginSources for why that gating exists.
func originWire(o store.Origin, visibleSource string) map[string]any {
	if o.IsZero() {
		return nil
	}
	m := map[string]any{"kind": string(o.Kind)}
	if visibleSource != "" {
		m["source"] = visibleSource
	}
	if o.Definition != "" {
		m["definition"] = o.Definition
	}
	return m
}

// gateOriginSources resolves which version origins may name their source entity,
// returning source labels keyed by meta index; an absent index renders no
// source. origin_source is an entity id, and entity existence is secret, so an
// ungated cross-entity copy would be an existence oracle. Definition name and
// origin kind are config and are not gated. Probes are batched per source type
// via ReadableFacesMany. A probe error is treated as deny.
func gateOriginSources(ctx context.Context, gate readGate, metas []store.VersionMeta) map[int]string {
	byType := map[string][]string{}
	for _, m := range metas {
		if m.Origin.Source == "" || m.Origin.SourceType == "" {
			continue
		}
		byType[m.Origin.SourceType] = append(byType[m.Origin.SourceType], m.Origin.Source)
	}
	if len(byType) == 0 {
		return nil
	}

	allowed := map[string]acl.FaceVerdicts{}
	for typ, ids := range byType {
		verdicts, err := gate.ReadableFacesMany(ctx, typ, ids)
		if err != nil {
			continue
		}
		allowed[typ] = verdicts
	}

	out := map[int]string{}
	for i, m := range metas {
		o := m.Origin
		if o.Source == "" || o.SourceType == "" {
			continue
		}
		verdicts, ok := allowed[o.SourceType]
		if !ok {
			continue
		}
		verdict := verdicts.For(o.Source)
		if verdict.None() || (o.SourceFace != "" && !verdict.Contains(entityPkg.Face(o.SourceFace))) {
			continue
		}
		out[i] = o.SourceLabel()
	}
	return out
}
