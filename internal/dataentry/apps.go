package dataentry

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"os"
	"path"
	"sort"
	"strings"

	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// appsDir is the project directory holding custom apps. Each app is a
// subdirectory containing an index.html (plus any sibling assets). Mirrors the
// actions/ and scripts/ convention: a top-level project directory loaded
// traversal-resistant via os.OpenRoot, never stored in the entity store, so it
// lives on the filesystem in every storage backend.
//
// An app is live iff apps/<id>/index.html exists and <id> is a valid app id.
// Unpublish by renaming the folder or removing its index.html.
const appsDir = project.AppsDir

// appIndexFile is the entry document for an app directory.
const appIndexFile = "index.html"

// appSDKEntry is the reserved per-app path that serves the rela bridge SDK
// (window.rela). Apps include it with <script src="_rela.js"></script>. The
// leading underscore keeps it from colliding with a real app asset (entry names
// starting with "_" are not served from the app's own files).
const appSDKEntry = "_rela.js"

// appCSSEntry is the reserved per-app path that serves rela's optional theme
// tokens + base controls. Apps opt in with <link rel="stylesheet"
// href="_rela.css">. Reserved like _rela.js (underscore-prefixed).
const appCSSEntry = "_rela.css"

// maxAppFileBytes caps the size of any single served app file. Generous for a
// single-page app's assets while bounding memory pressure from a pathological
// file.
const maxAppFileBytes = 4 * 1024 * 1024

// appMetaPrefix is the prefix for the <meta name="..."> tags an app uses to
// describe itself (title/label/description) in its index.html. Cosmetic only.
const appMetaPrefix = "rela-app:"

// appInfo is the parsed, client-facing description of an app. ID is the folder
// name; the rest comes from <meta name="rela-app:*"> tags in index.html.
type appInfo struct {
	ID          string
	Title       string
	Label       string
	Description string
}

// scanApps lists the live apps under {projectRoot}/apps: every subdirectory
// with a valid id that contains an index.html. It reads each index.html to
// extract metadata, so the returned list is populated for the sidebar. A
// missing apps/ directory yields an empty list, not an error.
func scanApps(projectRoot string) ([]appInfo, error) {
	root, err := os.OpenRoot(projectRoot)
	if err != nil {
		return nil, errors.New("cannot access project directory")
	}
	defer func() { _ = root.Close() }()

	appsRoot, err := root.OpenRoot(appsDir)
	if err != nil {
		// No apps/ directory (or not a directory) → no apps. Not an error.
		return nil, nil
	}
	defer func() { _ = appsRoot.Close() }()

	entries, err := fs.ReadDir(appsRoot.FS(), ".")
	if err != nil {
		return nil, fmt.Errorf("cannot list %s directory", appsDir)
	}

	var apps []appInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		if !dataentryconfig.ValidAppID(id) {
			continue
		}
		index, err := openAppEntry(projectRoot, id, appIndexFile)
		if err != nil {
			// No index.html → not an app.
			continue
		}
		info := parseAppMeta(index)
		info.ID = id
		apps = append(apps, info)
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].ID < apps[j].ID })
	return apps, nil
}

// appExists reports whether apps/<id>/index.html exists (id pre-validated).
func appExists(projectRoot, id string) bool {
	_, err := openAppEntry(projectRoot, id, appIndexFile)
	return err == nil
}

// openAppEntry reads {projectRoot}/apps/{id}/{entry} traversal-resistant: it
// opens a fresh os.Root scoped to the app's own directory, so "../" / absolute
// / symlink entries cannot escape it (the same guard used for action scripts,
// extended one level deeper). entry uses forward slashes; "." and ".." segments
// and absolute paths are rejected before opening. Returns the bytes, or an error
// for missing/oversize/escaping entries.
func openAppEntry(projectRoot, id, entry string) ([]byte, error) {
	if id == "" || !dataentryconfig.ValidAppID(id) {
		return nil, fmt.Errorf("invalid app id: %q", id)
	}
	clean := path.Clean("/" + entry)
	if clean == "/" {
		return nil, errors.New("empty entry path")
	}
	rel := strings.TrimPrefix(clean, "/")
	if !fs.ValidPath(rel) {
		return nil, fmt.Errorf("invalid entry path: %q", entry)
	}

	root, err := os.OpenRoot(projectRoot)
	if err != nil {
		return nil, errors.New("cannot access project directory")
	}
	defer func() { _ = root.Close() }()

	appRoot, err := root.OpenRoot(path.Join(appsDir, id))
	if err != nil {
		return nil, fmt.Errorf("app not found: %s", id)
	}
	defer func() { _ = appRoot.Close() }()

	f, err := appRoot.Open(rel)
	if err != nil {
		return nil, fmt.Errorf("app entry not found: %s/%s", id, rel)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("cannot stat app entry: %s/%s", id, rel)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("app entry is a directory: %s/%s", id, rel)
	}

	b, err := io.ReadAll(io.LimitReader(f, maxAppFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read app entry: %s/%s", id, rel)
	}
	if len(b) > maxAppFileBytes {
		return nil, fmt.Errorf("app entry too large: %s/%s (max %d bytes)", id, rel, maxAppFileBytes)
	}
	return b, nil
}

// appEntryContentType returns the Content-Type for an app entry by extension.
// A correct type matters for CSP: a .js served as text/plain won't run under
// script-src, and nosniff blocks the mismatch.
func appEntryContentType(entry string) string {
	if ct := mime.TypeByExtension(path.Ext(entry)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

// parseAppMeta extracts an app's self-description from <meta name="rela-app:*">
// tags in index.html's <head>. All fields are optional; the sidebar falls back
// to label→title→id. Parse errors yield an empty appInfo (best-effort).
func parseAppMeta(htmlBytes []byte) appInfo {
	var info appInfo
	doc, err := nethtml.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return info
	}
	head := findFirstElementByAtom(doc, atom.Head)
	if head == nil {
		return info
	}
	for c := head.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != nethtml.ElementNode || c.DataAtom != atom.Meta {
			continue
		}
		var name, content string
		for _, a := range c.Attr {
			switch a.Key {
			case "name":
				name = a.Val
			case "content":
				content = a.Val
			}
		}
		if !strings.HasPrefix(name, appMetaPrefix) {
			continue
		}
		switch strings.TrimPrefix(name, appMetaPrefix) {
		case "title":
			info.Title = content
		case "label":
			info.Label = content
		case "description":
			info.Description = content
		}
	}
	return info
}

// findFirstElementByAtom returns the first element node with the given atom in a
// depth-first walk, or nil.
func findFirstElementByAtom(n *nethtml.Node, a atom.Atom) *nethtml.Node {
	if n.Type == nethtml.ElementNode && n.DataAtom == a {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findFirstElementByAtom(c, a); found != nil {
			return found
		}
	}
	return nil
}
