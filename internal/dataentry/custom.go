package dataentry

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/project"
)

// Operator customisation hooks (TKT-3DBK6I, TKT-IWMETE).
//
// An operator may create a custom/ directory in their project. custom.css and
// custom.js there are referenced from the SPA shell when present; every other
// file in the directory (fonts, logos, images, nested paths) is served as-is at
// /_custom/<path>, so operator CSS can write url(/_custom/logo.svg). This is
// deliberately a "if it breaks, you keep the pieces" feature: the palette/theme
// system remains the supported path for ordinary branding.
//
// # The directory is PUBLIC and UNAUTHENTICATED
//
// /_custom/ is not an isAPIPath, so it sits outside both the JWT gate and the
// ACL — deliberately, so the shell and its assets load before login (gating
// them would render the login page unstyled). Everything in custom/ is
// therefore readable by anyone who can reach the server, NOT merely by a
// logged-in operator. This is a wider exposure than apps/, which is under
// /api/ and thus gated, despite apps/ being the pattern this code copies.
// Dot-prefixed paths are refused (see validCustomEntry) to catch the realistic
// accidents; everything else is the operator's responsibility and is
// documented as such in docs/customisation.md.
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
	// customCSSFile / customJSFile are the two entry points injected into the
	// SPA shell. They are NOT an allowlist — every file in custom/ is served.
	// Kept at these names rather than index.* because "index" implies "served
	// at the directory root", which is not what they are.
	customCSSFile = "custom.css"
	customJSFile  = "custom.js"

	// customURLPrefix is the fixed mount point. Underscore-prefixed to match
	// the reserved-path convention used by /_apps/. Registered as a ServeMux
	// prefix pattern, so it already matches nested paths.
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
//
// This is not a confidentiality measure — which files exist under custom/ is
// operator config, not entity data. It is simply that there is no useful
// distinction to draw and one path is easier to reason about. The oversize
// branch logs server-side so an operator whose too-large image silently 404s
// gets a diagnostic instead of a mystery.
var errCustomAssetNotFound = errors.New("custom asset not found")

// validCustomEntry normalises a request-relative path and reports whether it may
// be served from custom/. Returns the cleaned path and ok.
//
// Two independent checks, doing different jobs — do not conflate them:
//
//   - path.Clean + fs.ValidPath reject malformed and traversal spellings. This
//     is what stops "../secret"; os.OpenRoot below is the real containment.
//   - The dot-segment check rejects operator accidents (.env, .git/, .DS_Store)
//     that are perfectly valid paths. It contributes ZERO traversal defense:
//     path.Clean resolves ".." away BEFORE this runs, so "../secret" arrives
//     here as "secret" with no dot segment. An earlier draft claimed otherwise.
//
// The dot rule is a filename-shape heuristic, not a sensitivity classifier. It
// does NOT catch notes.md, backup.sql, id_rsa, or editor backups like
// "custom.css~" and "#custom.css#" (only dot-prefixed swap files such as
// .custom.css.swp). It also refuses .well-known/, the one false positive we
// know of. Both facts are documented rather than silently assumed away.
func validCustomEntry(entry string) (string, bool) {
	clean := path.Clean("/" + entry)
	if clean == "/" {
		return "", false
	}
	rel := strings.TrimPrefix(clean, "/")
	if !fs.ValidPath(rel) {
		return "", false
	}
	for seg := range strings.SplitSeq(rel, "/") {
		if strings.HasPrefix(seg, ".") {
			return "", false
		}
	}
	return rel, true
}

// projectAssets reads the operator's custom/ and apps/ files. Declared here,
// at the consumer: config.Loader plus the optional [config.Stater] and
// [config.DirLister] capabilities, which [NewApp] requires of its loader.
type projectAssets interface {
	Load(ctx context.Context, name string) ([]byte, error)
	Stat(ctx context.Context, name string) (fs.FileInfo, error)
	Dirs(ctx context.Context, dir string) ([]string, error)
}

// openCustomEntry reads custom/{entry} through files.
//
// Containment is the loader's: on disk a [rootfs.Dir] opens every directory
// level as its own os.Root, so a symlink inside custom/ pointing at
// ../schema.yaml never resolves. TestOpenCustomEntry_Symlink pins it.
//
// Every failure returns errCustomAssetNotFound.
func openCustomEntry(ctx context.Context, files projectAssets, entry string) ([]byte, error) {
	f, err := openCustomEntryFile(ctx, files, entry)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(f.Content)
}

// customEntryFile is a custom/ entry plus the metadata http.ServeContent
// needs to honor Range and conditional requests.
type customEntryFile struct {
	Content io.ReadSeeker
	ModTime time.Time
	Size    int64
}

// openCustomEntryFile resolves and reads a custom/ entry.
//
// The size gate runs on the Stat, BEFORE reading. Reading first and checking
// after would let one oversize file in custom/ cost a full read and
// allocation per request on a route that is deliberately unauthenticated, a
// trivially reachable amplifier.
//
// Surfaced, not silent: the uniform 404 is explicitly not a confidentiality
// measure, so an operator whose hero image exceeds the cap deserves to know
// why it "just 404s" while their logo works. `rel` is request-relative and
// never the absolute project path.
func openCustomEntryFile(ctx context.Context, files projectAssets, entry string) (*customEntryFile, error) {
	rel, ok := validCustomEntry(entry)
	if !ok {
		return nil, errCustomAssetNotFound
	}
	name := project.CustomDir + "/" + rel
	info, err := files.Stat(ctx, name)
	if err != nil || info.IsDir() {
		return nil, errCustomAssetNotFound
	}
	if info.Size() > maxCustomFileBytes {
		slog.Warn("custom asset exceeds size cap, not served",
			"entry", rel, "max_bytes", maxCustomFileBytes)
		return nil, errCustomAssetNotFound
	}
	b, err := files.Load(ctx, name)
	// Re-checked after the read: the file may have grown since the Stat.
	if err != nil || len(b) > maxCustomFileBytes {
		return nil, errCustomAssetNotFound
	}
	return &customEntryFile{Content: bytes.NewReader(b), ModTime: info.ModTime(), Size: int64(len(b))}, nil
}

// customAssetExists reports whether an entry is servable from custom/. Runs on
// every SPA shell request, so it STATS rather than reads.
//
// Note this is a genuine TOCTOU with the subsequent /_custom/ fetch: the file
// can be deleted (or half-written by an editor without atomic rename) between
// the shell referencing it and the browser requesting it. The precomputed
// variants remove the cache-population race, NOT this one. The consequence is
// bounded — a 404, or a syntax error in operator-authored code — and is
// consistent with the feature's "if it breaks, you keep the pieces" contract.
func customAssetExists(ctx context.Context, files projectAssets, entry string) bool {
	rel, ok := validCustomEntry(entry)
	if !ok {
		return false
	}
	info, err := files.Stat(ctx, project.CustomDir+"/"+rel)
	return err == nil && !info.IsDir() && info.Size() <= maxCustomFileBytes
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
// TRIP-WIRE: this table is 2^2 = 4 entries because the injected set is exactly
// two and fixed. Arbitrary assets under custom/ are never injected, so they do
// not multiply it. If a third injected entry is ever added (a custom.head.html,
// a second stylesheet, a per-theme variant) this becomes 8 and the switch in
// selectShell becomes combinatorial — redesign rather than adding a fifth field.
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
func (v shellVariants) selectShell(ctx context.Context, files projectAssets) []byte {
	hasCSS := customAssetExists(ctx, files, customCSSFile)
	hasJS := customAssetExists(ctx, files, customJSFile)
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
// under custom/, serving them, and deciding which SPA shell variant to
// hand out.
//
// A focused type rather than more methods on App — these four fields are one
// coherent thing (where the files live, whether referencing them is enabled,
// and the precomputed shells), and nothing here needs the rest of App.
type customAssets struct {
	files projectAssets
	// enabled is read per request rather than snapshotted: data-entry.yaml is
	// reloadable, so caching the flag at construction would leave a running
	// server honoring a stale disable_custom_injection.
	enabled  func() bool
	variants shellVariants
}

// newCustomAssets precomputes the shell variants once. shell is the embedded
// index.html; enabled is consulted per request.
func newCustomAssets(files projectAssets, shell []byte, enabled func() bool) *customAssets {
	return &customAssets{
		files:    files,
		enabled:  enabled,
		variants: buildShellVariants(shell),
	}
}

// shell returns the SPA shell to serve for the current filesystem and config
// state, or nil when there is no shell to rewrite (unreadable embedded
// index.html) and the caller should fall back to the plain file server.
func (c *customAssets) shell(ctx context.Context) []byte {
	if len(c.variants.plain) == 0 {
		return nil
	}
	if !c.enabled() {
		return c.variants.plain
	}
	return c.variants.selectShell(ctx, c.files)
}
