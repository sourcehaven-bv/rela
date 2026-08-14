package dataentry

import (
	"context"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// commandHandler serves the user-configured command surface: the SSE-streaming
// shell-exec endpoints (/api/command/, /api/command-cancel/), the file/URL
// launchers (/api/open-file, /api/open-url), and the command-resolution query
// behind /api/v1/_commands. Extracted from App (TKT-R68TV8) to shrink the god
// object.
//
// Its collaborators are supplied as narrow closures over App (consumer-side
// interfaces per CLAUDE.md) rather than a handle to App itself:
//
//   - schema yields the current Schema snapshot (command/list/view config).
//   - services yields the read Services bundle passed to the store helpers.
//   - projectRoot is the exec cwd + the RELA_PROJECT_ROOT env base.
//   - executeView runs a view (a views-cluster method) to assemble the stdin
//     payload for a view-context command.
//
// The handler owns no mutable state of its own; runningCommands (the in-flight
// exec registry) stays a package-level sync.Map shared with handleCommandCancel.
type commandHandler struct {
	schema      func() *Schema
	services    func() Services
	projectRoot func() string
	// schemaFile yields the resolved schema basename for the command stdin
	// payload. Optional: nil falls back to the canonical name.
	schemaFile  func() string
	executeView func(ctx context.Context, view ViewConfig, entryID string) (*viewResult, error)

	// aclImpl yields the active ACL, consulted by authorizeCommand to gate
	// execution (TKT-MJ02AO). A closure, not a value, for the same reason as
	// the other fields and as app.go's affordance wiring: tests reassign
	// app.acl AFTER construction, so a captured value would go stale and the
	// handler would authorize against the wrong policy.
	aclImpl func() acl.ACL
}

// currentACL resolves the active ACL, or nil when the handler was constructed
// without the aclImpl closure. Callers must treat nil as deny (see
// authorizeCommand) — a wiring omission has to fail closed, not panic and not
// grant.
func (h *commandHandler) currentACL() acl.ACL {
	if h.aclImpl == nil {
		return nil
	}
	return h.aclImpl()
}

// registerCommandRoutes mounts the command-exec and launcher endpoints. The
// command-resolution query is mounted separately under /api/v1/ by the v1
// router (handleV1Commands delegates to h.resolve). handleOpenURL is a method
// on the handler (exercised by tests) but, matching the pre-extraction router,
// is not mounted here.
func (h *commandHandler) registerCommandRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/command/", h.handleCommandExec)
	mux.HandleFunc("/api/command-cancel/", h.handleCommandCancel)
	mux.HandleFunc("/api/open-file", h.handleOpenFile)
}
