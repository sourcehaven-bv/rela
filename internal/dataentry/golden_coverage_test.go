package dataentry_test

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
)

// TestGoldenParity_CoversEveryMethod pins that the corpus records every
// method each route answers. The route patterns come from the router walk.
// The methods a pattern answers are not listed by hand: the test sends
// goldenSweepMethods (and any other method the walk probes) to each pattern
// on a corpus app, and every method that does not answer 405 must have a
// corpus case or a reasoned exemption. A case whose method answers 405 is
// not coverage, so it fails too. A method whose answer is byte for byte the
// GET answer (a handler that ignores the method) is covered by the GET case.
// A handler that grows a method therefore cannot ship outside the wire
// contract by accident.
//
// The sweep sends one request per method to one path per pattern: the path of
// a corpus case for that pattern where one exists, so the request reaches an
// entity the fixture has, else the walk's probe path.
func TestGoldenParity_CoversEveryMethod(t *testing.T) {
	routes := dataentry.RouterWalkRoutes()
	patterns, probePath, probeMethods := goldenPatterns(routes)
	cases := goldenCases()

	covered := map[string][]string{}
	sweepPath := map[string]string{}
	for _, c := range cases {
		pattern, ok := goldenPatternOf(patterns, c.path)
		if !ok {
			t.Errorf("corpus case %s (%s %s) matches no route of the router walk; add a probe "+
				"to routerWalkProbes", c.slug, c.method, c.path)
			continue
		}
		key := c.method + " " + pattern
		if !slices.Contains(covered[key], c.slug) {
			covered[key] = append(covered[key], c.slug)
		}
		if _, ok := sweepPath[pattern]; !ok {
			sweepPath[pattern] = c.path
		}
	}

	answered := sweepGoldenRoutes(t, patterns, func(pattern string) (string, []string) {
		path := sweepPath[pattern]
		if path == "" {
			path = probePath[pattern]
		}
		return path, goldenMethodsFor(probeMethods[pattern])
	})

	for _, key := range slices.Sorted(maps.Keys(answered)) {
		got := answered[key]
		method, pattern, _ := strings.Cut(key, " ")
		_, exempt := goldenMethodExemptions[key]
		answersLikeGET := method != http.MethodGet && got == answered[http.MethodGet+" "+pattern]
		cases := covered[key]
		if answersLikeGET {
			cases = covered[http.MethodGet+" "+pattern]
		}
		switch {
		case got.status == http.StatusMethodNotAllowed && len(covered[key]) > 0:
			t.Errorf("%s answers 405, which is not coverage; drop corpus cases %v", key, covered[key])
		case got.status == http.StatusMethodNotAllowed && exempt:
			t.Errorf("%s answers 405; drop its exemption", key)
		case got.status == http.StatusMethodNotAllowed || goldenMethodExemptions["* "+pattern] != "":
		case len(cases) > 0 && exempt:
			t.Errorf("%s has corpus cases %v and an exemption; drop the exemption", key, cases)
		case len(cases) == 0 && !exempt:
			t.Errorf("%s answers %d and has no corpus case; add one to the corpus, or exempt it "+
				"in goldenMethodExemptions with a reason", key, got.status)
		}
	}
	for key, slugs := range covered {
		if _, swept := answered[key]; !swept && goldenUnsweptRoutes[strings.SplitN(key, " ", 2)[1]] == "" {
			t.Errorf("corpus cases %v use %s, which the sweep did not send; add the method to the "+
				"walk probes", slugs, key)
		}
	}
	for key := range goldenMethodExemptions {
		method, pattern, _ := strings.Cut(key, " ")
		if !slices.Contains(patterns, pattern) {
			t.Errorf("exemption %q names no route of the router walk; drop it", key)
			continue
		}
		if method == "*" {
			continue
		}
		if _, swept := answered[key]; !swept {
			t.Errorf("exemption %q names a method the sweep does not send; drop it", key)
		}
	}
	for pattern := range goldenUnsweptRoutes {
		if !slices.Contains(patterns, pattern) {
			t.Errorf("unswept route %q is not a route of the router walk; drop it", pattern)
		}
	}
}

// goldenSweepMethods are the methods the coverage sweep sends to every
// route pattern, besides any other method the walk probes for it.
var goldenSweepMethods = []string{
	http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete,
}

// goldenMethodsFor returns the sweep methods plus the extra ones a pattern's
// probes use.
func goldenMethodsFor(probed []string) []string {
	out := slices.Clone(goldenSweepMethods)
	for _, m := range probed {
		if !slices.Contains(out, m) {
			out = append(out, m)
		}
	}
	return out
}

// goldenPatterns returns the walk's distinct route patterns in walk order,
// a probe path for each, and the methods its probes use.
func goldenPatterns(routes []dataentry.RouterWalkRoute) (
	patterns []string, probePath map[string]string, probeMethods map[string][]string,
) {
	probePath = map[string]string{}
	probeMethods = map[string][]string{}
	for _, r := range routes {
		if _, ok := probePath[r.Route]; !ok {
			patterns = append(patterns, r.Route)
			probePath[r.Route] = r.Path
		}
		probeMethods[r.Route] = append(probeMethods[r.Route], r.Method)
	}
	return patterns, probePath, probeMethods
}

// sweepGoldenRoutes sends every method target names for a pattern to that
// pattern, on a fresh corpus app per pattern, as the administrator over the
// JWT gate (the only wiring that mounts the MCP endpoint). It returns the
// answer per "METHOD pattern".
func sweepGoldenRoutes(
	t *testing.T, patterns []string, target func(pattern string) (path string, methods []string),
) map[string]goldenSweepAnswer {
	t.Helper()
	var mu sync.Mutex
	answered := map[string]goldenSweepAnswer{}
	t.Run("sweep", func(t *testing.T) {
		for _, pattern := range patterns {
			if goldenUnsweptRoutes[pattern] != "" {
				continue
			}
			t.Run(pattern, func(t *testing.T) {
				t.Parallel()
				path, methods := target(pattern)
				g := newGoldenApp(t, goldenDeclJWT)
				for _, method := range methods {
					c := goldenCase{
						goldenEndpoint: goldenEndpoint{method: method, path: path, body: "{}"},
						variant:        goldenDeclJWT, as: goldenAdmin,
					}
					rec := httptest.NewRecorder()
					serveFailingOnPanic(t, g.router, rec, goldenRequest(t, c))
					mu.Lock()
					answered[method+" "+pattern] = goldenSweepAnswer{status: rec.Code, body: rec.Body.String()}
					mu.Unlock()
				}
			})
		}
	})
	return answered
}

// goldenSweepAnswer is the status and body a sweep request got.
type goldenSweepAnswer struct {
	status int
	body   string
}

// goldenPatternOf returns the pattern of patterns that serves path. Of
// several matching patterns it picks the one with the most literal segments,
// the way the dispatcher prefers `_export` over an id.
func goldenPatternOf(patterns []string, path string) (string, bool) {
	path, _, _ = strings.Cut(path, "?")
	segments := strings.Split(path, "/")
	best, bestLiterals := "", -1
	for _, p := range patterns {
		if literals, ok := matchRoute(strings.Split(p, "/"), segments); ok && literals > bestLiterals {
			best, bestLiterals = p, literals
		}
	}
	return best, bestLiterals >= 0
}

// matchRoute reports whether segments fit pattern, and how many of the
// pattern's segments are literal. {name} matches one non-empty segment and
// {name...} the rest of the path.
func matchRoute(pattern, segments []string) (literals int, ok bool) {
	for i, p := range pattern {
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "...}") {
			return literals, true
		}
		if i >= len(segments) {
			return 0, false
		}
		switch {
		case strings.HasPrefix(p, "{"):
			if segments[i] == "" {
				return 0, false
			}
		case p == segments[i]:
			literals++
		default:
			return 0, false
		}
	}
	return literals, len(pattern) == len(segments)
}

// Reasons a method of a route has no corpus case.
const (
	exemptGit = "runs git against the project checkout; the corpus project has no repository, " +
		"and the answer carries no entity data"
	exemptSSE = "the event stream stays open until the client leaves, so it has no single answer " +
		"to record; its events carry ids only and are gated per subscriber"
)

// goldenUnsweptRoutes are the patterns the sweep does not send to, with the
// reason. They have no corpus case either.
var goldenUnsweptRoutes = map[string]string{
	"/api/events":     exemptSSE,
	"/api/v1/_events": exemptSSE,
}

// goldenMethodExemptions are the "METHOD pattern" pairs the corpus does not
// record although the route answers them, with the reason. A method of "*"
// exempts every method of the pattern.
var goldenMethodExemptions = map[string]string{
	"* /api/git/status":     exemptGit,
	"* /api/git/sync":       exemptGit,
	"* /api/v1/_git/status": exemptGit,
	"* /api/v1/_git/sync":   exemptGit,
	"GET /api/v1/_conflicts/resolve": "reads the conflicted file named resolve, which is the " +
		"GET /api/v1/_conflicts/{path...} read the conflict cases record",
}

// TestGoldenParity_SlugListsNameEndpoints pins that every slug the variant
// lists name is an endpoint of the corpus, so a renamed or dropped endpoint
// cannot silently shrink a variant.
func TestGoldenParity_SlugListsNameEndpoints(t *testing.T) {
	slugs := map[string]bool{}
	for _, e := range goldenEndpoints() {
		slugs[e.slug] = true
	}
	for name, list := range map[string][]string{
		"goldenSwitchSlugs":     goldenSwitchSlugs,
		"goldenJWTRefusalSlugs": goldenJWTRefusalSlugs,
	} {
		var missing []string
		for _, slug := range list {
			if !slugs[slug] {
				missing = append(missing, slug)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			t.Errorf("%s names %s, which no corpus endpoint has", name, strings.Join(missing, ", "))
		}
	}
}
