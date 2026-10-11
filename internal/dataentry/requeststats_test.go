package dataentry

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// withLogLevel installs a default slog logger at the given level writing to
// the returned buffer, restoring the previous default on cleanup.
func withLogLevel(t *testing.T, level slog.Level) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: level})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// countingHandler simulates a store-backed handler: it records two
// statements against the request's stats (as the pgx tracer would) and
// writes a body.
func countingHandler(t *testing.T, wantStats bool) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stats := store.QueryStatsFrom(r.Context())
		if wantStats != (stats != nil) {
			t.Errorf("stats on context = %v, want present=%v", stats != nil, wantStats)
		}
		if stats != nil {
			stats.Record(1500 * time.Microsecond)
			stats.Record(500 * time.Microsecond)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestRequestStats_DebugEmitsHeaderAndLog(t *testing.T) {
	buf := withLogLevel(t, slog.LevelDebug)
	h := requestStats(countingHandler(t, true), nil, ticketsWords)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/tickets?page=2", http.NoBody))

	if got, want := rec.Header().Get("Server-Timing"), `db;dur=2.0;desc="2 queries"`; got != want {
		t.Errorf("Server-Timing = %q, want %q", got, want)
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("status passthrough = %d, want 201", rec.Code)
	}
	out := buf.String()
	for _, want := range []string{"msg=request", "method=GET", "path=/api/v1/tickets", "status=201", "queries=2", "db_ms=2", "wall_ms="} {
		if !strings.Contains(out, want) {
			t.Errorf("request log missing %q in %q", want, out)
		}
	}
}

func TestRequestStats_InfoIsPassThrough(t *testing.T) {
	buf := withLogLevel(t, slog.LevelInfo)
	h := requestStats(countingHandler(t, false), nil, nil)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody))

	if got := rec.Header().Get("Server-Timing"); got != "" {
		t.Errorf("Server-Timing must be absent below Debug, got %q", got)
	}
	if buf.Len() != 0 {
		t.Errorf("no request record below Debug, got %q", buf.String())
	}
}

// accessLogger returns an Info-level logger writing to the returned buffer,
// standing in for the --access-log sink.
func accessLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, nil)), &buf
}

// --access-log: the record goes to the access logger at Info, nothing goes
// to the application log, and the client never sees the query count.
func TestRequestStats_AccessLogAtInfoWithoutHeader(t *testing.T) {
	appLog := withLogLevel(t, slog.LevelInfo)
	logger, buf := accessLogger()
	h := requestStats(countingHandler(t, true), logger, ticketsWords)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/tickets?token=secret", http.NoBody))

	if got := rec.Header().Get("Server-Timing"); got != "" {
		t.Errorf("access log must not add Server-Timing, got %q", got)
	}
	out := buf.String()
	for _, want := range []string{"level=INFO", "msg=request", "method=GET", "path=/api/v1/tickets ", "status=201", "queries=2", "db_ms=2", "wall_ms="} {
		if !strings.Contains(out, want) {
			t.Errorf("access log missing %q in %q", want, out)
		}
	}
	if strings.Contains(out, "secret") {
		t.Errorf("access log must not contain the query string, got %q", out)
	}
	if appLog.Len() != 0 {
		t.Errorf("application log must stay free of request records, got %q", appLog.String())
	}
}

// The access logger is independent of the application level: --quiet
// (Warn) does not silence it.
func TestRequestStats_AccessLogIgnoresQuiet(t *testing.T) {
	withLogLevel(t, slog.LevelWarn)
	logger, buf := accessLogger()
	h := requestStats(countingHandler(t, true), logger, nil)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody))

	if !strings.Contains(buf.String(), "msg=request") {
		t.Errorf("access log empty under Warn, got %q", buf.String())
	}
}

// Debug plus --access-log: the header behaves as under Debug alone, and the
// record goes to both sinks, once each: Debug in the application log (where
// it closes the request's SQL lines) and Info in the access log.
func TestRequestStats_AccessLogUnderDebugLogsToBoth(t *testing.T) {
	appLog := withLogLevel(t, slog.LevelDebug)
	logger, buf := accessLogger()
	h := requestStats(countingHandler(t, true), logger, nil)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody))

	if got := rec.Header().Get("Server-Timing"); got == "" {
		t.Error("Debug must still stamp Server-Timing when the access log is on")
	}
	if n := strings.Count(buf.String(), "level=INFO msg=request"); n != 1 {
		t.Errorf("access records = %d, want 1 in %q", n, buf.String())
	}
	if n := strings.Count(appLog.String(), "level=DEBUG msg=request"); n != 1 {
		t.Errorf("debug records = %d, want 1 in %q", n, appLog.String())
	}
}

// A handler that panics still produces a record, marked panic=true, and the
// panic still propagates to net/http. Before any write the status is 500;
// after WriteHeader it is the status the handler sent. The application log
// stays free of request records either way.
func TestRequestStats_AccessLogRecordsPanic(t *testing.T) {
	for _, tc := range []struct {
		name   string
		write  int
		status string
	}{
		{"before write", 0, "status=500"},
		{"after write", http.StatusCreated, "status=201"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			appLog := withLogLevel(t, slog.LevelInfo)
			logger, buf := accessLogger()
			h := requestStats(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.write != 0 {
					w.WriteHeader(tc.write)
				}
				panic("boom")
			}), logger, nil)

			func() {
				defer func() {
					if recover() == nil {
						t.Error("panic must propagate")
					}
				}()
				h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody))
			}()

			for _, want := range []string{"msg=request", tc.status, "panic=true"} {
				if !strings.Contains(buf.String(), want) {
					t.Errorf("record missing %q in %q", want, buf.String())
				}
			}
			if appLog.Len() != 0 {
				t.Errorf("application log must stay free of request records, got %q", appLog.String())
			}
		})
	}
}

// A path over maxLoggedPath is cut and flagged, so one oversized request
// cannot produce a record syslog drops or the journal bloats on.
func TestRequestStats_AccessLogTruncatesLongPath(t *testing.T) {
	withLogLevel(t, slog.LevelInfo)
	logger, buf := accessLogger()
	h := requestStats(countingHandler(t, true), logger, nil)

	long := "/api/v1/" + strings.Repeat("a/", maxLoggedPath)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, long, http.NoBody))

	shape, _ := routeShape(long, nil, len(long))
	out := buf.String()
	if !strings.Contains(out, "path="+shape[:maxLoggedPath]+" ") || strings.Contains(out, shape[:maxLoggedPath+1]) {
		t.Errorf("path not cut at %d bytes in %q", maxLoggedPath, out)
	}
	if !strings.Contains(out, "path_truncated=true") {
		t.Errorf("truncation not flagged in %q", out)
	}
}

// The cut never splits a rune: a multi-byte character straddling the limit
// is dropped whole, so the value stays valid UTF-8.
func TestTruncateUTF8_KeepsRunesWhole(t *testing.T) {
	s := strings.Repeat("a", maxLoggedPath-1) + "é" + "tail"
	got, cut := truncateUTF8(s, maxLoggedPath)
	if !cut || got != s[:maxLoggedPath-1] || !utf8.ValidString(got) {
		t.Errorf("truncateUTF8 = %q (len %d), cut=%v; want the %d bytes before é", got[len(got)-3:], len(got), cut, maxLoggedPath-1)
	}
	if got, cut := truncateUTF8("short", maxLoggedPath); got != "short" || cut {
		t.Errorf("short input changed: %q, %v", got, cut)
	}
}

// An oversized method (any token is valid HTTP) is logged as OTHER.
func TestRequestStats_AccessLogCapsMethod(t *testing.T) {
	withLogLevel(t, slog.LevelInfo)
	logger, buf := accessLogger()
	h := requestStats(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), logger, nil)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody)
	r.Method = strings.Repeat("X", maxLoggedMethod+1)
	h.ServeHTTP(httptest.NewRecorder(), r)

	if !strings.Contains(buf.String(), "method=OTHER ") || strings.Contains(buf.String(), r.Method) {
		t.Errorf("method not capped in %q", buf.String())
	}
}

// A 103 Early Hints before the final status is not the request's status.
func TestRequestStats_InformationalStatusIsNotFinal(t *testing.T) {
	withLogLevel(t, slog.LevelInfo)
	logger, buf := accessLogger()
	h := requestStats(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusEarlyHints)
		w.WriteHeader(http.StatusAccepted)
	}), logger, nil)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody))

	if !strings.Contains(buf.String(), "status=202") {
		t.Errorf("want final status 202 in %q", buf.String())
	}
}

// An event stream with only the access log on: no header, one record when
// the stream ends.
func TestRequestStats_AccessLogSSE(t *testing.T) {
	withLogLevel(t, slog.LevelInfo)
	logger, buf := accessLogger()
	h := requestStats(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.(http.Flusher).Flush()
	}), logger, nil)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/_events", http.NoBody))

	if got := rec.Header().Get("Server-Timing"); got != "" {
		t.Errorf("Server-Timing on SSE = %q", got)
	}
	if n := strings.Count(buf.String(), "msg=request"); n != 1 {
		t.Errorf("records = %d, want 1 in %q", n, buf.String())
	}
}

func TestRequestStats_SSEGetsNoHeaderButFlushes(t *testing.T) {
	withLogLevel(t, slog.LevelDebug)
	flushed := false
	h := requestStats(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("wrapped writer must still satisfy http.Flusher for SSE")
		}
		_, _ = w.Write([]byte("data: x\n\n"))
		f.Flush()
		flushed = true
	}), nil, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/_events", http.NoBody))
	if !flushed {
		t.Fatal("handler did not run to Flush")
	}
	if got := rec.Header().Get("Server-Timing"); got != "" {
		t.Errorf("SSE response must not carry Server-Timing, got %q", got)
	}
	if !rec.Flushed {
		t.Error("Flush was not forwarded to the underlying writer")
	}
}

func TestRequestStats_ImplicitWriteHeaderStillStamps(t *testing.T) {
	withLogLevel(t, slog.LevelDebug)
	h := requestStats(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		store.QueryStatsFrom(r.Context()).Record(time.Millisecond)
		_, _ = w.Write([]byte("body without explicit WriteHeader"))
	}), nil, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/_schema", http.NoBody))
	if got, want := rec.Header().Get("Server-Timing"), `db;dur=1.0;desc="1 queries"`; got != want {
		t.Errorf("Server-Timing = %q, want %q", got, want)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("implicit status = %d, want 200", rec.Code)
	}
}

// The router wires requestStats outermost; prove a real API route through
// NewRouter carries the header under Debug and not under Info.
func TestRequestStats_WiredIntoRouter(t *testing.T) {
	for _, tc := range []struct {
		name  string
		level slog.Level
		want  bool
	}{
		{"debug", slog.LevelDebug, true},
		{"info", slog.LevelInfo, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withLogLevel(t, tc.level)
			app := newTestAppV1(t)
			router := app.NewRouter()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/_schema", http.NoBody).WithContext(context.Background())
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d", rec.Code)
			}
			if got := rec.Header().Get("Server-Timing") != ""; got != tc.want {
				t.Errorf("Server-Timing present = %v, want %v", got, tc.want)
			}
		})
	}
}

// WithAccessLog reaches the middleware through NewRouter: a real API route
// at Info logs a request record and still sends no header.
func TestRequestStats_AccessLogWiredIntoRouter(t *testing.T) {
	withLogLevel(t, slog.LevelInfo)
	logger, buf := accessLogger()
	app := newTestAppV1(t)
	router := app.NewRouter(WithAccessLog(logger))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/_schema", http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if got := rec.Header().Get("Server-Timing"); got != "" {
		t.Errorf("access log must not add Server-Timing, got %q", got)
	}
	if !strings.Contains(buf.String(), "msg=request method=GET path=/api/v1/_schema status=200") {
		t.Errorf("no request record in %q", buf.String())
	}
}

// ticketsWords knows the one schema word the timing tests request.
func ticketsWords() routeWords { return routeWords{"tickets": true} }

// TestRouteShape pins the allowlist (GitHub #1782): route words and schema
// or config names survive, every other segment (entity ids, attachment file
// names) becomes "*".
func TestRouteShape(t *testing.T) {
	words := routeWords{"tickets": true, "ticket": true, "blocks": true, "ticket_detail": true, "screenshot": true}
	for _, tc := range []struct{ path, want string }{
		{"/api/v1/tickets", "/api/v1/tickets"},
		{"/api/v1/tickets/TKT-1", "/api/v1/tickets/*"},
		{"/api/v1/tickets/TKT-1@draft/_attachments/screenshot/ziekmelding.pdf", "/api/v1/tickets/*/_attachments/screenshot/*"},
		{"/api/v1/tickets/TKT-1/relations/blocks/TKT-2", "/api/v1/tickets/*/relations/blocks/*"},
		{"/api/v1/_views/ticket_detail/TKT-1", "/api/v1/_views/ticket_detail/*"},
		{"/api/v1/_views/secret_name/TKT-1", "/api/v1/_views/*/*"},
		{"/api/v1/_schema/types/ticket", "/api/v1/_schema/types/ticket"},
		{"/api/v1/unknown_plural/x", "/api/v1/*/*"},
		{"/api/v1/tickets/a%2Fb.pdf", "/api/v1/tickets/*"},
		{"/api/v1/tickets/../x", "/api/v1/tickets/*/*"},
		// An id equal to a route word is kept: the documented collision rule.
		{"/api/v1/tickets/status", "/api/v1/tickets/status"},
		{"/tickets/TKT-1", "/tickets/*"},
		{"/", "/"},
		{"/api/v1/tickets/", "/api/v1/tickets/"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			if got, _ := routeShape(tc.path, words, maxLoggedPath); got != tc.want {
				t.Errorf("routeShape(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

// A path near the header limit stops being shaped at the cap, so its cost is
// bounded by the cap and not by the path.
func TestRouteShape_StopsAtCap(t *testing.T) {
	long := strings.Repeat("/a", 500_000)
	got, truncated := routeShape(long, nil, maxLoggedPath)
	if !truncated || len(got) > maxLoggedPath {
		t.Fatalf("len %d truncated %v, want <= %d and truncated", len(got), truncated, maxLoggedPath)
	}
	allocs := testing.AllocsPerRun(5, func() { routeShape(long, nil, maxLoggedPath) })
	if allocs > 20 {
		t.Errorf("routeShape allocates %.0f times on a long path; want a small constant", allocs)
	}
}

// TestFixedRouteWords_CoverRegisteredRoutes keeps fixedRouteWords in step
// with the literal segments of every route pattern this package registers.
// A missing word would only cost detail, not privacy, but drift is silent.
func TestFixedRouteWords_CoverRegisteredRoutes(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "HandleFunc" && sel.Sel.Name != "Handle") {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			pattern, _ := strconv.Unquote(lit.Value)
			if _, after, ok := strings.Cut(pattern, " "); ok {
				pattern = after // "POST /hooks/"
			}
			for seg := range strings.SplitSeq(pattern, "/") {
				if seg == "" || strings.HasPrefix(seg, "{") {
					continue
				}
				if !fixedRouteWords[seg] {
					t.Errorf("%s: route segment %q is not in fixedRouteWords", fset.Position(lit.Pos()), seg)
				}
			}
			return true
		})
	}
}

// Through the real router the schema supplies the words: the type's plural
// and the property are logged, the id and file name are not.
func TestRequestStats_AccessLogMasksIdentity(t *testing.T) {
	withLogLevel(t, slog.LevelInfo)
	logger, buf := accessLogger()
	app := newTestAppV1(t)
	router := app.NewRouter(WithAccessLog(logger))
	router.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001/_attachments/title/ziekmelding.pdf", http.NoBody))
	out := buf.String()
	if !strings.Contains(out, "path=/api/v1/tickets/*/_attachments/title/* ") {
		t.Errorf("want the route shape in %q", out)
	}
	if strings.Contains(out, "TKT-001") || strings.Contains(out, "ziekmelding") {
		t.Errorf("identity leaked into %q", out)
	}
}

// The word set follows the schema snapshot: a published schema with a new
// type is picked up, and the cache serves repeat calls without rebuilding.
func TestRouteWordCache_FollowsSnapshot(t *testing.T) {
	meta := testMeta()
	s1 := &Schema{Meta: meta}
	current := s1
	c := &routeWordCache{schema: func() *Schema { return current }}
	if w := c.words(); !w["tickets"] || !w["depends_on"] || !w["title"] {
		t.Fatalf("words = %v, want plural, relation and property", w)
	}
	entry := c.cached.Load()
	c.words()
	if c.cached.Load() != entry {
		t.Fatal("cache rebuilt for the same snapshot")
	}
	current = &Schema{Meta: meta, Cfg: &Config{Views: map[string]ViewConfig{"ticket_detail": {}}}}
	if w := c.words(); !w["ticket_detail"] {
		t.Errorf("new snapshot not picked up: %v", w)
	}
}
