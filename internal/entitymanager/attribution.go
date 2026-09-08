package entitymanager

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// withStoreAttribution translates the request principal into the store's
// write-authorship attribution (store.WithAttribution) at the write boundary.
// Every public write entry point calls this first, so nested store writes —
// automations, cascades, managed-order renumbering — inherit the attribution
// of the human intent that triggered them, mirroring how the audit log
// attributes those same writes.
//
// Only a REAL identity is forwarded: a zero principal, or the {unknown,
// unknown} fallback principal.From returns for an unstamped ctx, leaves the
// ctx unchanged so backends persist NULL authorship and the version sweep
// falls back to its system principal (RR-U964M0 — a literal "unknown" in the
// columns would defeat that fallback).
func withStoreAttribution(ctx context.Context) context.Context {
	p := principal.From(ctx)
	if p.IsZero() || (p.User == "unknown" && p.Tool == "unknown") {
		return ctx
	}
	return store.WithAttribution(ctx, store.Attribution{User: p.User, Tool: p.Tool})
}

// withCopyOrigin marks ctx as producing writes from a declared copy
// definition, so the store stamps provenance on the rows the copy writes and
// the version sweep can later record "this version was copied from X".
//
// It is the copy kernel's counterpart to withStoreAttribution and reaches the
// store the same single sanctioned way — boundary-populated, carried on ctx.
// There is no `withManualOrigin`: a direct edit is the ABSENCE of this call,
// which is why nothing else in the write path needs to opt out.
// sourceType is recorded so a read-out path can gate the source id against
// the reader's own read verdict — an entity id is row-level secret and the ACL
// probe is keyed by (type, id).
//
// sourceFace is the DECLARED face name, never the stored coordinate. The two
// differ for exactly one face per type — the one named by `bare_face`, whose
// stored coordinate is the empty string — and that is the whole of the bug
// this signature guards against: a copy declared `from: policy@draft` on a
// type with `bare_face: draft` would otherwise record an empty face and read
// back as a bare `POL-4`, dropping the one fact the annotation carries. The
// caller resolves it through metamodel.DeclaredFace; see store.Origin.
// SourceFace for why provenance holds a name rather than a coordinate.
func withCopyOrigin(
	ctx context.Context, definition, sourceID, sourceType, sourceFace string,
) context.Context {
	return store.WithOrigin(ctx, store.Origin{
		Kind:       store.OriginCopy,
		Source:     sourceID,
		SourceFace: sourceFace,
		SourceType: sourceType,
		Definition: definition,
	})
}
