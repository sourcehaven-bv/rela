package dataentry

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

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
// Below Debug it is a pass-through: nothing is attached, nothing is
// emitted, no header is set. That gate is deliberate and security-relevant,
// not a convenience. A per-response query count varies with the rows a
// request touched — on a path that still loads neighbors one by one it
// varies with rows the principal is NOT allowed to see — so it is an
// existence side channel of the kind docs/acl-security.md rules out. Wall
// time is already observable by any client; a machine-readable statement
// count is not, and it stays an operator diagnostic (RR-64OR7D).
//
// SSE responses carry no header: their WriteHeader fires before the stream
// does any work, so the numbers would be meaningless; the log record on
// disconnect is still emitted.
func requestStats(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !slog.Default().Enabled(r.Context(), slog.LevelDebug) {
			next.ServeHTTP(w, r)
			return
		}
		ctx, stats := store.WithQueryStats(r.Context())
		start := time.Now()
		sw := &statsResponseWriter{
			ResponseWriter: w,
			stats:          stats,
			header:         !isSSEPath(r.URL.Path),
		}
		next.ServeHTTP(sw, r.WithContext(ctx))
		slog.DebugContext(ctx, "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.statusOrDefault(),
			"wall_ms", millis(time.Since(start)),
			"queries", stats.Queries(),
			"db_ms", millis(stats.Duration()),
		)
	})
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
// Unwrap serves http.ResponseController for any other optional interface.
type statsResponseWriter struct {
	http.ResponseWriter
	stats   *store.QueryStats
	header  bool
	status  int
	written bool
}

func (w *statsResponseWriter) WriteHeader(code int) {
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
