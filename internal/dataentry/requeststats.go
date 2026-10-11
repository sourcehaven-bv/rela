package dataentry

import (
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// requestStats is the outermost middleware. When slog's default handler is
// enabled at Debug it attaches a [store.QueryStats] to the request context,
// so a database-backed store accounts every statement the request issues,
// and reports the result twice: a `Server-Timing` response header
// (`db;dur=<ms>;desc="<n> queries"`, visible in browser devtools) and one
// `request` log record with method, path, status, wall time, query count
// and database time.
//
// Below Debug, and without an access logger, it is a pass-through: nothing
// is attached, nothing is emitted, no header is set. That gate is deliberate
// and security-relevant, not a convenience. A per-response query count
// varies with the rows a request touched — on a path that still loads
// neighbors one by one it varies with rows the principal is NOT allowed to
// see — so it is an existence side channel of the kind docs/acl-security.md
// rules out. Wall time is already observable by any client; a
// machine-readable statement count is not, and it stays an operator
// diagnostic (RR-64OR7D).
//
// SSE responses carry no header: their WriteHeader fires before the stream
// does any work, so the numbers would be meaningless; the log record on
// disconnect is still emitted.
//
// The logged path is the route shape, not the path itself: see
// [routeShape]. That holds for the Debug copy too; a developer who needs the
// raw path has it in the SQL arguments `--verbose` already logs.
//
// accessLog (--access-log) is a separate logger that receives the same
// `request` record at Info for every request. It is its own sink so an
// operator can collect request timing without Debug, which would also log
// every SQL statement with its bound arguments, and without per-request
// lines burying the application log. Under Debug the record goes to both:
// the Debug copy marks where a request's SQL ends in the debug stream. The
// access log never sets the header: the record goes to the operator, the
// header to the client, and only the header is the side channel described
// above.
//
// The record is written from a defer, so a request whose handler panicked
// is still logged, with panic=true. If the panic came before any response,
// status is 500. That status is recorded, not sent: net/http closes the
// connection, so the client sees a reset (a proxy in front reports 502).
// Consumers counting failures should count it. A deliberate
// panic(http.ErrAbortHandler) or runtime.Goexit is also logged as a panic;
// no handler does either today, and telling them apart would need a
// recover that rewrites the stack net/http prints.
func requestStats(next http.Handler, accessLog *slog.Logger, words func() routeWords) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		debug := slog.Default().Enabled(r.Context(), slog.LevelDebug)
		if !debug && accessLog == nil {
			next.ServeHTTP(w, r)
			return
		}
		ctx, stats := store.WithQueryStats(r.Context())
		start := time.Now()
		sw := &statsResponseWriter{
			ResponseWriter: w,
			stats:          stats,
			header:         debug && !isSSEPath(r.URL.Path),
		}
		completed := false
		defer func() {
			var known routeWords
			if words != nil {
				known = words()
			}
			attrs := requestAttrs(r, sw, completed, time.Since(start), known)
			if debug {
				slog.LogAttrs(ctx, slog.LevelDebug, "request", attrs...)
			}
			if accessLog != nil {
				accessLog.LogAttrs(ctx, slog.LevelInfo, "request", attrs...)
			}
		}()
		next.ServeHTTP(sw, r.WithContext(ctx))
		completed = true
	})
}

// maxLoggedPath and maxLoggedMethod cap client-controlled fields in the
// request record. Go accepts a request line up to the header limit (1 MB),
// and a record that size would be dropped by syslog or bloat the journal.
// Real API paths are far shorter; the longest method rela serves is
// CalDAV's MKCALENDAR.
const (
	maxLoggedPath   = 512
	maxLoggedMethod = 32
)

func requestAttrs(
	r *http.Request, sw *statsResponseWriter, completed bool, wall time.Duration, words routeWords,
) []slog.Attr {
	method := r.Method
	if len(method) > maxLoggedMethod {
		method = "OTHER"
	}
	path, truncated := routeShape(r.URL.EscapedPath(), words, maxLoggedPath)
	status := sw.statusOrDefault()
	if !completed && !sw.written {
		status = http.StatusInternalServerError
	}
	attrs := make([]slog.Attr, 0, 8)
	attrs = append(attrs,
		slog.String("method", method),
		slog.String("path", path),
		slog.Int("status", status),
		slog.Float64("wall_ms", millis(wall)),
		slog.Int64("queries", sw.stats.Queries()),
		slog.Float64("db_ms", millis(sw.stats.Duration())),
	)
	if truncated {
		attrs = append(attrs, slog.Bool("path_truncated", true))
	}
	if !completed {
		attrs = append(attrs, slog.Bool("panic", true))
	}
	return attrs
}

// routeWords is the set of names a schema snapshot adds to the route words:
// entity types, their plurals, relations, property names, and the names of
// the data-entry config's views, lists, forms, documents and the like. All of
// it is configuration, which is not a secret, and naming the view or document
// is what makes a slow-route report useful. nil adds nothing.
type routeWords map[string]bool

// routeWordCache builds [routeWords] once per schema snapshot. words takes the
// snapshot once, so a reload mid-request cannot mix two schemas in one line.
type routeWordCache struct {
	schema func() *Schema
	cached atomic.Pointer[routeWordEntry]
}

type routeWordEntry struct {
	schema *Schema
	words  routeWords
}

func (c *routeWordCache) words() routeWords {
	s := c.schema()
	if e := c.cached.Load(); e != nil && e.schema == s {
		return e.words
	}
	w := buildRouteWords(s)
	c.cached.Store(&routeWordEntry{schema: s, words: w})
	return w
}

func buildRouteWords(s *Schema) routeWords {
	w := routeWords{}
	if s == nil {
		return w
	}
	if s.Meta != nil {
		for name, def := range s.Meta.Entities {
			w[name] = true
			w[def.GetPlural(name)] = true
			for prop := range def.Properties {
				w[prop] = true
			}
		}
		for name := range s.Meta.Relations {
			w[name] = true
		}
	}
	if c := s.Cfg; c != nil {
		for _, names := range []iter.Seq[string]{
			maps.Keys(c.Forms), maps.Keys(c.Lists), maps.Keys(c.Views), maps.Keys(c.EntityViews),
			maps.Keys(c.Kanbans), maps.Keys(c.Calendars), maps.Keys(c.Gantts), maps.Keys(c.Documents),
			maps.Keys(c.Feeds), maps.Keys(c.Commands), maps.Keys(c.Actions), maps.Keys(c.Webhooks),
			maps.Keys(c.Pages), maps.Keys(c.NextActions),
		} {
			for name := range names {
				w[name] = true
			}
		}
	}
	return w
}

// fixedRouteWords are the literal segments of rela's own routes.
// TestFixedRouteWords_CoverRegisteredRoutes keeps it in step with the
// registered patterns; a segment missing here is logged as "*", which loses
// detail but discloses nothing.
var fixedRouteWords = map[string]bool{
	"api": true, "v1": true, "v2": true, "static": true, "assets": true, "hooks": true, "webhooks": true,
	"idp": true, "events": true, "command": true, "command-cancel": true, "command-file": true,
	"help": true, "git": true, "status": true, "sync": true, "types": true, "relations": true,
	"file": true, "clone": true, "restore": true, "resolve": true, "export": true, "import": true,
	"logo": true, "accept": true, "principal": true, "calendars": true, ".well-known": true,
	"caldav": true,
	// SPA client routes.
	"list": true, "kanban": true, "calendar": true, "gantt": true, "document": true, "form": true,
	"p": true, "search": true, "settings": true, "dashboard": true,
	"_action": true, "_actions": true, "_analyze": true, "_apps": true, "_attachments": true,
	"_caldav": true, "_commands": true, "_comments": true, "_config": true, "_conflicts": true,
	"_copies": true, "_custom": true, "_dashboard": true, "_documents": true, "_events": true,
	"_export": true, "_feeds": true, "_gantts": true, "_git": true, "_history": true,
	"_lifetimes": true, "_mcp": true, "_me": true, "_nav_items": true, "_nav_status": true,
	"_next_action": true, "_openapi.json": true, "_palette": true, "_position": true,
	"_relation_history": true, "_schema": true, "_search": true, "_settings": true,
	"_sidebar": true, "_sidepanel": true, "_templates": true, "_theme": true,
	"_transforms": true, "_views": true,
}

// routeShape reduces an escaped request path to the shape of its route for
// the request log: a segment that is neither a route word nor in words
// becomes "*". The log is for timing, which does not need the resource's
// identity, and a path carries entity ids and attachment file names that can
// be personal data (GitHub #1782). It is an allowlist, so an id or file name
// that happens to equal a route word or a config name is the only thing it
// lets through.
//
// It stops once the shape passes max bytes and reports truncation, so a path
// near the 1 MB header limit costs no more than a short one.
func routeShape(p string, words routeWords, maxLen int) (string, bool) {
	var b strings.Builder
	for i := 0; ; i++ {
		seg, rest, more := strings.Cut(p, "/")
		if i > 0 {
			b.WriteByte('/')
		}
		if seg == "" || fixedRouteWords[seg] || words[seg] {
			b.WriteString(seg)
		} else {
			b.WriteByte('*')
		}
		if b.Len() > maxLen {
			out, _ := truncateUTF8(b.String(), maxLen)
			return out, true
		}
		if !more {
			return b.String(), false
		}
		p = rest
	}
}

// shapedPath is the request's route shape for an application-log warning
// that has no schema snapshot to hand: route words only, so entity types are
// masked too.
func shapedPath(r *http.Request) string {
	p, _ := routeShape(r.URL.EscapedPath(), nil, maxLoggedPath)
	return p
}

// truncateUTF8 cuts s to at most n bytes without splitting a rune, so the
// logged value stays valid UTF-8 and is not escaped byte by byte.
func truncateUTF8(s string, n int) (string, bool) {
	if len(s) <= n {
		return s, false
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n], true
}

// isSSEPath names the two event-stream routes registered on the outer mux
// (see NewRouter). Kept here rather than derived from the mux so the
// middleware needs no handle on the router.
func isSSEPath(p string) bool {
	return p == "/api/events" || p == "/api/v1/_events"
}

// statsResponseWriter records the status code and, on the first
// WriteHeader, stamps the Server-Timing header from the stats accumulated
// so far. Handlers finish their store reads before writing (they build the
// response body in memory), so the number at WriteHeader time is the
// request's total.
//
// Flush is forwarded so the SSE and command-stream handlers, which assert
// http.Flusher on the writer they receive, keep working when wrapped.
// Unwrap serves http.ResponseController for any other optional interface;
// a direct `w.(http.Hijacker)` assertion would NOT see through this wrapper,
// which is fine only because no handler hijacks — add a forwarder here
// before one does, or Debug mode would change its behavior. A handler that
// writes through the unwrapped writer also bypasses status recording, so
// the record would say 200.
type statsResponseWriter struct {
	http.ResponseWriter
	stats   *store.QueryStats
	header  bool
	status  int
	written bool
}

func (w *statsResponseWriter) WriteHeader(code int) {
	// 1xx responses are informational (103 Early Hints) and do not end the
	// header phase; 101 does and is recorded like a final status.
	if code >= 100 && code < 200 && code != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(code)
		return
	}
	if !w.written {
		w.written = true
		w.status = code
		if w.header {
			w.ResponseWriter.Header().Set("Server-Timing", serverTimingValue(w.stats))
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statsResponseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *statsResponseWriter) Flush() {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statsResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statsResponseWriter) statusOrDefault() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

// serverTimingValue formats one Server-Timing metric. `dur` is the summed
// database time in milliseconds (the unit the header specifies); `desc`
// carries the count because Server-Timing has no other slot for it.
func serverTimingValue(s *store.QueryStats) string {
	return fmt.Sprintf(`db;dur=%.1f;desc="%d queries"`, millis(s.Duration()), s.Queries())
}

// millis renders a duration as fractional milliseconds with microsecond
// resolution — the unit both Server-Timing and the request log use.
func millis(d time.Duration) float64 {
	return float64(d.Microseconds()) / float64(time.Millisecond/time.Microsecond)
}
