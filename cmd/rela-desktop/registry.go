package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentry"
)

// Multi-project support.
//
// A project is mounted at /p/<id>/ and served by its own handler, so several
// projects can be open at once — File > Open Project opens a window on the new
// project instead of replacing the current one.
//
// Identity is a hash of the absolute path, not a slug of the folder name.
// Desktop projects arrive through a file picker from anywhere on disk, so
// ~/acme/tickets and ~/globex/tickets are both legitimately "tickets" and
// would collide. A server, where an operator configures the list deliberately,
// can afford readable slugs; see projectID's doc.

// projectIDLen is the hex prefix length of the path hash. 12 hex chars is 48
// bits: short enough for a readable URL, far beyond collision range for the
// handful of projects one person opens.
const projectIDLen = 12

// projectID derives a stable identifier for a project root.
//
// The path is cleaned but NOT resolved through symlinks: two paths that reach
// the same directory by different routes get different ids, which is the
// conservative outcome — a duplicate mount is confusing, a wrong mount serves
// the wrong data. Callers that care pass an already-absolute, cleaned path.
func projectID(absPath string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(absPath)))
	return hex.EncodeToString(sum[:])[:projectIDLen]
}

// loadedProject is one open project: its services, its HTTP handler, and the
// lifecycle handles needed to shut it down independently of its siblings.
type loadedProject struct {
	id      string
	root    string // absolute project directory
	name    string // display name, from the metamodel
	app     *dataentry.App
	svc     *appbuild.Services
	handler http.Handler

	// stopScheduler cancels this project's background scheduler. Held per
	// project so closing one project's last window cannot stop another's.
	stopScheduler context.CancelFunc

	// windows counts open windows showing this project. The scheduler runs
	// only while this is > 0: a schedule exists to run unattended, but running
	// it for a project the user has closed is background work (mail, HTTP, AI)
	// they did not ask for.
	windows int
}

// projectRegistry holds every loaded project, keyed by id.
//
// Its own mutex, not Desktop's: registry lookups happen on every HTTP request,
// and sharing Desktop's lock would serialize them behind project loads.
type projectRegistry struct {
	mu       sync.RWMutex
	projects map[string]*loadedProject

	// active is the id served at the bare root for callers that have not been
	// taught about /p/<id>/ yet — the welcome page's bindings, and any window
	// opened without a prefix. Empty when no project is loaded.
	active string
}

func newProjectRegistry() *projectRegistry {
	return &projectRegistry{projects: make(map[string]*loadedProject)}
}

// get returns the project with this id, or nil.
func (r *projectRegistry) get(id string) *loadedProject {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.projects[id]
}

// byRoot returns the project loaded from this directory, or nil. Used to
// decide whether an Open Project request should focus an existing window
// rather than load a second copy of the same directory.
func (r *projectRegistry) byRoot(absPath string) *loadedProject {
	return r.get(projectID(absPath))
}

// activeProject returns the project served at the bare root, or nil.
func (r *projectRegistry) activeProject() *loadedProject {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.active == "" {
		return nil
	}
	return r.projects[r.active]
}

// add registers a loaded project and makes it active.
func (r *projectRegistry) add(p *loadedProject) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.projects[p.id] = p
	r.active = p.id
}

// setActive marks id as the project served at the bare root. Unknown ids are
// ignored rather than clearing the active project, so a stale window cannot
// blank the root for everyone.
func (r *projectRegistry) setActive(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.projects[id]; ok {
		r.active = id
	}
}

// remove unregisters a project and returns it so the caller can release it
// outside the lock — closing a store is slow and must not block request
// routing for other projects.
func (r *projectRegistry) remove(id string) *loadedProject {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.projects[id]
	if p == nil {
		return nil
	}
	delete(r.projects, id)
	if r.active == id {
		r.active = ""
		// Fall back to any remaining project so the root keeps working.
		for otherID := range r.projects {
			r.active = otherID
			break
		}
	}
	return p
}

// all returns every loaded project, for the switcher.
func (r *projectRegistry) all() []*loadedProject {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*loadedProject, 0, len(r.projects))
	for _, p := range r.projects {
		out = append(out, p)
	}
	return out
}

// retainWindow records that a window is showing this project.
func (r *projectRegistry) retainWindow(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p := r.projects[id]; p != nil {
		p.windows++
	}
}

// releaseWindow drops a window reference and reports whether the project now
// has none, so the caller can stop its scheduler.
func (r *projectRegistry) releaseWindow(id string) (last bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.projects[id]
	if p == nil {
		return false
	}
	if p.windows > 0 {
		p.windows--
	}
	return p.windows == 0
}

// projectPrefix is the URL space each project is mounted under.
const projectPrefix = "/p/"

// splitProjectPath separates a /p/<id>/... request into its project id and the
// path within that project. It reports ok=false for any path outside the
// prefix, which the caller serves from the active project instead.
//
// The returned path always begins with "/" so it can be handed to a mux
// unchanged: "/p/abc" (no trailing slash) yields "/", not "".
func splitProjectPath(p string) (id, rest string, ok bool) {
	if !strings.HasPrefix(p, projectPrefix) {
		return "", "", false
	}
	trimmed := strings.TrimPrefix(p, projectPrefix)
	id, rest, found := strings.Cut(trimmed, "/")
	if id == "" {
		return "", "", false
	}
	if !found {
		return id, "/", true
	}
	return id, "/" + rest, true
}

// projectsPath is the desktop shell's own endpoint, served ahead of any
// project's router.
//
// It lives here rather than in internal/dataentry because the registry is a
// property of the shell: rela-server will answer the same question from its
// own configuration, and dataentry has no business knowing how a host mounts
// projects.
const projectsPath = "/api/v1/_projects"

// projectSummary is one entry in the switcher.
type projectSummary struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Root   string `json:"root"`
	Href   string `json:"href"`
	Active bool   `json:"active"`
}

// serveProjects lists the open projects, most recently opened last, for the
// switcher to render. Sorted by name so the order does not shift under the
// user as projects are opened and closed.
func (d *Desktop) serveProjects(w http.ResponseWriter, _ *http.Request) {
	all := d.registry.all()
	active := d.registry.activeProject()

	out := make([]projectSummary, 0, len(all))
	for _, p := range all {
		out = append(out, projectSummary{
			ID:     p.id,
			Name:   p.name,
			Root:   p.root,
			Href:   projectPrefix + p.id + "/",
			Active: active != nil && active.id == p.id,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID // stable when two projects share a name
	})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		slog.Debug("could not write project list", "error", err)
	}
}
