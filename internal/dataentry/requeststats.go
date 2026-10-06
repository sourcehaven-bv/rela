package dataentry

import (
	"fmt"
	"log/slog"
	"net/http"
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
func requestStats(next http.Handler, accessLog *slog.Logger) http.Handler {
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
			attrs := requestAttrs(r, sw, completed, time.Since(start))
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

func requestAttrs(r *http.Request, sw *statsResponseWriter, completed bool, wall time.Duration) []slog.Attr {
	method := r.Method
	if len(method) > maxLoggedMethod {
		method = "OTHER"
	}
	path, truncated := truncateUTF8(r.URL.Path, maxLoggedPath)
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
