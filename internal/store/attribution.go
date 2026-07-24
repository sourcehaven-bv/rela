package store

import "context"

// Attribution names who/what to attribute a write to. It is the store's OWN
// notion of authorship — deliberately not principal.Principal: the store never
// learns the Principal (no roles, no assertion claims, no identity semantics),
// only the two opaque labels a versioning backend needs to answer "who last
// edited this row".
//
// Attribution reaches the store the same way version-capture attribution does:
// populated from the request principal at the entitymanager write boundary,
// never read from any other source inside a store implementation. The ctx
// carrier below is the second sanctioned boundary-populated attribution input
// (the first being the PrincipalUser/PrincipalTool fields on VersionInput /
// RelationVersionInput).
//
// Absent attribution is a valid, expected state — a write whose ctx carries no
// Attribution (background jobs, unstamped principals, tests) MUST be persisted
// as NULL/unknown authorship, never defaulted to a made-up identity. The
// version sweep then falls back to its system principal (tool
// "version-sweep") for rows with no recorded editor. A PARTIALLY unknown
// identity is different: the boundary forwards it verbatim (e.g. a CLI write
// under an unset $USER stores user "unknown" with the real tool), because the
// known component is real information. TKT-ZIRMGM / RR-2VWA0Q / RR-5JIN8U.
type Attribution struct {
	User string
	Tool string
}

// IsZero reports whether a carries no authorship information at all.
func (a Attribution) IsZero() bool {
	return a.User == "" && a.Tool == ""
}

type attributionKey struct{}

// WithAttribution returns a ctx carrying a as the write-authorship attribution
// for store writes. Called only at the entitymanager write boundary, and only
// with a real identity — an unknown or zero principal must NOT be translated
// into an Attribution (RR-U964M0: stamping a literal "unknown" would defeat
// the NULL-means-unknown contract and the sweep's system-principal fallback).
func WithAttribution(ctx context.Context, a Attribution) context.Context {
	return context.WithValue(ctx, attributionKey{}, a)
}

// AttributionFrom returns the Attribution carried by ctx, or the zero
// Attribution when none was set. It never fabricates a default: absent
// attribution must stay absent so backends persist NULL.
func AttributionFrom(ctx context.Context) Attribution {
	a, _ := ctx.Value(attributionKey{}).(Attribution)
	return a
}
