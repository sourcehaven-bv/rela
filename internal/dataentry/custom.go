package dataentry

import (
	"errors"
	"io"
	"os"
	"strings"
)

// Operator customisation hooks (TKT-3DBK6I).
//
// An operator may drop custom.css / custom.js in their project root. Both are
// served under customURLPrefix and, when present, referenced from the SPA
// shell. This is deliberately a "if it breaks, you keep the pieces" feature:
// the palette/theme system remains the supported path for ordinary branding.
//
// # Trust model — NOT the same as apps/
//
// Custom apps (apps/<id>/) are UNTRUSTED and confined: null-origin sandboxed
// iframe, path-scoped CSP, connect-src 'none', a closed method allow-list
// bridge. They exist so a *distributable* app is safe to install.
//
// custom.js is the opposite: it runs as a module in the SPA's own document —
// same-origin, no CSP, no sandbox, unrestricted fetch to /api/v1/* with the
// session cookie. That is correct, because an operator editing their own
// project directory already controls the metamodel, Lua scripts and ACL, so
// there is no privilege boundary left to defend. Do not reason from "apps are
// sandboxed" to "custom.js is sandboxed" — the two have opposite postures.
const (
	// customCSSFile / customJSFile are the ONLY two names served. An exact
	// allowlist (rather than apps/'s arbitrary-entry handling) makes traversal
	// structurally impossible before the filesystem is touched.
	customCSSFile = "custom.css"
	customJSFile  = "custom.js"

	// customURLPrefix is the fixed mount point. Underscore-prefixed to match
	// the reserved-path convention used by /_apps/.
	customURLPrefix = "/_custom/"
)

// spaIndexFile is the SPA shell within the embedded build output.
const spaIndexFile = "index.html"

// maxCustomFileBytes caps a single served customisation file. Shares the app
// file cap: generous for a stylesheet or script, while bounding the memory a
// pathological file can force us to buffer.
const maxCustomFileBytes = maxAppFileBytes

// errCustomAssetNotFound is the single error every failure collapses into.
// Callers map it to a uniform 404 so a missing file, a directory, an oversize
// file and a traversal attempt are indistinguishable from outside, and no
// system path leaks in the message (matching openAppEntry).
var errCustomAssetNotFound = errors.New("custom asset not found")

// isCustomAssetName reports whether name is one of the two allowlisted files.
func isCustomAssetName(name string) bool {
	return name == customCSSFile || name == customJSFile
}

// customAssetContentType returns the Content-Type for an allowlisted name.
// Only ever called with a name that passed isCustomAssetName.
func customAssetContentType(name string) string {
	if name == customCSSFile {
		return "text/css; charset=utf-8"
	}
	return "text/javascript; charset=utf-8"
}

// openCustomAsset reads {projectRoot}/{name} traversal-resistant.
//
// name MUST be one of the two allowlisted literals — it is never derived from
// a request path segment beyond an exact comparison, so ".." and absolute
// paths cannot reach here at all. os.OpenRoot is kept as defense-in-depth
// (and to reject a symlink pointing outside the project), matching the
// "earlier rejection gives better errors" rationale in openLocalScript.
//
// Every failure returns errCustomAssetNotFound.
func openCustomAsset(projectRoot, name string) ([]byte, error) {
	if !isCustomAssetName(name) {
		return nil, errCustomAssetNotFound
	}

	root, err := os.OpenRoot(projectRoot)
	if err != nil {
		return nil, errCustomAssetNotFound
	}
	defer func() { _ = root.Close() }()

	f, err := root.Open(name)
	if err != nil {
		return nil, errCustomAssetNotFound
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return nil, errCustomAssetNotFound
	}

	b, err := io.ReadAll(io.LimitReader(f, maxCustomFileBytes+1))
	if err != nil || len(b) > maxCustomFileBytes {
		return nil, errCustomAssetNotFound
	}
	return b, nil
}

// customAssetExists reports whether an allowlisted customisation file is
// present. Runs on every SPA shell request, so it STATS rather than reads —
// reading here would pull up to maxCustomFileBytes into memory twice per page
// load (once per file) only to discard it.
//
// Note this is a genuine TOCTOU with the subsequent /_custom/ fetch: the file
// can be deleted (or half-written by an editor without atomic rename) between
// the shell referencing it and the browser requesting it. The precomputed
// variants remove the cache-population race, NOT this one. The consequence is
// bounded — a 404 or a syntax error in operator-authored code — and is
// consistent with the feature's stated "if it breaks, you keep the pieces"
// contract, so it is accepted rather than locked against.
func customAssetExists(projectRoot, name string) bool {
	if !isCustomAssetName(name) {
		return false
	}
	root, err := os.OpenRoot(projectRoot)
	if err != nil {
		return false
	}
	defer func() { _ = root.Close() }()

	info, err := root.Stat(name)
	return err == nil && !info.IsDir()
}

// The tags injected into the SPA shell.
//
// type="module" is deliberate: it defers automatically (running after DOM
// parse and after the SPA's own module) and gives the operator imports and
// top-level await with no build step.
const (
	customCSSTag = `<link rel="stylesheet" href="` + customURLPrefix + customCSSFile + `">`
	customJSTag  = `<script type="module" src="` + customURLPrefix + customJSFile + `"></script>`
)

// shellVariants holds the four possible SPA shells, precomputed once.
//
// Precomputing (rather than rewriting per request) means no cache-population
// race and no lock on the hot path; selecting per request by a cheap stat
// means adding custom.css does NOT require a server restart. Both properties
// matter: an operator who adds custom.css and sees it served at /_custom/ but
// never applied would reasonably file a bug.
type shellVariants struct {
	plain []byte // neither file
	css   []byte // custom.css only
	js    []byte // custom.js only
	both  []byte
}

// buildShellVariants precomputes every injected form of the shell.
//
// If the shell lacks the expected insertion points, injection is skipped for
// that tag rather than producing corrupt HTML.
func buildShellVariants(shell []byte) shellVariants {
	v := shellVariants{plain: shell}
	v.css = injectTags(shell, customCSSTag, "")
	v.js = injectTags(shell, "", customJSTag)
	v.both = injectTags(shell, customCSSTag, customJSTag)
	return v
}

// selectShell returns the shell to serve for the current filesystem state.
// The caller is responsible for the enabled check.
func (v shellVariants) selectShell(projectRoot string) []byte {
	hasCSS := customAssetExists(projectRoot, customCSSFile)
	hasJS := customAssetExists(projectRoot, customJSFile)
	switch {
	case hasCSS && hasJS:
		return v.both
	case hasCSS:
		return v.css
	case hasJS:
		return v.js
	default:
		return v.plain
	}
}

// injectTags inserts cssTag before </head> and jsTag before </body>.
//
// A targeted string insertion, NOT a golang.org/x/net/html parse+render
// round-trip. A full round-trip would normalise the entire document
// (attribute quoting and order, entity re-encoding) and would be the first
// html.Render in the codebase; the shell is our own embedded, known-shape
// output, so a lossless splice is smaller and easier to assert on. x/net/html
// being an existing dependency is not on its own a reason to use it.
//
// An empty tag is skipped. A missing insertion point leaves the shell
// untouched for that tag.
func injectTags(shell []byte, cssTag, jsTag string) []byte {
	out := string(shell)
	if cssTag != "" {
		out = insertBefore(out, "</head>", cssTag)
	}
	if jsTag != "" {
		out = insertBefore(out, "</body>", jsTag)
	}
	return []byte(out)
}

// insertBefore splices tag in ahead of the LAST occurrence of marker,
// preserving the marker's own indentation. Returns s unchanged if marker is
// absent.
func insertBefore(s, marker, tag string) string {
	i := strings.LastIndex(s, marker)
	if i < 0 {
		return s
	}
	// Reuse the indentation preceding the marker so injected output matches
	// the surrounding formatting.
	indent := ""
	if nl := strings.LastIndex(s[:i], "\n"); nl >= 0 {
		indent = s[nl+1 : i]
	}
	return s[:i] + tag + "\n" + indent + s[i:]
}

// customAssets owns the operator-customisation surface: locating the two files
// under the project root, serving them, and deciding which SPA shell variant to
// hand out.
//
// A focused type rather than more methods on App — these four fields are one
// coherent thing (where the files live, whether referencing them is enabled,
// and the precomputed shells), and nothing here needs the rest of App.
type customAssets struct {
	projectRoot string
	// enabled is read per request rather than snapshotted: data-entry.yaml is
	// reloadable, so caching the flag at construction would leave a running
	// server honoring a stale disable_custom_injection.
	enabled  func() bool
	variants shellVariants
}

// newCustomAssets precomputes the shell variants once. shell is the embedded
// index.html; enabled is consulted per request.
func newCustomAssets(projectRoot string, shell []byte, enabled func() bool) *customAssets {
	return &customAssets{
		projectRoot: projectRoot,
		enabled:     enabled,
		variants:    buildShellVariants(shell),
	}
}

// shell returns the SPA shell to serve for the current filesystem and config
// state, or nil when there is no shell to rewrite (unreadable embedded
// index.html) and the caller should fall back to the plain file server.
func (c *customAssets) shell() []byte {
	if len(c.variants.plain) == 0 {
		return nil
	}
	if !c.enabled() {
		return c.variants.plain
	}
	return c.variants.selectShell(c.projectRoot)
}
