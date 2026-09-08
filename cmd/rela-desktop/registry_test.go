package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/desktop"
)

// Desktop projects arrive through a file picker from anywhere on disk, so two
// different directories can share a folder name. The id must distinguish them.
func TestProjectID(t *testing.T) {
	acme := projectID("/Users/x/work/acme/tickets")
	globex := projectID("/Users/x/work/globex/tickets")

	assert.NotEqual(t, acme, globex, "same folder name, different paths must not collide")
	assert.Len(t, acme, projectIDLen)
	assert.Equal(t, acme, projectID("/Users/x/work/acme/tickets"), "must be stable")
	assert.Equal(t, acme, projectID("/Users/x/work/acme/./tickets"), "must be path-cleaned")
	assert.Equal(t, acme, projectID("/Users/x/work/acme/tickets/"), "trailing slash is the same dir")
}

func TestSplitProjectPath(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantID  string
		wantRes string
		wantOK  bool
	}{
		{"bare root is not a project path", "/", "", "", false},
		{"api at root", "/api/v1/entities", "", "", false},
		{"project root without trailing slash", "/p/abc", "abc", "/", true},
		{"project root with trailing slash", "/p/abc/", "abc", "/", true},
		{"nested path", "/p/abc/api/v1/entities", "abc", "/api/v1/entities", true},
		{"path with query-ish characters", "/p/abc/list/x%20y", "abc", "/list/x%20y", true},
		{"empty id is not a project path", "/p/", "", "", false},
		{"empty id with trailing slash", "/p//api", "", "", false},
		{"prefix-like but not the prefix", "/psomething", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, rest, ok := splitProjectPath(tc.in)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantID, id)
			assert.Equal(t, tc.wantRes, rest)
		})
	}
}

// The rest path is handed to a mux unchanged, so it must always be rooted.
func TestSplitProjectPathAlwaysRooted(t *testing.T) {
	for _, in := range []string{"/p/abc", "/p/abc/", "/p/abc/x", "/p/abc/x/y"} {
		_, rest, ok := splitProjectPath(in)
		require.True(t, ok, in)
		assert.Equal(t, byte('/'), rest[0], "rest %q for %q must start with /", rest, in)
	}
}

func TestRegistryLifecycle(t *testing.T) {
	r := newProjectRegistry()
	a := &loadedProject{id: "a", root: "/a", name: "A"}
	b := &loadedProject{id: "b", root: "/b", name: "B"}

	t.Run("empty registry has no active project", func(t *testing.T) {
		assert.Nil(t, r.activeProject())
		assert.Nil(t, r.get("a"))
		assert.Empty(t, r.all())
	})

	t.Run("add makes a project active", func(t *testing.T) {
		r.add(a)
		assert.Equal(t, a, r.get("a"))
		assert.Equal(t, a, r.activeProject())
	})

	t.Run("adding a second switches active", func(t *testing.T) {
		r.add(b)
		assert.Equal(t, b, r.activeProject())
		assert.Len(t, r.all(), 2)
	})

	t.Run("setActive ignores unknown ids", func(t *testing.T) {
		r.setActive("nope")
		assert.Equal(t, b, r.activeProject(), "a stale window must not blank the root")
	})

	t.Run("setActive switches between known ids", func(t *testing.T) {
		r.setActive("a")
		assert.Equal(t, a, r.activeProject())
	})

	t.Run("remove returns the project and falls back", func(t *testing.T) {
		got := r.remove("a")
		assert.Equal(t, a, got)
		assert.Nil(t, r.get("a"))
		// "a" was active; the root must keep working via the survivor.
		assert.Equal(t, b, r.activeProject())
	})

	t.Run("removing the last leaves no active project", func(t *testing.T) {
		r.remove("b")
		assert.Nil(t, r.activeProject())
		assert.Empty(t, r.all())
	})

	t.Run("removing an unknown id is a no-op", func(t *testing.T) {
		assert.Nil(t, r.remove("gone"))
	})
}

// The scheduler runs only while a project has an open window, so the count
// must be accurate and must never go negative.
func TestRegistryWindowCounting(t *testing.T) {
	r := newProjectRegistry()
	r.add(&loadedProject{id: "a"})

	r.retainWindow("a")
	r.retainWindow("a")

	assert.False(t, r.releaseWindow("a"), "one window still open")
	assert.True(t, r.releaseWindow("a"), "last window closed")

	t.Run("releasing below zero stays at zero", func(t *testing.T) {
		assert.True(t, r.releaseWindow("a"))
		assert.Equal(t, 0, r.get("a").windows)
	})

	t.Run("unknown project is not last", func(t *testing.T) {
		assert.False(t, r.releaseWindow("nope"))
	})
}

// byRoot is what decides "focus the existing window" vs "load another copy".
func TestRegistryByRoot(t *testing.T) {
	r := newProjectRegistry()
	root := "/Users/x/work/acme/tickets"
	p := &loadedProject{id: projectID(root), root: root}
	r.add(p)

	assert.Equal(t, p, r.byRoot(root))
	assert.Equal(t, p, r.byRoot(root+"/"), "trailing slash is the same project")
	assert.Nil(t, r.byRoot("/Users/x/work/globex/tickets"))
}

// Routing is the whole point of the registry: a /p/<id>/ request must reach
// that project's handler with the prefix stripped, and everything else must
// reach the active project unchanged.
func TestServeHTTP_RoutesByProject(t *testing.T) {
	newStub := func(name string, seen *string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*seen = r.URL.Path
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(name))
		})
	}

	var pathA, pathB string
	a := &loadedProject{id: "aaa", root: "/a", name: "A", handler: newStub("A", &pathA)}
	b := &loadedProject{id: "bbb", root: "/b", name: "B", handler: newStub("B", &pathB)}

	d := &Desktop{prefs: &desktop.Preferences{}, registry: newProjectRegistry()}
	d.registry.add(a)
	d.registry.add(b) // b is active, being added last
	d.handler = b.handler

	t.Run("prefixed request reaches its own project", func(t *testing.T) {
		rec := httptest.NewRecorder()
		d.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/p/aaa/api/v1/entities", http.NoBody))

		assert.Equal(t, "A", rec.Body.String(), "must reach project A, not the active one")
		assert.Equal(t, "/api/v1/entities", pathA, "the prefix must be stripped")
	})

	t.Run("the other project is reachable at the same time", func(t *testing.T) {
		rec := httptest.NewRecorder()
		d.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/p/bbb/list/x", http.NoBody))

		assert.Equal(t, "B", rec.Body.String())
		assert.Equal(t, "/list/x", pathB)
	})

	t.Run("project root keeps a rooted path", func(t *testing.T) {
		rec := httptest.NewRecorder()
		d.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/p/aaa", http.NoBody))

		assert.Equal(t, "/", pathA)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("unprefixed request reaches the active project", func(t *testing.T) {
		rec := httptest.NewRecorder()
		d.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/entities", http.NoBody))

		assert.Equal(t, "B", rec.Body.String())
	})

	t.Run("unknown project serves the welcome page, not a bare 404", func(t *testing.T) {
		rec := httptest.NewRecorder()
		d.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/p/gone/list/x", http.NoBody))

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "not open",
			"a closed project should say so rather than 404")
	})
}

// The request handed to a project must be a copy: mutating the caller's
// URL would corrupt it for anything sharing the request.
func TestServeHTTP_DoesNotMutateCallerRequest(t *testing.T) {
	d := &Desktop{prefs: &desktop.Preferences{}, registry: newProjectRegistry()}
	d.registry.add(&loadedProject{
		id: "aaa",
		handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	})

	req := httptest.NewRequest(http.MethodGet, "/p/aaa/list/x", http.NoBody)
	d.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, "/p/aaa/list/x", req.URL.Path, "the caller's request must be untouched")
}
