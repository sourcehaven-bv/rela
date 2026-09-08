package dataentry

import (
	"context"

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
	// The copy DEFINITION name is operator-authored configuration, not data
	// (CLAUDE.md: "the configuration is not a secret; the data is"), so it is
	// served ungated — unlike the source id, which is a row.
	if o.Definition != "" {
		m["definition"] = o.Definition
	}
	return m
}

// gateOriginSources resolves which version origins may name their source
// entity, returning source labels keyed by the index of the meta they belong
// to. An index absent from the result must render no source.
//
// # Why a gate at all
//
// origin_source is an ENTITY ID, and whether an entity exists is a genuine
// secret under the row-level read rule (a denied GET is indistinguishable
// from a 404). A CROSS-ENTITY copy names a different entity, so echoing it
// unconditionally would turn the history endpoint into an existence oracle
// for any id a reader can get copied into something they can read. That is
// the reason store.Origin carries SourceType at all: the ACL probe is keyed
// by (type, id), so the type has to be recorded at write time to be gateable
// at read time.
//
// The definition name and the origin KIND are not gated — they are
// operator-authored configuration, which is not confidential.
//
// # Batched, not per row
//
// Probes are grouped by source type and issued through PermitsReadMany, so a
// long timeline costs one probe per distinct type rather than one per version.
// A probe error is treated as DENY (the label is withheld) rather than
// surfaced: failing closed here loses a decoration, while failing open leaks
// the existence of a row.
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

	allowed := map[string]map[string]bool{}
	for typ, ids := range byType {
		verdicts, err := gate.PermitsReadMany(ctx, typ, ids)
		if err != nil {
			// Fail closed: no verdicts for this type means no source labels
			// for it. See the godoc.
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
		if allowed[o.SourceType][o.Source] {
			out[i] = o.SourceLabel()
		}
	}
	return out
}
