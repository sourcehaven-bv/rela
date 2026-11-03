package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A search whose client went away is answered with nothing and logs no
// warning (TKT-T2I7P2). On atlas such cancels were a quarter of all
// /_search requests, each logged as a failure.
//
// The request's own context decides, not the error: pgx reports a statement
// canceled by a cancel request as SQLSTATE 57014, which does not wrap
// context.Canceled, and a context.Canceled from another context does not mean
// this client left.
func TestSearch_CanceledRequestStaysSilent(t *testing.T) {
	for _, tc := range []struct {
		name         string
		err          error
		cancelClient bool
	}{
		{"error wraps context.Canceled", fmt.Errorf("load headers: %w", context.Canceled), true},
		{"opaque error after the client left", errors.New("canceling statement due to user request (SQLSTATE 57014)"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			appLog := withLogLevel(t, slog.LevelInfo)
			app := newTestAppV1(t)
			app.searcher = &fakeSearcher{err: tc.err}
			rebindVisibleSearcher(t, app)

			ctx, cancel := context.WithCancel(context.Background())
			if tc.cancelClient {
				cancel()
			} else {
				defer cancel()
			}
			req := httptest.NewRequest(http.MethodGet, "/api/v1/_search?q=alpha", http.NoBody).WithContext(ctx)
			rec := httptest.NewRecorder()
			app.handleV1Search(rec, req)

			if rec.Body.Len() != 0 {
				t.Errorf("a canceled search must write nothing, got %d: %s", rec.Code, rec.Body)
			}
			if strings.Contains(appLog.String(), "level=WARN") {
				t.Errorf("a canceled search must not log a warning, got %q", appLog.String())
			}
		})
	}
}

// An error that wraps context.Canceled while this client still waits is
// answered: it came from another context, and silence would send an empty
// 200.
func TestSearch_ForeignCancelIsAnswered(t *testing.T) {
	withLogLevel(t, slog.LevelInfo)
	app := newTestAppV1(t)
	app.searcher = &fakeSearcher{err: fmt.Errorf("shared call: %w", context.Canceled)}
	rebindVisibleSearcher(t, app)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_search?q=alpha", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Search(rec, req)

	if rec.Code < 500 || rec.Body.Len() == 0 {
		t.Errorf("got %d with %d bytes, want an answered 5xx", rec.Code, rec.Body.Len())
	}
}

// A deadline is the server giving up, not the client: it is still answered
// and logged.
func TestSearch_DeadlineIsNotACancel(t *testing.T) {
	appLog := withLogLevel(t, slog.LevelInfo)
	app := newTestAppV1(t)
	app.searcher = &fakeSearcher{err: fmt.Errorf("search: %w", context.DeadlineExceeded)}
	rebindVisibleSearcher(t, app)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_search?q=alpha", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1Search(rec, req)

	if rec.Code < 500 {
		t.Errorf("status = %d, want a 5xx", rec.Code)
	}
	if !strings.Contains(appLog.String(), "level=WARN") {
		t.Errorf("a deadline must be logged, got %q", appLog.String())
	}
}
