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
