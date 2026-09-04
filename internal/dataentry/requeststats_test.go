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
	h := requestStats(countingHandler(t, true))

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
	h := requestStats(countingHandler(t, false))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody))

	if got := rec.Header().Get("Server-Timing"); got != "" {
		t.Errorf("Server-Timing must be absent below Debug, got %q", got)
	}
	if buf.Len() != 0 {
		t.Errorf("no request record below Debug, got %q", buf.String())
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
	}))
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
	}))
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
