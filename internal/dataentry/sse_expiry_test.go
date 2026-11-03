package dataentry

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// Event streams end when the verified assertion expires (TKT-5I6REX). The
// timing tests serve the real router inside a synctest bubble with an
// in-process ResponseWriter, so the fake clock decides when the expiry timer
// fires and "stays open" is checked without real waiting. The app is built
// outside the bubble: its search index starts goroutines that never exit.
// Expiries are set inside the bubble, where time.Now is the fake clock.

// tokenVerifier maps each token to the identity it verifies as, so a test
// can give two requests the same claims and different expiries.
type tokenVerifier map[string]AssertedIdentity

func (v tokenVerifier) VerifyAssertion(_ context.Context, raw string) (AssertedIdentity, error) {
	id, ok := v[raw]
	if !ok {
		return AssertedIdentity{}, errors.New("invalid")
	}
	return id, nil
}

// streamRecorder is an in-process streaming ResponseWriter. Reads happen
// after synctest.Wait, when the handler is blocked, but the mutex keeps the
// race detector satisfied either way.
type streamRecorder struct {
	mu     sync.Mutex
	header http.Header
	buf    bytes.Buffer
}

func newStreamRecorder() *streamRecorder { return &streamRecorder{header: http.Header{}} }

func (s *streamRecorder) Header() http.Header { return s.header }
func (s *streamRecorder) WriteHeader(int)     {}
func (s *streamRecorder) Flush()              {}

func (s *streamRecorder) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *streamRecorder) body() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// liveConnCount is how many streams the live hub has registered.
func (h *liveHub) liveConnCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.conns)
}

// expiryApp builds an app with a long ping interval relative to the test
// and, when verifier is non-nil, the JWT gate installed.
func expiryApp(t *testing.T, verifier assertionVerifier) (*App, http.Handler) {
	t.Helper()
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	app.sseTiming = sseTimings{flush: 20 * time.Millisecond, ping: 10 * time.Second}
	if verifier != nil {
		if err := app.SetJWTGate(JWTGateConfig{Verifier: verifier, HeaderName: assertionHeader}); err != nil {
			t.Fatalf("SetJWTGate: %v", err)
		}
	}
	return app, app.NewRouter()
}

// serveStream starts the events stream at path on h in a goroutine. done
// closes when the handler returns; cancel ends the request.
func serveStream(h http.Handler, path, token string) (rec *streamRecorder, done <-chan struct{}, cancel func()) {
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, path, http.NoBody)
	if token != "" {
		req.Header.Set(assertionHeader, token)
	}
	rec = newStreamRecorder()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		h.ServeHTTP(rec, req)
	}()
	return rec, finished, cancel
}

func returned(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

// AC1: a stream opened through the JWT gate ends at the assertion's expiry,
// and its live connection is unregistered. Both routes share the handler;
// the legacy one has no live connection.
func TestSSEExpiry_StreamEndsAtExpiry(t *testing.T) {
	for _, tc := range []struct {
		path     string
		wantLive int
	}{
		{"/api/v1/_events", 1},
		{"/api/events", 0},
	} {
		t.Run(tc.path, func(t *testing.T) {
			verifier := tokenVerifier{}
			app, h := expiryApp(t, verifier)
			synctest.Test(t, func(t *testing.T) {
				const lifetime = 90 * time.Second
				verifier["tok"] = AssertedIdentity{Subject: "usr_alice", ExpiresAt: time.Now().Add(lifetime)}

				rec, done, cancel := serveStream(h, tc.path, "tok")
				defer cancel()
				synctest.Wait()
				if !strings.HasPrefix(rec.body(), "event: ready\n") {
					t.Fatalf("body = %q, want a ready frame first", rec.body())
				}
				if n := app.live.liveConnCount(); n != tc.wantLive {
					t.Fatalf("live connections = %d, want %d", n, tc.wantLive)
				}

				time.Sleep(lifetime - time.Second)
				synctest.Wait()
				if returned(done) {
					t.Fatal("stream ended before the expiry")
				}

				time.Sleep(time.Second)
				synctest.Wait()
				if !returned(done) {
					t.Fatal("stream still open at the expiry")
				}
				if n := app.live.liveConnCount(); n != 0 {
					t.Errorf("live connections after expiry = %d, want 0", n)
				}
			})
		})
	}
}

// AC2: a stream with no known expiry stays open past the same interval
// while pings keep arriving. That holds without the gate, and through the
// gate when the verifier reports a zero expiry.
func TestSSEExpiry_NoExpiryStaysOpen(t *testing.T) {
	for _, tc := range []struct {
		name     string
		verifier assertionVerifier
		token    string
	}{
		{"no gate", nil, ""},
		{"gate with zero expiry", tokenVerifier{"tok": {Subject: "usr_alice"}}, "tok"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, h := expiryApp(t, tc.verifier)
			synctest.Test(t, func(t *testing.T) {
				rec, done, cancel := serveStream(h, "/api/v1/_events", tc.token)
				synctest.Wait()
				time.Sleep(2 * 90 * time.Second)
				synctest.Wait()
				if returned(done) {
					t.Fatal("stream without an expiry ended on its own")
				}
				if n := strings.Count(rec.body(), "event: ping\n"); n < 10 {
					t.Errorf("pings = %d, want at least 10", n)
				}

				cancel()
				<-done
			})
		})
	}
}

// AC3: an expiry that has already passed ends the stream before anything is
// registered or written.
func TestSSEExpiry_PastExpiryWritesNothing(t *testing.T) {
	verifier := tokenVerifier{}
	app, h := expiryApp(t, verifier)
	synctest.Test(t, func(t *testing.T) {
		verifier["tok"] = AssertedIdentity{Subject: "usr_alice", ExpiresAt: time.Now().Add(-time.Second)}

		rec, done, cancel := serveStream(h, "/api/v1/_events", "tok")
		defer cancel()
		synctest.Wait()
		if !returned(done) {
			t.Fatal("stream with a past expiry is still open")
		}
		if body := rec.body(); body != "" {
			t.Errorf("body = %q, want no frames", body)
		}
		if n := app.live.liveConnCount(); n != 0 {
			t.Errorf("live connections = %d, want 0", n)
		}
	})
}

// A write deadline never reaches past the expiry, so a blocked write fails
// when the credential does.
func TestSSEConn_WriteDeadlineCappedAtExpiry(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name   string
		expiry time.Time
		capped bool
	}{
		{"no expiry", time.Time{}, false},
		{"expiry after the deadline", now.Add(time.Hour), false},
		{"expiry before the deadline", now.Add(time.Second), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got time.Time
			c := &sseConn{
				w: &bytes.Buffer{}, flusher: newStreamRecorder(), timing: sseTimings{ping: time.Minute},
				expiry:      tc.expiry,
				setDeadline: func(d time.Time) error { got = d; return nil },
			}
			if err := c.frame("ping", "", "{}"); err != nil {
				t.Fatal(err)
			}
			if tc.capped != got.Equal(tc.expiry) {
				t.Errorf("deadline = %v, expiry = %v, want capped = %v", got, tc.expiry, tc.capped)
			}
		})
	}
}

// AC5: the expiry is not part of the identity. A live-view PUT whose token
// has a later expiry but the same claims is accepted, while changed claims
// still read as stale.
func TestSSEExpiry_RefreshedTokenKeepsStreamOwner(t *testing.T) {
	claims := AssertedIdentity{Subject: "usr_alice", OrgID: "org_acme", Roles: []string{"admin"}}
	stream, refreshed, other := claims, claims, claims
	stream.ExpiresAt = time.Now().Add(time.Hour)
	refreshed.ExpiresAt = time.Now().Add(2 * time.Hour)
	other.Roles = []string{"viewer"}
	other.ExpiresAt = refreshed.ExpiresAt

	app, h := expiryApp(t, tokenVerifier{"tok-stream": stream, "tok-refreshed": refreshed, "tok-other": other})
	app.startStoreEventBridge()
	t.Cleanup(app.StopWatching)
	env := &liveEnv{app: app, srv: httptest.NewServer(h)}
	t.Cleanup(env.srv.Close)

	c := openStream(t, env.srv, "/api/v1/_events", http.Header{assertionHeader: {"tok-stream"}})
	f := c.next(t)
	if f.event != "ready" {
		t.Fatalf("first frame = %+v, want ready", f)
	}
	conn := frameData(t, f)["conn"]
	sub := liveSubEntry{"k", "/api/v1/tickets"}

	if code, body := env.put(t, http.Header{assertionHeader: {"tok-other"}}, conn, sub); code != http.StatusConflict {
		t.Fatalf("PUT with changed claims = %d %s, want 409", code, body)
	}
	if code, body := env.put(t, http.Header{assertionHeader: {"tok-refreshed"}}, conn, sub); code != http.StatusNoContent {
		t.Fatalf("PUT with a refreshed token = %d %s, want 204", code, body)
	}
}
