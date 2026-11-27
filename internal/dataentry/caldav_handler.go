package dataentry

import (
	"io"
	"net/http"

	"github.com/emersion/go-webdav/caldav"
)

// caldavRoutes owns the CalDAV HTTP surface: route registration, the two
// middleware wrappers, and the site-root probe.
//
// It exists so those handlers hang off a focused type rather than accreting on
// App. They need almost nothing from it — the alias service to decide whether
// to register at all, and App itself only as the backend's collaborator — so
// keeping them as App methods grew the god-object load line (`just plimsoll`)
// for no gain in cohesion. The wiring site constructs one and calls it; see
// App.registerCalDAVRoutes.
type caldavRoutes struct {
	app *App
}

// newCalDAVRoutes returns the CalDAV route set for an app, or nil when no
// alias service is wired.
//
// nil means "CalDAV is not configured": serving collections with no way to
// remember client-created resources would duplicate every to-do on the next
// sync, so the absence of the alias service disables the whole surface rather
// than degrading it.
func newCalDAVRoutes(a *App) *caldavRoutes {
	if a == nil || a.caldavAliases == nil {
		return nil
	}
	return &caldavRoutes{app: a}
}

// register mounts the CalDAV server on the inner /api/ mux.
//
// Registered on the INNER /api/ mux so the endpoint inherits the whole existing
// chain: attachACLRequest, the read gate, requireVerifiedJWT (which carries the
// upstream identity assertion) and principal stamping. CalDAV gets no bespoke
// gating of its own — it is an /api/ route like any other, which is the entire
// reason for mounting it here.
//
// Nothing is registered when no `caldav:` collections are declared, or when no
// alias service is wired: serving collections with no way to remember
// client-created resources would duplicate every to-do on the next sync.
func (c *caldavRoutes) register(mux *http.ServeMux) {
	mux.Handle(caldavPathPrefix, withCalDAVBodyLimit(c.withPropPatch(c.withCTag(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			h := &caldav.Handler{
				Backend: &caldavBackend{app: c.app, baseURL: feedBaseURL(r)},
				Prefix:  caldavPathPrefix,
			}
			h.ServeHTTP(w, r)
		})))))
}

// maxCalDAVBodyBytes caps every CalDAV request body.
//
// # Why this exists: an unbounded body is a remote process KILL
//
// go-ical's decoder recurses once per `BEGIN:` line with no depth limit
// (upstream carries a `// TODO: check CALDAV:max-resource-size precondition`),
// so a body of repeated `BEGIN:A` recurses until the goroutine stack is
// exhausted. Reproduced against this tree:
//
//	 9 MB  -> parses, returns EOF
//	27 MB  -> runtime: goroutine stack exceeds 1000000000-byte limit
//	          fatal error: stack overflow
//
// That is a FATAL ERROR, not a panic: `recover()` cannot catch it and the whole
// rela-server process dies, taking every other user's session with it. One
// request, repeatable after each restart.
//
// The auth chain does not save us. CalDAV is mounted under `/api/` so a valid
// assertion is required — which makes this an escalation, not a mitigation: a
// principal permitted to read NOTHING can still kill the server for everyone.
// The 30s ReadTimeout does not help either; 27 MB arrives in well under a
// second on a LAN.
//
// # Why 1 MiB
//
// Observed real bodies: an Apple client-created VTODO is 298 bytes, a completed
// one 630, and the largest PROPFIND a client sends is a couple of KB. 1 MiB is
// ~3000x headroom for legitimate traffic while sitting ~20x below the crash
// threshold — the gap matters, because a depth bomb is compact enough that a
// cap set close to the threshold would not actually prevent it.
//
// Fixed rather than configurable: there is no plausible deployment that needs a
// larger CalDAV body, and a knob here would only be a way to reintroduce the
// crash. (Contrast `max_attachment_bytes`, which is configurable because upload
// sizes genuinely vary per deployment.)
const maxCalDAVBodyBytes = 1 << 20

// withCalDAVBodyLimit caps the request body for the whole CalDAV surface.
//
// Applied at the OUTERMOST layer so every method inherits it — PUT, PROPFIND,
// PROPPATCH, REPORT and anything go-webdav adds later. Wrapping an inner
// handler instead would protect only the methods someone remembered, which is
// how CalDAV came to inherit the auth chain but none of the size discipline
// that `internal/dataentry` already applies to attachments, themes and logos.
//
// http.MaxBytesReader (rather than io.LimitReader) so an over-large body fails
// the read with *http.MaxBytesError instead of being silently TRUNCATED into a
// valid-looking shorter document.
func withCalDAVBodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxCalDAVBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// caldavWellKnownPath is the RFC 6764 bootstrap URL. A client is configured
// with a bare server address and looks here first to find the real CalDAV root.
const caldavWellKnownPath = "/.well-known/caldav"

// registerWellKnown mounts the RFC 6764 discovery redirect on the OUTER mux,
// because the path is fixed at the site root. It is not gated: it reveals
// only that the server speaks CalDAV, and the target carries the real gates.
func (c *caldavRoutes) registerWellKnown(mux *http.ServeMux, spa http.Handler) {
	mux.HandleFunc(caldavWellKnownPath, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, caldavPathPrefix, http.StatusMovedPermanently)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" && r.URL.Path == "/" {
			c.serveRootPrincipalProbe(w)
			return
		}
		spa.ServeHTTP(w, r)
	})
}

// serveRootPrincipalProbe answers a site-root PROPFIND with where the
// principal and its calendars live. It is hand-written because go-webdav
// cannot answer outside its Prefix. It returns both properties because
// Apple's accountsd never issues the follow-up probe. Unsupported properties
// are omitted, as RFC 4918 allows.
func (c *caldavRoutes) serveRootPrincipalProbe(w http.ResponseWriter) {
	principal := caldavPathPrefix + caldavPrincipalSegment + "/"
	home := principal + caldavHomeSegment + "/"

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("DAV", "1, 3, calendar-access")
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>`+
		`<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">`+
		`<response><href>/</href><propstat><prop>`+
		`<current-user-principal><href>`+principal+`</href></current-user-principal>`+
		`<C:calendar-home-set><href>`+home+`</href></C:calendar-home-set>`+
		`</prop><status>HTTP/1.1 200 OK</status></propstat></response></multistatus>`)
}
