package dataentry

import (
	"net/http"
	"strings"
)

// serveAsset serves any file from the operator's custom/ directory at
// /_custom/<path>.
//
// Every failure is a uniform 404 — a missing file, a directory, an oversize
// file, a dot-prefixed path and a traversal attempt are indistinguishable. That
// is not a confidentiality measure (which files exist under custom/ is operator
// config, not entity data, and the root CLAUDE.md rule says config is not a
// secret); there is simply no useful distinction to draw. The oversize case
// additionally logs server-side, since a silent 404 for a too-large image is a
// genuinely confusing operator experience.
//
// NOT gated on the injection flag: disable_custom_injection only suppresses the
// shell references, so an operator can still fetch files directly to check what
// is being served.
//
// There is deliberately NO index resolution: /_custom/fonts/ must 404, not
// serve fonts/index.html. "Serve arbitrary files from a directory" is the exact
// phrasing that invites someone to add it later.
func (c *customAssets) serveAsset(w http.ResponseWriter, r *http.Request) {
	entry := strings.TrimPrefix(r.URL.Path, customURLPrefix)

	body, err := openCustomEntry(c.projectRoot, entry)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	h := w.Header()
	// Extension-based, from the same fixed map apps/ uses: deterministic across
	// deploy boxes (unlike mime.TypeByExtension), unknown → octet-stream, which
	// a browser will neither execute nor render.
	h.Set("Content-Type", appEntryContentType(entry))
	// The bytes are operator-authored and may be executable; never let a
	// browser sniff a different type out of them.
	h.Set("X-Content-Type-Options", "nosniff")
	// These URLs are NOT content-hashed — /_custom/logo.svg is stable forever —
	// so heuristic caching would serve a stale asset after an operator edits it
	// with no way to bust it. no-cache means "revalidate", not "don't store".
	//
	// Known gap (RR-DR-ETAG): there is no ETag/Last-Modified, so a static
	// webfont re-transfers in full on every navigation. RR-CR-ETAG deferred
	// this when the only outputs were a 3.4KB shell and two text files; that
	// rationale does not carry to a 200KB font, and the fix (http.ServeContent)
	// would bring conditional requests and Range in one call.
	h.Set("Cache-Control", "no-cache")

	// #nosec G705 -- not an HTML sink in the XSS sense. The bytes are
	// operator-authored by design: this is the documented "custom.js is fully
	// trusted" contract (see custom.go). Serving them adds no capability the
	// operator does not already have — custom.js is injected into the SPA's own
	// document, same-origin with no CSP, so it can already reach every API
	// endpoint with the caller's session. Escaping operator CSS or JS would
	// break the feature rather than protect anything.
	//
	// NOTE: the pre-TKT-IWMETE version of this comment claimed the boundary was
	// "the two-name allowlist, which decides WHICH file may be read". That
	// allowlist is gone. The boundary is now validCustomEntry + the nested
	// os.OpenRoot containment in openCustomEntry.
	_, _ = w.Write(body)
}
