package dataentry

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
)

// serveAsset serves any file from the operator's custom/ directory at
// /_custom/<path>. Every failure is a uniform 404, for simplicity rather than
// confidentiality; an oversize file is also logged. It is not gated on
// disable_custom_injection, and it deliberately does no index resolution.
// Responses set nosniff, since the bytes may be executable. Delivery goes
// through http.ServeContent for conditional, Range and HEAD support.
//
//archplan:auth=none reason="operator assets the SPA shell loads before login"
func (c *customAssets) serveAsset(w http.ResponseWriter, r *http.Request) {
	entry := strings.TrimPrefix(r.URL.Path, customURLPrefix)

	f, err := openCustomEntryFile(r.Context(), c.files, entry)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()

	h := w.Header()
	h.Set("Content-Type", appEntryContentType(entry))
	h.Set("X-Content-Type-Options", "nosniff")
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
	// The containment boundary is validCustomEntry + the loader's per-area
	// os.Root — NOT a filename allowlist, which TKT-IWMETE removed.
	http.ServeContent(w, r, entry, f.ModTime, f.Content)
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
	sum := sha256.Sum256([]byte(strconv.FormatInt(modUnixNano, 10) + ":" + strconv.FormatInt(size, 10)))
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}
