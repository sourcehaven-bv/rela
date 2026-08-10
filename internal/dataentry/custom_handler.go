package dataentry

import (
	"net/http"
	"strings"
)

// serveAsset serves the operator's custom.css / custom.js from the project root
// at /_custom/<name>.
//
// Every failure is a uniform 404 — a missing file, a directory, an oversize
// file and a non-allowlisted name are indistinguishable. That is not a
// confidentiality measure (the existence of custom.css is operator config, not
// entity data, and the root CLAUDE.md rule says config is not a secret); it is
// simply that there is no useful distinction to draw and one path is easier to
// reason about.
//
// Note this is NOT gated on the injection flag: disable_custom_injection only
// suppresses the shell references, so an operator can still fetch the files
// directly to check what is being served.
func (c *customAssets) serveAsset(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, customURLPrefix)

	body, err := openCustomAsset(c.projectRoot, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	h := w.Header()
	h.Set("Content-Type", customAssetContentType(name))
	// The bytes are operator-authored and executable; never let a browser
	// sniff a different type out of them.
	h.Set("X-Content-Type-Options", "nosniff")
	// These URLs are NOT content-hashed — /_custom/custom.css is stable
	// forever — so heuristic caching would serve a stale stylesheet after an
	// operator edits it, with no way for them to bust it. no-cache means
	// "revalidate", not "don't store", so it stays cheap.
	h.Set("Cache-Control", "no-cache")

	// #nosec G705 -- not an HTML sink. The response is text/css or
	// text/javascript with nosniff, so a browser never parses it as HTML. The
	// bytes are operator-authored by design: this is the documented "custom.js
	// is fully trusted" contract (see custom.go), and escaping a stylesheet or
	// a script would break the feature rather than protect anything. The
	// genuine boundary is the two-name allowlist above, which decides WHICH
	// file may be read, not what it contains.
	_, _ = w.Write(body)
}
