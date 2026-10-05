package dataentry

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
	h := requestStats(countingHandler(t, true), nil)

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
	h := requestStats(countingHandler(t, false), nil)

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
	h := requestStats(countingHandler(t, true), logger)

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
	h := requestStats(countingHandler(t, true), logger)

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
	h := requestStats(countingHandler(t, true), logger)

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
			}), logger)

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
	h := requestStats(countingHandler(t, true), logger)

	long := "/api/v1/" + strings.Repeat("a", 2*maxLoggedPath)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, long, http.NoBody))

	out := buf.String()
	if !strings.Contains(out, "path="+long[:maxLoggedPath]+" ") || strings.Contains(out, long[:maxLoggedPath+1]) {
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
	h := requestStats(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), logger)

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
	}), logger)

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
	}), logger)

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
	}), nil)
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
	}), nil)
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

// SetAccessLog reaches the middleware through NewRouter: a real API route
// at Info logs a request record and still sends no header.
func TestRequestStats_AccessLogWiredIntoRouter(t *testing.T) {
	withLogLevel(t, slog.LevelInfo)
	logger, buf := accessLogger()
	app := newTestAppV1(t)
	app.SetAccessLog(logger)
	router := app.NewRouter()
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
