package dataentry

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// The collection ctag (`http://calendarserver.org/ns/:getctag`) is the cheap
// poll every CalDAV client makes: fetch one property, and if it is unchanged
// since last time, skip enumerating the collection entirely.
//
// It is not in any RFC — it predates RFC 6578 sync-collection and is a
// CalendarServer extension — but it is what Apple Reminders actually polls, and
// go-webdav has no support for it (no mention of "ctag" anywhere in v0.7.0) and
// no seam to add one: propFindCalendar builds a closed map[xml.Name]PropFindFunc.
//
// So this wraps the handler rather than extending it. The wrapper answers a
// PROPFIND that asks for getctag by delegating to go-webdav and splicing the
// property into the collection's <prop> element. Everything else — every other
// method, every other property, resource-level PROPFIND — passes through
// untouched, so go-webdav stays the single implementation of the protocol.

// ctagNamespace is the CalendarServer extension namespace.
const ctagNamespace = "http://calendarserver.org/ns/"

// ctagRequested reports whether a PROPFIND body asks for getctag.
//
// The body is consumed and returned, since a handler downstream still needs it.
func ctagRequested(r *http.Request) (wanted bool, body []byte, err error) {
	if r.Body == nil {
		return false, nil, nil
	}
	body, err = io.ReadAll(r.Body)
	if err != nil {
		return false, nil, err
	}
	_ = r.Body.Close()

	// An empty body means allprop. Clients that want the ctag ask for it by
	// name, and allprop deliberately does NOT include it: it is an extension,
	// and RFC 4918 8.1 excludes expensive/extension properties from allprop.
	// Computing it there would make every bare PROPFIND render the collection.
	if len(body) == 0 {
		return false, body, nil
	}

	dec := xml.NewDecoder(strings.NewReader(string(body)))
	for {
		tok, tokErr := dec.Token()
		if tokErr != nil {
			return false, body, nil //nolint:nilerr // a malformed body is go-webdav's to reject, not ours
		}
		if se, ok := tok.(xml.StartElement); ok {
			if se.Name.Space == ctagNamespace && se.Name.Local == "getctag" {
				return true, body, nil
			}
		}
	}
}

// spliceCtag replaces the empty, 404-status getctag go-webdav emits for the
// property it does not know, with a real value for each collection.
//
// go-webdav renders an unknown requested property as an EMPTY element inside a
// propstat carrying `404 Not Found` — so simply inserting a second getctag
// elsewhere would leave the client choosing between two, one of them empty and
// explicitly not-found. Both the element and its enclosing status have to be
// fixed.
//
// This walks response elements and treats each independently, keyed on its own
// <href>. That is what makes the Depth:1 home-set request work — the request
// Apple Reminders actually sends (verified on the wire): one multistatus
// carrying the home set AND every member collection, where only the members
// have a ctag. An earlier version spliced into the FIRST <prop> it found, which
// silently did nothing here because the first response is the home set.
//
// String surgery rather than parse-and-re-serialize: re-encoding go-webdav's
// XML risks changing namespace prefixes or element ordering that clients may be
// sensitive to, for no benefit.
func spliceCtag(body []byte, ctagFor func(href string) (string, bool)) []byte {
	s := string(body)
	var out strings.Builder
	// The rewrite only ever grows the body (an empty element gains a value), so
	// start at the input size and let append handle the rest.
	out.Grow(len(s))

	for s != "" {
		// Matches "<response" as a prefix, so it also matches
		// "<responsedescription". That is harmless as go-webdav emits them
		// today (a nested responsedescription always follows its enclosing
		// <response>, so the outer element is found first), but it is an
		// accident of ordering rather than a property we established: a
		// multistatus-level responsedescription would be matched here. The
		// href lookup below is what makes it safe — an element with no
		// <href> is returned untouched.
		i := strings.Index(s, "<response")
		if i < 0 {
			out.WriteString(s)
			break
		}
		end := strings.Index(s[i:], "</response>")
		if end < 0 {
			out.WriteString(s)
			break
		}
		end += i + len("</response>")

		out.WriteString(s[:i])
		out.WriteString(rewriteResponseCtag(s[i:end], ctagFor))
		s = s[end:]
	}
	return []byte(out.String())
}

// rewriteResponseCtag fixes the getctag inside ONE <response> element.
func rewriteResponseCtag(resp string, ctagFor func(href string) (string, bool)) string {
	href, ok := firstElementText(resp, "href")
	if !ok {
		return resp
	}
	ctag, ok := ctagFor(href)
	if !ok {
		return resp // not a collection we can tag; leave go-webdav's answer alone
	}

	// Give the property a value...
	filled := `<getctag xmlns="` + ctagNamespace + `">` + ctag + `</getctag>`
	for _, empty := range []string{
		`<getctag xmlns="` + ctagNamespace + `"></getctag>`,
		`<getctag xmlns="` + ctagNamespace + `"/>`,
	} {
		resp = strings.Replace(resp, empty, filled, 1)
	}

	// ...and correct the propstat that declared it missing. Only the propstat
	// containing the ctag is touched: a response can carry several, and the
	// others legitimately report 404 for properties we really do not have.
	return retagPropstatStatus(resp, filled)
}

// retagPropstatStatus rewrites the <status> of whichever propstat contains
// marker, from 404 to 200.
func retagPropstatStatus(resp, marker string) string {
	at := strings.Index(resp, marker)
	if at < 0 {
		return resp
	}
	// The status belongs to the propstat this marker sits in, so look only as
	// far as that propstat's end.
	end := strings.Index(resp[at:], "</propstat>")
	if end < 0 {
		return resp
	}
	end += at
	const notFound = "<status>HTTP/1.1 404 Not Found</status>"
	const found = "<status>HTTP/1.1 200 OK</status>"
	seg := strings.Replace(resp[at:end], notFound, found, 1)
	return resp[:at] + seg + resp[end:]
}

func firstElementText(s, name string) (string, bool) {
	open := "<" + name + ">"
	i := strings.Index(s, open)
	if i < 0 {
		return "", false
	}
	i += len(open)
	j := strings.Index(s[i:], "</"+name+">")
	if j < 0 {
		return "", false
	}
	return s[i : i+j], true
}

// ctagResponseWriter buffers a response so its body can be rewritten.
//
// Deliberately does NOT forward Flusher/Hijacker: the body must be complete
// before it can be rewritten, so streaming it through would defeat the purpose.
// PROPFIND has no use for either, and the wrapper is only ever installed on
// PROPFIND (see withCTag).
type ctagResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	buf         strings.Builder
}

// WriteHeader records the FIRST status, matching net/http, where subsequent
// calls are ignored with a warning.
//
// Last-wins was wrong and reachable: go-webdav's ServeMultiStatus writes 207
// and streams, and an encoder error mid-stream returns to Handler.ServeHTTP,
// which calls ServeError -> http.Error -> WriteHeader(500). Recording the 500
// then skipped the ctag splice (status != 207) and emitted the truncated XML
// with a plaintext error line appended to it.
func (w *ctagResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
}

func (w *ctagResponseWriter) Write(p []byte) (int, error) { return w.buf.WriteString(string(p)) }

// withCTag wraps the CalDAV handler so a collection PROPFIND that asks for
// getctag gets one.
//
// The ctag is computed from the SAME rendered objects the collection would
// serve (calfeed.ICal.CollectionTag over each entry's ETag), so it changes if
// and only if a client would see different bytes — including for a deletion,
// which removes an entry from the hash. Deriving it from a change counter or
// mtime would be cheaper and wrong: store events are dropped when a
// subscriber's buffer fills, and fsstore has no monotonic sequence, so a
// counter can silently skip a change and leave a client permanently stale.
//
// COST: computing it renders the collection, which is exactly the work the
// ctag exists to let clients skip. It is still a large win — the client
// transfers one property instead of every VTODO, and skips the REPORT that
// follows — but it is not free on the server. Making it genuinely cheap needs a
// stored per-collection tag maintained on write (TKT-WAA092's postgres variant
// is the natural home), not a different hash here.
func (c *caldavRoutes) withCTag(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" || !strings.HasPrefix(r.URL.Path, caldavPathPrefix) {
			next.ServeHTTP(w, r)
			return
		}
		wanted, body, err := ctagRequested(r)
		if err != nil {
			http.Error(w, "read request body", http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		if !wanted {
			next.ServeHTTP(w, r)
			return
		}

		rec := &ctagResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		out := []byte(rec.buf.String())
		if rec.status == http.StatusMultiStatus {
			out = spliceCtag(out, c.ctagResolver(r.Context(), feedBaseURL(r)))
		}
		// The body is XML built from the collection's own content. Pin the type
		// and forbid sniffing so a user-authored title can never be rendered as
		// HTML by a browser that reached this endpoint.
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Length", strconv.Itoa(len(out)))
		w.WriteHeader(rec.status)
		// #nosec G705 -- not an HTML sink: the body is go-webdav's XML, whose
		// text nodes it escapes via encoding/xml, plus a ctag that is base32 of
		// a hash. Served as application/xml under nosniff (set above).
		_, _ = w.Write(out)
	})
}

// ctagResolver maps a response href to that collection's ctag.
//
// Returns false for anything that is not a collection — the home set, the
// principal, an individual resource — so those responses are left exactly as
// go-webdav wrote them. Results are memoized per request because a Depth:1
// home-set PROPFIND asks about every collection at once and each computation
// renders one.
func (c *caldavRoutes) ctagResolver(ctx context.Context, baseURL string) func(string) (string, bool) {
	// Same baseURL the render path uses: the ctag hashes per-resource ETags, and
	// those cover the URL property, so a resolver with a different base would
	// compute a tag that never matches what is served.
	b := &caldavBackend{app: c.app, baseURL: baseURL}
	seen := map[string]string{}
	return func(href string) (string, bool) {
		name, resource, ok := b.splitPath(href)
		if !ok || name == "" || resource != "" {
			return "", false
		}
		if ctag, hit := seen[name]; hit {
			return ctag, ctag != ""
		}
		ctag, err := b.collectionCTag(ctx, name)
		if err != nil {
			// A collection we cannot render is not one we can tag. The client
			// simply does not get the optimisation for it.
			seen[name] = ""
			return "", false
		}
		seen[name] = ctag
		return ctag, true
	}
}
