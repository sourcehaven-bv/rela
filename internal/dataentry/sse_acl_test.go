package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/changefeed"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// sseSink is a thread-safe io.Writer + http.Flusher that records the
// SSE wire bytes written by runSSELoop, so tests can assert exactly what
// reached a connection.
type sseSink struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (s *sseSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *sseSink) Flush() {}

func (s *sseSink) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// testEvent is what a test feeds a connection: a broker frame (Name set),
// an entity change (EntityType set) or a relation change.
type testEvent struct {
	Name           string
	Data           string
	EntityType     string
	RelationChange bool
}

// chanSource is a store change watcher whose events a test sends by hand.
type chanSource struct{ ch chan store.Event }

func (c chanSource) Subscribe(int) (events <-chan store.Event, cancel func()) { return c.ch, func() {} }

// startTestFeed returns a started change feed and the channel that feeds it.
func startTestFeed(t *testing.T) (feed *changefeed.Bus, events chan<- store.Event) {
	t.Helper()
	feed, err := changefeed.New()
	if err != nil {
		t.Fatal(err)
	}
	src := chanSource{ch: make(chan store.Event, 64)}
	if err := feed.Start(src); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(feed.Close)
	return feed, src.ch
}

// runSSEConn drives runSSELoop on a goroutine with the given gated ctx.
// Broker frames go to the loop's broker channel; entity and relation
// changes go through a real change feed, as store events do in
// production. The caller sends events, waits past the flush window, then
// reads the sink. stop ends the loop.
func runSSEConn(ctx context.Context, t *testing.T, app *App) (send chan<- testEvent, sink *sseSink, stop func()) {
	t.Helper()
	feed, storeEvents := startTestFeed(t)
	sub := feed.Subscribe("")
	broker := make(chan sseEvent, 64)
	in := make(chan testEvent, 64)
	sink = &sseSink{}
	loopCtx, cancel := context.WithCancel(ctx)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_events", http.NoBody).WithContext(loopCtx)
	conn := &sseConn{w: sink, flusher: sink, timing: defaultSSETimings}
	done := make(chan struct{})
	go func() {
		app.runSSELoop(conn, req, broker, sub, sub.Start(), nil)
		close(done)
	}()
	go func() {
		for ev := range in {
			switch {
			case ev.Name != "":
				broker <- sseEvent{Name: ev.Name, Data: ev.Data}
			case ev.RelationChange:
				storeEvents <- store.Event{Op: store.EventRelationCreated, RelationType: "member-of", From: "TKT-1", To: "G"}
			default:
				storeEvents <- store.Event{Op: store.EventEntityUpdated, EntityType: ev.EntityType, EntityID: "TKT-1"}
			}
		}
	}()
	stop = func() {
		cancel()
		<-done
		close(in)
		sub.Close()
	}
	return in, sink, stop
}

// settle waits for one flush window plus slack so coalesced frames land.
func settle() { time.Sleep(sseFlushInterval + 80*time.Millisecond) }

// sseData returns the data payloads in an SSE transcript, one per frame.
func sseData(out string) []string {
	var data []string
	for line := range strings.SplitSeq(out, "\n") {
		if rest, ok := strings.CutPrefix(line, "data: "); ok {
			data = append(data, rest)
		}
	}
	return data
}

// TestSSEACL_PerTypeGating pins AC1: a principal with read:[ticket]
// receives a ticket change nudge but NOT a feature change. AC3: no
// entity id ever appears on the wire.
func TestSSEACL_PerTypeGating(t *testing.T) {
	app := newTestAppV1(t)
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)

	send, sink, stop := runSSEConn(gateCtxFor(aliceCtx(), t, d), t, app)
	defer stop()

	send <- testEvent{EntityType: "ticket"}
	send <- testEvent{EntityType: "feature"}
	settle()

	out := sink.String()
	if !strings.Contains(out, `event: entity:changed`) || !strings.Contains(out, `"type":"ticket"`) {
		t.Errorf("expected a ticket change frame, got:\n%s", out)
	}
	if strings.Contains(out, `"type":"feature"`) {
		t.Errorf("feature change leaked to a ticket-only principal:\n%s", out)
	}
}

// TestSSEACL_RoleRelationInheritance pins AC2: a Query-verdict principal
// (relation-scoped read) still receives the type nudge (verdict != DenyAll),
// but not a type they can't read at all.
func TestSSEACL_RoleRelationInheritance(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
	seedEntity(app, &entity.Entity{ID: "PRJ-42", Type: "project", Properties: map[string]any{"title": "Granted"}})
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T"}})
	seedRelation(app, entity.NewRelation("alice", "editor-of", "PRJ-42"))
	seedRelation(app, entity.NewRelation("TKT-001", "belongs-to", "PRJ-42"))

	d := mustNewACL(t, &acl.Policy{
		Roles:               map[string]acl.RoleDef{"editor": {Read: []string{"ticket"}}},
		RoleRelations:       map[string]acl.RoleRelationDef{"editor-of": {Confers: "editor"}},
		InheritRolesThrough: []string{"belongs-to"},
	}, app.store)
	setTestACL(t, app, d)

	send, sink, stop := runSSEConn(gateCtxFor(aliceCtx(), t, d), t, app)
	defer stop()

	send <- testEvent{EntityType: "ticket"}  // Query verdict → delivered
	send <- testEvent{EntityType: "feature"} // no grant → withheld
	settle()

	out := sink.String()
	if !strings.Contains(out, `"type":"ticket"`) {
		t.Errorf("Query-verdict principal should receive ticket nudges:\n%s", out)
	}
	if strings.Contains(out, `"type":"feature"`) {
		t.Errorf("feature change leaked to a principal with no feature grant:\n%s", out)
	}
}

// TestSSEACL_DenyAllWithholds pins AC4: a principal with no read grant
// on a type receives zero nudges for it (no timing signal).
func TestSSEACL_DenyAllWithholds(t *testing.T) {
	app := newTestAppV1(t)
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)

	// bob has no assignment → DenyAll on everything.
	send, sink, stop := runSSEConn(gateCtxFor(principalCtx("bob"), t, d), t, app)
	defer stop()

	send <- testEvent{EntityType: "ticket"}
	send <- testEvent{EntityType: "feature"}
	settle()

	if out := sink.String(); strings.Contains(out, "entity:changed") {
		t.Errorf("denied principal received a change nudge (timing leak):\n%s", out)
	}
}

// TestSSEACL_NoIDOnWire pins AC3 explicitly: even seeding an entity with
// a meaningful id, the wire frame carries only the type.
func TestSSEACL_NoIDOnWire(t *testing.T) {
	app := newTestAppV1(t)
	send, sink, stop := runSSEConn(aliceCtx(), t, app) // NopACL ctx → all visible
	defer stop()

	send <- testEvent{EntityType: "ticket"}
	settle()

	out := sink.String()
	if !strings.Contains(out, `"type":"ticket"`) {
		t.Fatalf("expected a ticket change frame:\n%s", out)
	}
	// The `id:` field is the opaque resume token, not an entity id; the
	// payloads must carry no id at all.
	for _, data := range sseData(out) {
		for _, leak := range []string{"TKT-", "id", `"id"`} {
			if strings.Contains(data, leak) {
				t.Errorf("entity id token %q leaked onto the SSE wire:\n%s", leak, out)
			}
		}
	}
}

// TestSSEACL_Debounce pins AC5: a burst of N same-type events within a
// window produces exactly one frame.
func TestSSEACL_Debounce(t *testing.T) {
	app := newTestAppV1(t)
	send, sink, stop := runSSEConn(aliceCtx(), t, app)
	defer stop()

	for range 20 {
		send <- testEvent{EntityType: "ticket"}
	}
	settle()

	if n := strings.Count(sink.String(), "event: entity:changed"); n != 1 {
		t.Errorf("burst of 20 ticket events produced %d frames, want 1 (debounce)", n)
	}
}

// TestSSEACL_DebounceMultiType pins that distinct types in a burst each
// get exactly one frame.
func TestSSEACL_DebounceMultiType(t *testing.T) {
	app := newTestAppV1(t)
	send, sink, stop := runSSEConn(aliceCtx(), t, app)
	defer stop()

	for range 5 {
		send <- testEvent{EntityType: "ticket"}
		send <- testEvent{EntityType: "feature"}
	}
	settle()

	out := sink.String()
	if got := strings.Count(out, `"type":"ticket"`); got != 1 {
		t.Errorf("ticket frames = %d, want 1", got)
	}
	if got := strings.Count(out, `"type":"feature"`); got != 1 {
		t.Errorf("feature frames = %d, want 1", got)
	}
}

// TestSSEACL_TwoPrincipalsDifferentFrames pins the RR-GVHEIK invariant:
// two simultaneous connections with different principals get different
// frames for the SAME store event — the gate is genuinely per-connection.
func TestSSEACL_TwoPrincipalsDifferentFrames(t *testing.T) {
	app := newTestAppV1(t)
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"t-viewer": {Read: []string{"ticket"}},
			"f-viewer": {Read: []string{"feature"}},
		},
		Assignments: map[string]string{"alice": "t-viewer", "bob": "f-viewer"},
	}, app.store)
	setTestACL(t, app, d)

	aSend, aSink, aStop := runSSEConn(gateCtxFor(principalCtx("alice"), t, d), t, app)
	defer aStop()
	bSend, bSink, bStop := runSSEConn(gateCtxFor(principalCtx("bob"), t, d), t, app)
	defer bStop()

	// The same store event reaches both connections.
	for _, s := range []chan<- testEvent{aSend, bSend} {
		s <- testEvent{EntityType: "ticket"}
		s <- testEvent{EntityType: "feature"}
	}
	settle()

	aOut, bOut := aSink.String(), bSink.String()
	if !strings.Contains(aOut, `"type":"ticket"`) || strings.Contains(aOut, `"type":"feature"`) {
		t.Errorf("alice (ticket-only) frames wrong:\n%s", aOut)
	}
	if !strings.Contains(bOut, `"type":"feature"`) || strings.Contains(bOut, `"type":"ticket"`) {
		t.Errorf("bob (feature-only) frames wrong:\n%s", bOut)
	}
}

// recordingReadGate counts ReadQuery calls and delegates verdicts, so a
// test can assert the per-connection cache (one resolve per type, not
// per event) and drive the membership-invalidation path.
type recordingReadGate struct {
	readGate
	mu       sync.Mutex
	calls    map[string]int
	verdicts map[string]acl.ReadQueryResult
}

func (g *recordingReadGate) ReadQuery(_ context.Context, entityType string) acl.ReadQueryResult {
	g.mu.Lock()
	g.calls[entityType]++
	v, ok := g.verdicts[entityType]
	g.mu.Unlock()
	if ok {
		return v
	}
	return acl.ReadQueryResult{AllowAll: true}
}

func (g *recordingReadGate) callCount(typ string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls[typ]
}

// TestSSEACL_VerdictCached pins the caching half of AC6: the type
// verdict is resolved once per connection and cached (no per-event
// walk), and a RelationChange clears the cache so the next event
// re-resolves. Uses a counting mock to observe resolve calls; the
// genuine membership-flip is pinned separately against a real resolver
// (TestSSEACL_MembershipChangeReGates).
func TestSSEACL_VerdictCached(t *testing.T) {
	app := newTestAppV1(t)
	rg := &recordingReadGate{
		calls:    map[string]int{},
		verdicts: map[string]acl.ReadQueryResult{"ticket": {AllowAll: true}},
	}
	ctx := withReadGate(aliceCtx(), rg)

	send, _, stop := runSSEConn(ctx, t, app)
	defer stop()

	// Three ticket events → the verdict is resolved ONCE (cached).
	for range 3 {
		send <- testEvent{EntityType: "ticket"}
	}
	settle()
	if c := rg.callCount("ticket"); c != 1 {
		t.Errorf("ReadQuery(ticket) called %d times across 3 events, want 1 (cached)", c)
	}

	// A relation change clears the cache; the next ticket event re-resolves.
	// (app.acl is nil here → freshReadGate keeps the mock, so the same gate
	// is re-queried — this pins the cache-clear, not the membership walk.)
	send <- testEvent{RelationChange: true}
	send <- testEvent{EntityType: "ticket"}
	settle()
	if c := rg.callCount("ticket"); c != 2 {
		t.Errorf("ReadQuery(ticket) called %d times after invalidation, want 2 (re-resolved)", c)
	}
}

// TestSSEACL_MembershipChangeReGates pins the genuine-membership half of
// AC6 against a REAL acl.Declarative (RR — code review): a principal who
// GAINS read access mid-connection (via a new group membership / role
// edge) starts receiving the type's nudges after the relation-change
// re-derives the gate, WITHOUT reconnecting. This is the case the mocked
// test could not show — it exercises acl.Request.Globals being re-walked
// against the current graph rather than the connection-start snapshot.
func TestSSEACL_MembershipChangeReGates(t *testing.T) {
	app := newTestAppV1(t)
	// alice is initially NOT in the group that confers ticket read.
	seedEntity(app, &entity.Entity{ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"}})
	seedEntity(app, &entity.Entity{ID: "editors", Type: "person", Properties: map[string]any{"title": "Editors"}})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"editors": "viewer"},
	}, app.store)
	setTestACL(t, app, d)

	send, sink, stop := runSSEConn(gateCtxFor(aliceCtx(), t, d), t, app)
	defer stop()

	// Before membership: alice has DenyAll on ticket → no nudge.
	send <- testEvent{EntityType: "ticket"}
	settle()
	if strings.Contains(sink.String(), "entity:changed") {
		t.Fatalf("alice received a ticket nudge before gaining membership:\n%s", sink.String())
	}

	// Grant: alice joins the editors group (member-of edge). This is a
	// role-conferring relation write → RelationChange → re-derive gate.
	seedRelation(app, entity.NewRelation("alice", "member-of", "editors"))
	send <- testEvent{RelationChange: true}
	send <- testEvent{EntityType: "ticket"}
	settle()
	if !strings.Contains(sink.String(), `"type":"ticket"`) {
		t.Errorf("alice should receive ticket nudges after joining the conferring group (re-gate), got:\n%s", sink.String())
	}
}

// errReadGate returns an error-free ReadQuery but flips DenyAll/Query so
// we can also exercise the deny path; for the error path we use a gate
// whose ReadQuery returns a zero verdict (neither AllowAll nor Query),
// which entityTypeVisible treats as fail-closed.
type zeroVerdictGate struct{ readGate }

func (zeroVerdictGate) ReadQuery(context.Context, string) acl.ReadQueryResult {
	return acl.ReadQueryResult{} // neither AllowAll nor Query → withhold (fail-closed)
}

// TestSSEACL_FailClosedOnZeroVerdict pins AC7's fail-closed posture: an
// unresolvable / zero verdict withholds the nudge rather than leaking it.
func TestSSEACL_FailClosedOnZeroVerdict(t *testing.T) {
	app := newTestAppV1(t)
	ctx := withReadGate(aliceCtx(), zeroVerdictGate{})

	send, sink, stop := runSSEConn(ctx, t, app)
	defer stop()

	send <- testEvent{EntityType: "ticket"}
	settle()

	if out := sink.String(); strings.Contains(out, "entity:changed") {
		t.Errorf("zero/unresolvable verdict must fail closed (withhold), got:\n%s", out)
	}
}

// TestSSEACL_NonEntityEventsPassThrough pins that refresh / git frames
// are delivered ungated and immediately (no gate, no debounce).
func TestSSEACL_NonEntityEventsPassThrough(t *testing.T) {
	app := newTestAppV1(t)
	// A DenyAll-everything principal still gets non-entity frames.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)

	send, sink, stop := runSSEConn(gateCtxFor(principalCtx("bob"), t, d), t, app)
	defer stop()

	send <- testEvent{Name: "refresh", Data: "refresh"}
	send <- testEvent{Name: "git:status", Data: "{}"}
	// give the immediate (non-debounced) writes a moment
	time.Sleep(50 * time.Millisecond)

	out := sink.String()
	if !strings.Contains(out, "event: refresh") {
		t.Errorf("refresh frame not delivered to a denied principal:\n%s", out)
	}
	if !strings.Contains(out, "event: git:status") {
		t.Errorf("git:status frame not delivered:\n%s", out)
	}
}

// TestSSEACL_NopACLAllTypesFlow pins AC8: without ACL, every type nudge
// flows (nop gate → AllowAll), id-less.
func TestSSEACL_NopACLAllTypesFlow(t *testing.T) {
	app := newTestAppV1(t) // no app.acl set → nopReadGate via ctx fallback
	send, sink, stop := runSSEConn(context.Background(), t, app)
	defer stop()

	send <- testEvent{EntityType: "ticket"}
	send <- testEvent{EntityType: "feature"}
	settle()

	out := sink.String()
	if !strings.Contains(out, `"type":"ticket"`) || !strings.Contains(out, `"type":"feature"`) {
		t.Errorf("NopACL should deliver every type nudge, got:\n%s", out)
	}
}
