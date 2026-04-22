// Package frontendparity is a test-only package whose job is cross-boundary
// drift detection between the Go frontendroutes catalog and the Vue Router
// definition in frontend/src/router/index.ts. Keeping these checks in a
// dedicated package lets internal/frontendroutes stay a stdlib-only leaf.
package frontendparity

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/frontendroutes"
)

// routerFileRelative is the path from this test file to the Vue router source.
const routerFileRelative = "../../frontend/src/router/index.ts"

// tsRouteEntry matches one { path: '...', name: '...' } block inside the
// routes array. The regex is intentionally narrow: it only handles single-
// quoted string literals on adjacent lines, which is how the router file is
// formatted today. If the file is restructured the regex returns no matches
// and the test fails loudly so we can update both sides together.
var tsRouteEntry = regexp.MustCompile(`(?m)path:\s*'([^']+)',\s*\n\s*name:\s*'([^']+)'`)

func TestFrontendCatalog_matchesVueRouter(t *testing.T) {
	t.Parallel()

	routerPath := resolveRouterPath(t)
	data, err := os.ReadFile(routerPath)
	if err != nil {
		t.Fatalf("cannot read Vue router file %s: %v", routerPath, err)
	}

	tsRoutes := extractTSRoutes(string(data))
	if len(tsRoutes) == 0 {
		t.Fatalf("no routes extracted from %s; parity regex likely needs updating to match the new router shape", routerPath)
	}

	goRoutes := map[string]string{} // name → path
	for _, r := range frontendroutes.All() {
		goRoutes[r.Name] = r.Path
	}

	// Every named TS route must exist in the Go catalog with the same path.
	for _, r := range tsRoutes {
		goPath, ok := goRoutes[r.name]
		if !ok {
			t.Errorf("route %q (path %q) present in frontend but missing from internal/frontendroutes — add it to internal/frontendroutes/routes.go", r.name, r.path)
			continue
		}
		if goPath != r.path {
			t.Errorf("route %q: frontend path %q != catalog path %q — update one of them", r.name, r.path, goPath)
		}
	}

	// Every Go route must exist in the TS router.
	tsByName := map[string]string{}
	for _, r := range tsRoutes {
		tsByName[r.name] = r.path
	}
	// Deterministic iteration for stable failure output.
	names := make([]string, 0, len(goRoutes))
	for n := range goRoutes {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if _, ok := tsByName[n]; !ok {
			t.Errorf("route %q (path %q) present in internal/frontendroutes but missing from frontend/src/router/index.ts — add it there or remove it from the catalog", n, goRoutes[n])
		}
	}
}

type tsRoute struct {
	path string
	name string
}

func extractTSRoutes(source string) []tsRoute {
	matches := tsRouteEntry.FindAllStringSubmatch(source, -1)
	out := make([]tsRoute, 0, len(matches))
	for _, m := range matches {
		out = append(out, tsRoute{path: m[1], name: m[2]})
	}
	return out
}

// resolveRouterPath returns an absolute path to the Vue router file,
// anchored at this source file's directory so `go test` works regardless
// of the caller's cwd.
func resolveRouterPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate parity test source file")
	}
	return filepath.Join(filepath.Dir(thisFile), routerFileRelative)
}
