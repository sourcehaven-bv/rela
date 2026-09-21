package dataentry

import (
	"context"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/visibility"
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
	schemaFile func() string
	// executeView runs a view for `kind: view` commands. The viewWorld
	// parameter is passed explicitly by the caller (always defaultViewWorld()
	// here — see commands.go), never read from ctx: this handler's output
	// leaves the process.
	executeView func(ctx context.Context, view ViewConfig, entryID string, w viewWorld) (*viewResult, error)

	// authz decides whether a command may execute (TKT-MJ02AO, TKT-AQIT9M).
	// It is chosen ONCE at the wiring site from (ACL, bind, override) — see
	// SelectCommandAuthorizer — because the decision needs the bind address,
	// which lives in cmd/rela-server, not in App. Unlike the old aclImpl
	// closure, this is a value: the authorizer is fixed for the process
	// lifetime (bind and policy don't change under a running server). Tests
	// that need a different verdict assign app.commands.authz directly rather
	// than reassigning app.acl (RR-CWBZVT). A nil authz is treated as deny by
	// the accessor, so a wiring omission fails closed.
	authz commandAuthorizer

	// redactor applies field-level `visible:` redaction to an ENTITY-context
	// payload (BUG-G2BASF). The view context gets this for free — its
	// viewResult arrives already row-gated and redacted from executeView
	// (see buildViewInput) — but the entity context reads the store
	// directly, so it owes the entity the same treatment.
	//
	// A closure over App for the same reason as the other fields: tests
	// rebind the affordance service after construction.
	redactor visibility.FieldRedactor
}

// authorizer returns the wired command authorizer, or a denyAuthorizer when the
// handler was constructed without one. A wiring omission has to fail closed —
// never panic, never grant.
func (h *commandHandler) authorizer() commandAuthorizer {
	if h.authz == nil {
		return denyAuthorizer{}
	}
	return h.authz
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
