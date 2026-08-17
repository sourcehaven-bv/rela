package dataentry

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
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
//
// Delivery goes through http.ServeContent (TKT-IWMETE follow-up), which gives
// conditional requests, Range and correct HEAD semantics in one call. Before
// this, a 200KB webfont re-transferred in full on every navigation and a
// non-GET method still received a body.
func (c *customAssets) serveAsset(w http.ResponseWriter, r *http.Request) {
	entry := strings.TrimPrefix(r.URL.Path, customURLPrefix)

	f, err := openCustomEntryFile(c.projectRoot, entry)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	h := w.Header()
	// Extension-based, from the same fixed map apps/ uses: deterministic across
	// deploy boxes (unlike mime.TypeByExtension), unknown → octet-stream, which
	// a browser will neither execute nor render.
	//
	// Set explicitly because ServeContent would otherwise sniff the first 512
	// bytes when it cannot infer a type from the name — the exact behavior
	// nosniff exists to prevent.
	h.Set("Content-Type", appEntryContentType(entry))
	// The bytes are operator-authored and may be executable; never let a
	// browser sniff a different type out of them.
	h.Set("X-Content-Type-Options", "nosniff")
	// These URLs are NOT content-hashed — /_custom/logo.svg is stable forever —
	// so heuristic caching would serve a stale asset after an operator edits it
	// with no way to bust it. no-cache means "revalidate, then reuse if
	// unchanged", which is exactly right now that there is an ETag to
	// revalidate against: the browser still asks every time, but an unmodified
	// asset comes back as a bodiless 304.
	h.Set("Cache-Control", "no-cache")
	h.Set("ETag", customEntryETag(f.ModTime.UnixNano(), f.Size))

	// #nosec G705 -- not an HTML sink in the XSS sense. The bytes are
	// operator-authored by design: this is the documented "custom.js is fully
	// trusted" contract (see custom.go). Serving them adds no capability the
	// operator does not already have — custom.js is injected into the SPA's own
	// document, same-origin with no CSP, so it can already reach every API
	// endpoint with the caller's session. Escaping operator CSS or JS would
	// break the feature rather than protect anything.
	//
	// The containment boundary is validCustomEntry + the nested os.OpenRoot in
	// openCustomEntryFile — NOT a filename allowlist, which TKT-IWMETE removed.
	http.ServeContent(w, r, entry, f.ModTime, f.File)
}

// customEntryETag derives a strong ETag from an entry's modtime and size.
//
// Deliberately NOT a content hash: hashing would mean reading the whole file on
// every request, which is what ServeContent was adopted to avoid, and on an
// unauthenticated route that is a read amplifier. modtime+size is what
// http.FileServer uses for the same reason. The failure mode — an edit that
// preserves both — is not reachable by an operator editing a file, and
// Cache-Control: no-cache means a stale entity is at worst one revalidation old
// rather than cached indefinitely.
//
// Hashed rather than formatted so the value leaks no filesystem metadata.
func customEntryETag(modUnixNano, size int64) string {
	// Formatted rather than converted to uint64: the values are non-negative
	// here, but a signed->unsigned cast is a gosec G115 finding and suppressing
	// it would be noise for no benefit. Hashing the text is equivalent.
	sum := sha256.Sum256([]byte(strconv.FormatInt(modUnixNano, 10) + ":" + strconv.FormatInt(size, 10)))
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}
