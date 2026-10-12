package dataentry

import (
	"bufio"
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// These tests drive the real handleSSE over a real HTTP connection, with
// changes coming from real store writes through the change feed
// (TKT-3A1200). The numbers in test names refer to the ticket's acceptance
// criteria.

// sseFrame is one parsed server-sent event.
type sseFrame struct {
	event, id, data string
}

// sseClient is one open SSE connection.
type sseClient struct {
	frames <-chan sseFrame
	close  func()
}

// resumeApp returns a test app with its change feed started on its store and
// short stream intervals.
func resumeApp(t *testing.T, timing sseTimings) *App {
	t.Helper()
	app := newTestAppV1(t)
	app.broker = newEventBroker()
	app.sseTiming = timing
	app.startStoreEventBridge()
	t.Cleanup(app.StopWatching)
	return app
}

// sseServer serves app.handleSSE. withCtx adds the principal and read gate
// that the production middleware would; stashReplayPrincipal saves the
// principal for live views, as in the router.
func sseServer(t *testing.T, app *App, withCtx func(context.Context) context.Context) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(stashReplayPrincipal(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.handleSSE(w, r.WithContext(withCtx(r.Context())))
	})))
	t.Cleanup(srv.Close)
	return srv
}

// ungated leaves the request context as is: no ACL, every type readable.
func ungated(ctx context.Context) context.Context { return ctx }

// connect opens a stream. lastEventID goes in the Last-Event-ID header and
// query in the URL; either may be empty.
func connect(t *testing.T, srv *httptest.Server, lastEventID, query string) *sseClient {
	t.Helper()
	url := "/api/v1/_events"
	if query != "" {
		url += "?last_event_id=" + query
	}
	hdr := http.Header{}
	if lastEventID != "" {
		hdr.Set("Last-Event-ID", lastEventID)
	}
	return openStream(t, srv, url, hdr)
}

// openStream opens the stream at path on srv with the given request headers.
func openStream(t *testing.T, srv *httptest.Server, path string, hdr http.Header) *sseClient {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+path, http.NoBody)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	maps.Copy(req.Header, hdr)
	resp, err := srv.Client().Do(req) //nolint:bodyclose // the reader goroutine and c.close close it
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	frames := make(chan sseFrame, 64)
	go func() {
		defer close(frames)
		defer resp.Body.Close()
		sc := bufio.NewScanner(resp.Body)
		var f sseFrame
		for sc.Scan() {
			line := sc.Text()
			switch {
			case line == "":
				if f.event != "" {
					frames <- f
				}
				f = sseFrame{}
			case strings.HasPrefix(line, "event: "):
				f.event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "id: "):
				f.id = strings.TrimPrefix(line, "id: ")
			case strings.HasPrefix(line, "data: "):
				f.data = strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	c := &sseClient{frames: frames, close: func() { cancel(); _ = resp.Body.Close() }}
	t.Cleanup(c.close)
	return c
}

// liveConnIDLen is the length of a connection id: 16 random bytes in
// unpadded base64url.
const liveConnIDLen = 22

// frameData decodes a frame's JSON object data.
func frameData(t *testing.T, f sseFrame) map[string]string {
	t.Helper()
	var d map[string]string
	if err := json.Unmarshal([]byte(f.data), &d); err != nil {
		t.Fatalf("frame %+v: data is not a JSON object: %v", f, err)
	}
	return d
}

// next returns the next frame other than ping, failing after a second.
func (c *sseClient) next(t *testing.T) sseFrame {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case f, ok := <-c.frames:
			if !ok {
				t.Fatal("stream ended")
			}
			if f.event != "ping" {
				return f
			}
		case <-deadline:
			t.Fatal("no frame within timeout")
		}
	}
}

// quiet asserts that no frame other than ping arrives within d.
func (c *sseClient) quiet(t *testing.T, d time.Duration) {
	t.Helper()
	deadline := time.After(d)
	for {
		select {
		case f, ok := <-c.frames:
			if ok && f.event != "ping" {
				t.Fatalf("unexpected frame %+v", f)
			}
			if !ok {
				return
			}
		case <-deadline:
			return
		}
	}
}

// write creates entities and waits until the feed has counted them.
func write(t *testing.T, app *App, ents ...*entity.Entity) {
	t.Helper()
	probe := app.feed.Subscribe("")
	defer probe.Close()
	want := probe.Start() + uint64(len(ents))
	for _, e := range ents {
		if err := app.store.CreateEntity(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-probe.Wake():
			if probe.Take().Seq >= want {
				return
			}
		case <-deadline:
			t.Fatal("feed did not count the writes")
		}
	}
}

var fastSSE = sseTimings{flush: 20 * time.Millisecond, ping: time.Hour}

func TestSSEResume_ReadyFrameCarriesToken(t *testing.T) { // AC1
	app := resumeApp(t, fastSSE)
	c := connect(t, sseServer(t, app, ungated), "", "")
	f := c.next(t)
	if f.event != "ready" || f.id == "" || len(frameData(t, f)["conn"]) != liveConnIDLen {
		t.Fatalf("first frame = %+v, want ready with an id and a connection id", f)
	}
	write(t, app, entity.New("TKT-90", "ticket"))
	f = c.next(t)
	if f.event != "entity:changed" || f.id == "" || f.data != `{"type":"ticket"}` {
		t.Fatalf("change frame = %+v, want entity:changed for ticket with an id", f)
	}
}

// A stream that did not pass stashReplayPrincipal gets no live views rather
// than replays run as the already resolved principal; the stream still works.
func TestSSE_NoStashedPrincipalNoLiveViews(t *testing.T) {
	app := resumeApp(t, fastSSE)
	srv := httptest.NewServer(http.HandlerFunc(app.handleSSE))
	t.Cleanup(srv.Close)
	c := connect(t, srv, "", "")
	if f := c.next(t); f.event != "ready" || f.data != "{}" {
		t.Fatalf("first frame = %+v, want ready without a connection id", f)
	}
	write(t, app, entity.New("TKT-93", "ticket"))
	if f := c.next(t); f.event != "entity:changed" {
		t.Fatalf("change frame = %+v, want entity:changed", f)
	}
}

func TestSSEResume_ReplaysMissedTypes(t *testing.T) { // AC2
	app := resumeApp(t, fastSSE)
	srv := sseServer(t, app, ungated)
	c := connect(t, srv, "", "")
	token := c.next(t).id
	c.close()

	write(t, app, entity.New("TKT-91", "ticket"), entity.New("FEAT-91", "feature"), entity.New("TKT-92", "ticket"))

	for _, via := range []string{"header", "query"} {
		t.Run(via, func(t *testing.T) {
			var r *sseClient
			if via == "header" {
				r = connect(t, srv, token, "")
			} else {
				r = connect(t, srv, "", token)
			}
			if f := r.next(t); f.event != "ready" {
				t.Fatalf("first frame = %+v, want ready (no resync)", f)
			}
			a, b := r.next(t), r.next(t)
			if a.data != `{"type":"feature"}` || b.data != `{"type":"ticket"}` {
				t.Fatalf("replayed %q, %q; want feature then ticket", a.data, b.data)
			}
			if a.id != "" || b.id == "" {
				t.Fatalf("ids = %q, %q; want the token on the last frame only", a.id, b.id)
			}
			r.quiet(t, 100*time.Millisecond)
		})
	}
}

func TestSSEResume_ReadyTokenDoesNotSkipReplay(t *testing.T) { // AC3
	app := resumeApp(t, sseTimings{flush: 300 * time.Millisecond, ping: time.Hour})
	srv := sseServer(t, app, ungated)
	c := connect(t, srv, "", "")
	token := c.next(t).id
	c.close()
	write(t, app, entity.New("TKT-93", "ticket"))

	// Resume, then drop before the flush delivers the replay.
	r := connect(t, srv, token, "")
	ready := r.next(t)
	r.close()

	again := connect(t, srv, ready.id, "")
	if f := again.next(t); f.event != "ready" {
		t.Fatalf("first frame = %+v, want ready", f)
	}
	if f := again.next(t); f.data != `{"type":"ticket"}` {
		t.Fatalf("replay from the ready token = %+v, want the ticket change", f)
	}
}

func TestSSEResume_UnusableTokenResyncs(t *testing.T) { // AC4
	app := resumeApp(t, fastSSE)
	srv := sseServer(t, app, ungated)
	other := resumeApp(t, fastSSE)
	foreign := connect(t, sseServer(t, other, ungated), "", "").next(t).id

	for name, token := range map[string]string{
		"garbage":       "garbage",
		"oversize":      strings.Repeat("A", 4096),
		"other process": foreign,
	} {
		t.Run(name, func(t *testing.T) {
			c := connect(t, srv, token, "")
			f := c.next(t)
			if d := frameData(t, f); f.event != "refresh" || d["reason"] != "resync" || d["conn"] == "" || f.id == "" {
				t.Fatalf("first frame = %+v, want refresh resync with an id", f)
			}
			write(t, app, entity.New("TKT-R"+strings.ReplaceAll(name, " ", "-"), "ticket"))
			if f := c.next(t); f.event != "entity:changed" {
				t.Fatalf("after resync got %+v, want live entity:changed", f)
			}
		})
	}
}

func TestSSEResume_HeaderWinsOverQuery(t *testing.T) {
	app := resumeApp(t, fastSSE)
	srv := sseServer(t, app, ungated)
	token := connect(t, srv, "", "").next(t).id
	c := connect(t, srv, token, "garbage")
	if f := c.next(t); f.event != "ready" {
		t.Fatalf("first frame = %+v, want ready (header honored, query ignored)", f)
	}
}

func TestSSEResume_HiddenTypesWithheld(t *testing.T) { // AC5
	app := resumeApp(t, fastSSE)
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	srv := sseServer(t, app, func(ctx context.Context) context.Context {
		// Keep the request context: its cancellation is how the handler
		// learns the client went away.
		return gateCtxFor(principal.With(ctx, principal.Principal{User: "alice", Tool: principal.ToolDataEntry}), t, d)
	})

	c := connect(t, srv, "", "")
	token := c.next(t).id

	// Live: a hidden-only write produces no frame at all.
	write(t, app, entity.New("FEAT-95", "feature"))
	c.quiet(t, 150*time.Millisecond)
	c.close()

	// Resumed: the hidden type is withheld, the readable one delivered.
	write(t, app, entity.New("TKT-95", "ticket"), entity.New("FEAT-96", "feature"))
	r := connect(t, srv, token, "")
	if f := r.next(t); f.event != "ready" {
		t.Fatalf("first frame = %+v, want ready", f)
	}
	if f := r.next(t); f.data != `{"type":"ticket"}` {
		t.Fatalf("resumed frame = %+v, want ticket only", f)
	}
	r.quiet(t, 150*time.Millisecond)
}

func TestSSEResume_PingCarriesFlushedToken(t *testing.T) {
	app := resumeApp(t, sseTimings{flush: 20 * time.Millisecond, ping: 100 * time.Millisecond})
	srv := sseServer(t, app, ungated)
	c := connect(t, srv, "", "")
	ready := c.next(t)
	write(t, app, entity.New("TKT-96", "ticket"))
	c.next(t) // the ticket change

	var ping sseFrame
	deadline := time.After(time.Second)
	for ping.event != "ping" {
		select {
		case ping = <-c.frames:
		case <-deadline:
			t.Fatal("no ping within a second")
		}
	}
	if ping.id == "" || ping.data != "{}" {
		t.Fatalf("ping = %+v, want data {} and an id", ping)
	}
	c.close()

	// The ping token is past the ticket change; the ready token is before it.
	p := connect(t, srv, ping.id, "")
	if f := p.next(t); f.event != "ready" {
		t.Fatalf("first frame = %+v, want ready", f)
	}
	p.quiet(t, 150*time.Millisecond)
	r := connect(t, srv, ready.id, "")
	r.next(t) // ready
	if f := r.next(t); f.data != `{"type":"ticket"}` {
		t.Fatalf("replay from the ready token = %+v, want ticket", f)
	}
}

func TestSSEResume_RetireEndsWithConfigChanged(t *testing.T) { // AC6
	app := resumeApp(t, fastSSE)
	c := connect(t, sseServer(t, app, ungated), "", "")
	c.next(t) // ready

	Retire(app)

	var last sseFrame
	deadline := time.After(2 * time.Second)
	for {
		select {
		case f, ok := <-c.frames:
			if !ok {
				if last.event != sseConfigChanged {
					t.Fatalf("last frame = %+v, want %s", last, sseConfigChanged)
				}
				return
			}
			last = f
		case <-deadline:
			t.Fatal("stream did not end after Retire")
		}
	}
}

// TestSSEResume_ConfigChangedEndsStream pins that config-changed is the last
// frame even while a flush is due: the stream ends on the frame itself, not
// on the broker channel closing after it.
func TestSSEResume_ConfigChangedEndsStream(t *testing.T) {
	app := resumeApp(t, sseTimings{flush: time.Millisecond, ping: time.Millisecond})
	sub := app.feed.Subscribe("")
	defer sub.Close()
	broker := make(chan sseEvent, 1)
	broker <- sseEvent{Name: sseConfigChanged, Data: sseConfigChanged}
	sink := &sseSink{}
	conn := &sseConn{w: sink, flusher: sink, timing: app.sseTiming}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_events", http.NoBody)

	done := make(chan struct{})
	go func() {
		app.runSSELoop(conn, req, broker, sub, sub.Start(), nil)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stream kept running after config-changed")
	}
	if got := sseData(sink.String()); len(got) == 0 || got[len(got)-1] != sseConfigChanged {
		t.Fatalf("frames = %q, want config-changed last", got)
	}
}
