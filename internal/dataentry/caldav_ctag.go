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
// property it does not know, with a real value for each collection. It fixes
// both the element and its enclosing propstat status, and treats each
// <response> independently by its own <href>, so a Depth:1 home-set request
// gets ctags on its member collections. It edits the string rather than
// re-serializing, so go-webdav's namespace prefixes and order are kept.
func spliceCtag(body []byte, ctagFor func(href string) (string, bool)) []byte {
	s := string(body)
	var out strings.Builder
	out.Grow(len(s))

	for s != "" {
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
		return resp
	}

	filled := `<getctag xmlns="` + ctagNamespace + `">` + ctag + `</getctag>`
	for _, empty := range []string{
		`<getctag xmlns="` + ctagNamespace + `"></getctag>`,
		`<getctag xmlns="` + ctagNamespace + `"/>`,
	} {
		resp = strings.Replace(resp, empty, filled, 1)
	}

	return retagPropstatStatus(resp, filled)
}

// retagPropstatStatus rewrites the <status> of whichever propstat contains
// marker, from 404 to 200.
func retagPropstatStatus(resp, marker string) string {
	at := strings.Index(resp, marker)
	if at < 0 {
		return resp
	}
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
// getctag gets one. The ctag is computed from the same rendered objects the
// collection serves, so it changes exactly when a client would see different
// bytes. A change counter would be cheaper but can skip dropped store events.
// Computing it renders the collection, so it is not free on the server.
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
			seen[name] = ""
			return "", false
		}
		seen[name] = ctag
		return ctag, true
	}
}
