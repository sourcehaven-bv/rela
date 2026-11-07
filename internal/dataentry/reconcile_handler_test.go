package dataentry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/reconcile"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/elementnaive"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// reconcilePolicy: root reads everything; alice reads tickets and features
// but sees only a ticket's title; bob reads only tickets.
const reconcilePolicy = `
roles:
  admin:
    read: ["*"]
  viewer:
    read: [ticket, feature]
    visible:
      ticket:
        - field: title
  narrow:
    read: [ticket]
assignments:
  root: admin
  alice: viewer
  bob: narrow
`

type noRollups struct{}

func (noRollups) Names(string) []string { return nil }

type reconcileFixture struct {
	t   *testing.T
	app *App
	d   *acl.Declarative
	h   *reconcileHandler
}

func newReconcileFixture(t *testing.T, gate reconcileGateConfig) *reconcileFixture {
	t.Helper()
	app := newTestAppV1(t)
	var policy acl.Policy
	if err := yaml.Unmarshal([]byte(reconcilePolicy), &policy); err != nil {
		t.Fatal(err)
	}
	d, err := acl.NewDeclarative(&policy, aclstore.MustNewGraph(app.store), app.store)
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := affordances.New(app.Meta(), storeRelationLookup{st: app.store}, d)
	if err != nil {
		t.Fatal(err)
	}
	reads, err := visibility.NewDeclarativeGate(d, defaultWorldScope(app.worlds))
	if err != nil {
		t.Fatal(err)
	}
	pol, err := visibility.NewPolicyRedactor(resolver)
	if err != nil {
		t.Fatal(err)
	}
	ranges, err := elementnaive.New(app.store)
	if err != nil {
		t.Fatal(err)
	}
	el, err := visibility.NewElements(visibility.ElementDeps{
		Ranges: ranges, Rows: app.store, Reads: reads, Policy: pol, Rollups: noRollups{},
	})
	if err != nil {
		t.Fatal(err)
	}
	h, err := newReconcileHandler(app.State, el, newReconcileGate(gate))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "one", "status": "open"}},
		{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "two", "status": "open"}},
		{ID: "TKT-003", Type: "ticket", Properties: map[string]any{"title": "three"}},
		{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "f1"}},
		{ID: "FEAT-002", Type: "feature", Properties: map[string]any{"title": "f2"}},
	} {
		seedEntity(app, e)
	}
	return &reconcileFixture{t: t, app: app, d: d, h: h}
}

func (f *reconcileFixture) post(user string, body any) *httptest.ResponseRecorder {
	f.t.Helper()
	raw, ok := body.(string)
	if !ok {
		b, err := json.Marshal(body)
		if err != nil {
			f.t.Fatal(err)
		}
		raw = string(b)
	}
	ctx := gateCtxFor(principalCtx(user), f.t, f.d)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, reconcilePath, strings.NewReader(raw))
	rec := httptest.NewRecorder()
	f.h.handleV1Reconcile(rec, req)
	return rec
}

func (f *reconcileFixture) answer(user string, ranges ...v1.ReconcileRange) []v1.ReconcileResult {
	f.t.Helper()
	rec := f.post(user, v1.ReconcileRequest{Ranges: ranges})
	if rec.Code != http.StatusOK {
		f.t.Fatalf("POST %s as %s: %d %s", reconcilePath, user, rec.Code, rec.Body)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		f.t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	var out v1.ReconcileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		f.t.Fatal(err)
	}
	if len(out.Ranges) != len(ranges) {
		f.t.Fatalf("got %d results for %d ranges", len(out.Ranges), len(ranges))
	}
	return out.Ranges
}

func fpRange(types ...string) v1.ReconcileRange {
	return v1.ReconcileRange{Kind: "entity", Entities: types, Op: "fingerprint"}
}

func listRange(limit int, types ...string) v1.ReconcileRange {
	return v1.ReconcileRange{Kind: "entity", Entities: types, Op: "list", Limit: limit}
}

func TestReconcile_PrincipalsSeeTheirOwnRows(t *testing.T) {
	f := newReconcileFixture(t, reconcileGateConfig{})
	both := []string{"ticket", "feature"}

	a := f.answer("alice", fpRange(both...), listRange(100, both...))
	b := f.answer("bob", fpRange(both...), listRange(100, both...))
	if *a[0].Count != 5 || *b[0].Count != 3 {
		t.Errorf("counts alice=%d bob=%d, want 5 and 3", *a[0].Count, *b[0].Count)
	}
	if a[0].Fingerprint == b[0].Fingerprint {
		t.Error("alice and bob got the same fingerprint over different rows")
	}
	for _, e := range b[1].Elements {
		if strings.HasPrefix(e.Address, "feature/") {
			t.Errorf("bob's list holds %s, which he may not read", e.Address)
		}
	}

	// The fingerprint is the fold of the listed elements.
	for _, res := range [][]v1.ReconcileResult{a, b} {
		var acc reconcile.Range
		for _, e := range res[1].Elements {
			var h canonical.Digest
			if err := h.UnmarshalText([]byte(e.Hash)); err != nil {
				t.Fatal(err)
			}
			acc = acc.Add(reconcile.Element{Kind: reconcile.KindEntity, Address: e.Address, Hash: h})
		}
		if acc.Sum.String() != res[0].Fingerprint || acc.Count != *res[0].Count {
			t.Errorf("fold of list = %s/%d, fingerprint = %s/%d",
				acc.Sum, acc.Count, res[0].Fingerprint, *res[0].Count)
		}
	}
}

func TestReconcile_HiddenFieldDoesNotReachTheHash(t *testing.T) {
	f := newReconcileFixture(t, reconcileGateConfig{})
	before := map[string]string{
		"alice": f.answer("alice", fpRange("ticket"))[0].Fingerprint,
		"root":  f.answer("root", fpRange("ticket"))[0].Fingerprint,
	}
	e, err := f.app.store.GetEntity(context.Background(), entity.Ref{ID: "TKT-001", Face: ""})
	if err != nil {
		t.Fatal(err)
	}
	e.Properties["status"] = "closed"
	if err := f.app.store.UpdateEntity(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	if got := f.answer("alice", fpRange("ticket"))[0].Fingerprint; got != before["alice"] {
		t.Error("a change to a field alice cannot see changed her fingerprint")
	}
	if got := f.answer("root", fpRange("ticket"))[0].Fingerprint; got == before["root"] {
		t.Error("a change to a visible field did not change root's fingerprint")
	}
}

func TestReconcile_Paging(t *testing.T) {
	f := newReconcileFixture(t, reconcileGateConfig{})
	var got []string
	rr := listRange(2, "ticket", "feature")
	for range 5 {
		res := f.answer("alice", rr)[0]
		for _, e := range res.Elements {
			got = append(got, e.Address)
		}
		if !res.More {
			break
		}
		last := res.Last
		rr.After = &last
	}
	if len(got) != 5 {
		t.Fatalf("paged %v, want 5 addresses", got)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Errorf("pages are not in strict address order: %v", got)
		}
	}

	// Bounds: [feature/, ticket/) holds the features only.
	hi := "ticket/"
	bounded := v1.ReconcileRange{Kind: "entity", Entities: []string{"ticket", "feature"},
		Lo: "feature/", Hi: &hi, Op: "fingerprint"}
	if c := *f.answer("alice", bounded)[0].Count; c != 2 {
		t.Errorf("count in [feature/, ticket/) = %d, want 2", c)
	}
}

// TestReconcile_PagingPastAPrefix pins the successor of `after`: an id that
// is a prefix of the next one is neither repeated nor skipped.
func TestReconcile_PagingPastAPrefix(t *testing.T) {
	f := newReconcileFixture(t, reconcileGateConfig{})
	for _, id := range []string{"TKT-1", "TKT-10", "TKT-100"} {
		seedEntity(f.app, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"title": id}})
	}
	want := f.answer("root", listRange(100, "ticket"))[0].Elements
	var got []v1.ReconcileElement
	rr := listRange(1, "ticket")
	for range 20 {
		res := f.answer("root", rr)[0]
		got = append(got, res.Elements...)
		if !res.More {
			break
		}
		last := res.Last
		rr.After = &last
	}
	if len(got) != len(want) {
		t.Fatalf("paged %d elements, one page holds %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("element %d: paged %v, listed %v", i, got[i], want[i])
		}
	}
}

func TestReconcile_RefusesBadRequests(t *testing.T) {
	f := newReconcileFixture(t, reconcileGateConfig{})
	hi := "A"
	after := "x"
	many := make([]v1.ReconcileRange, reconcileMaxRanges+1)
	for i := range many {
		many[i] = fpRange("ticket")
	}
	cases := []struct {
		name string
		body any
		code int
		want string
	}{
		{"not json", "{", http.StatusBadRequest, ""},
		{"unknown field", `{"ranges":[],"x":1}`, http.StatusBadRequest, ""},
		{"no ranges", v1.ReconcileRequest{}, http.StatusBadRequest, "must not be empty"},
		{"too many ranges", v1.ReconcileRequest{Ranges: many}, http.StatusBadRequest, "at most 32 ranges"},
		{"undeclared type", v1.ReconcileRequest{Ranges: []v1.ReconcileRange{fpRange("nope")}},
			http.StatusBadRequest, `entity type \"nope\" is not declared`},
		{"duplicate type", v1.ReconcileRequest{Ranges: []v1.ReconcileRange{fpRange("ticket", "ticket")}},
			http.StatusBadRequest, `entity type \"ticket\" is named twice`},
		{"undeclared relation", v1.ReconcileRequest{Ranges: []v1.ReconcileRange{{Kind: "relation",
			Entities: []string{"ticket"}, Relations: []string{"nope"}, Op: "fingerprint"}}},
			http.StatusBadRequest, `relation type \"nope\" is not declared`},
		{"relation range without relations", v1.ReconcileRequest{Ranges: []v1.ReconcileRange{{Kind: "relation",
			Entities: []string{"ticket"}, Op: "fingerprint"}}}, http.StatusBadRequest, "at least one relation"},
		{"relation ends out of scope", v1.ReconcileRequest{Ranges: []v1.ReconcileRange{{Kind: "relation",
			Entities: []string{"feature"}, Relations: []string{"implements"}, Op: "fingerprint"}}},
			http.StatusBadRequest, `relation type \"implements\" links [ticket] to [feature]`},
		{"entity range with relations", v1.ReconcileRequest{Ranges: []v1.ReconcileRange{{Kind: "entity",
			Entities: []string{"ticket"}, Relations: []string{"blocks"}, Op: "fingerprint"}}},
			http.StatusBadRequest, "names no relation"},
		{"no entities", v1.ReconcileRequest{Ranges: []v1.ReconcileRange{fpRange()}},
			http.StatusBadRequest, "at least one entity"},
		{"bad kind", v1.ReconcileRequest{Ranges: []v1.ReconcileRange{{Kind: "x", Entities: []string{"ticket"},
			Op: "fingerprint"}}}, http.StatusBadRequest, "kind"},
		{"bad op", v1.ReconcileRequest{Ranges: []v1.ReconcileRange{{Kind: "entity", Entities: []string{"ticket"},
			Op: "x"}}}, http.StatusBadRequest, "op"},
		{"fingerprint with limit", v1.ReconcileRequest{Ranges: []v1.ReconcileRange{{Kind: "entity",
			Entities: []string{"ticket"}, Op: "fingerprint", Limit: 3}}}, http.StatusBadRequest, "no limit"},
		{"list without limit", v1.ReconcileRequest{Ranges: []v1.ReconcileRange{listRange(0, "ticket")}},
			http.StatusBadRequest, "outside"},
		{"list budget", v1.ReconcileRequest{Ranges: []v1.ReconcileRange{
			listRange(1500, "ticket"), listRange(1500, "feature")}}, http.StatusBadRequest, "add up"},
		{"lo above hi", v1.ReconcileRequest{Ranges: []v1.ReconcileRange{{Kind: "entity",
			Entities: []string{"ticket"}, Lo: "B", Hi: &hi, Op: "fingerprint"}}}, http.StatusBadRequest, ""},
		{"lo and after", v1.ReconcileRequest{Ranges: []v1.ReconcileRange{{Kind: "entity",
			Entities: []string{"ticket"}, Lo: "a", After: &after, Op: "fingerprint"}}},
			http.StatusBadRequest, "exclusive"},
		{"bound with NUL", v1.ReconcileRequest{Ranges: []v1.ReconcileRange{{Kind: "entity",
			Entities: []string{"ticket"}, Lo: "a\x00", Op: "fingerprint"}}}, http.StatusBadRequest, ""},
		{"body too large", `{"ranges":[{"kind":"entity","entities":["` + strings.Repeat("a", reconcileBodyLimit) +
			`"]}]}`, http.StatusRequestEntityTooLarge, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := f.post("alice", tc.body)
			if rec.Code != tc.code {
				t.Fatalf("status %d, want %d: %s", rec.Code, tc.code, rec.Body)
			}
			if tc.want != "" && !strings.Contains(rec.Body.String(), tc.want) {
				t.Errorf("body %s does not say %q", rec.Body, tc.want)
			}
		})
	}
}

func TestReconcile_MethodAndWiring(t *testing.T) {
	f := newReconcileFixture(t, reconcileGateConfig{})
	req := httptest.NewRequestWithContext(aliceCtx(), http.MethodGet, reconcilePath, nil)
	rec := httptest.NewRecorder()
	f.h.handleV1Reconcile(rec, req)
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != http.MethodPost {
		t.Errorf("GET: %d Allow=%q", rec.Code, rec.Header().Get("Allow"))
	}

	var unwired *reconcileHandler
	req = httptest.NewRequestWithContext(aliceCtx(), http.MethodPost, reconcilePath, strings.NewReader("{}"))
	rec = httptest.NewRecorder()
	unwired.handleV1Reconcile(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Errorf("unwired: %d, want 501", rec.Code)
	}

	if err := SetReconcile(f.app, nil); err == nil {
		t.Error("SetReconcile accepted a nil read")
	}
	if _, err := newReconcileHandler(f.app.State, nil, newReconcileGate(reconcileGateConfig{})); err == nil {
		t.Error("newReconcileHandler accepted a nil reader")
	}
	if _, err := newReconcileHandler(f.app.State, f.h.elements, nil); err == nil {
		t.Error("newReconcileHandler accepted a nil gate")
	}
	if _, err := newReconcileHandler(nil, f.h.elements, newReconcileGate(reconcileGateConfig{})); err == nil {
		t.Error("newReconcileHandler accepted a nil schema")
	}
}

// TestReconcile_NoIdentityFailsClosed: without a bound request the gated
// read refuses, and nothing of the range is answered.
func TestReconcile_NoIdentityFailsClosed(t *testing.T) {
	f := newReconcileFixture(t, reconcileGateConfig{})
	body, err := json.Marshal(v1.ReconcileRequest{Ranges: []v1.ReconcileRange{fpRange("ticket")}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(aliceCtx(), http.MethodPost, reconcilePath, bytes.NewReader(body))
	rec := httptest.NewRecorder()
	f.h.handleV1Reconcile(rec, req)
	if rec.Code != http.StatusUnauthorized || strings.Contains(rec.Body.String(), "fingerprint") {
		t.Errorf("unbound request: %d %s, want 401 and no answer", rec.Code, rec.Body)
	}
}

// stubElements answers every range with one error.
type stubElements struct {
	err   error
	delay time.Duration
}

func (s stubElements) Fingerprint(ctx context.Context, _ visibility.ElementTypes, _ store.ElementRange) (reconcile.Range, error) {
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return reconcile.Range{}, ctx.Err()
		}
	}
	return reconcile.Range{}, s.err
}

func (s stubElements) List(ctx context.Context, types visibility.ElementTypes, r store.ElementRange, _ int) (visibility.ElementPage, error) {
	_, err := s.Fingerprint(ctx, types, r)
	return visibility.ElementPage{}, err
}

func TestReconcile_RangeErrors(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		code   int
		status string
	}{
		{"too large answers per range", visibility.ErrRangeTooLarge, http.StatusOK, v1.ReconcileTooLarge},
		{"meta gated refuses", visibility.ErrRelationMetaGated, http.StatusUnprocessableEntity, ""},
		{"invalid query refuses", store.ErrInvalidQuery, http.StatusBadRequest, ""},
		{"other error is 500", errors.New("disk on fire"), http.StatusInternalServerError, ""},
		{"no identity is 401", acl.ErrNoIdentity, http.StatusUnauthorized, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newReconcileFixture(t, reconcileGateConfig{})
			f.h.elements = stubElements{err: tc.err}
			rec := f.post("alice", v1.ReconcileRequest{Ranges: []v1.ReconcileRange{fpRange("ticket")}})
			if rec.Code != tc.code {
				t.Fatalf("status %d, want %d: %s", rec.Code, tc.code, rec.Body)
			}
			if strings.Contains(rec.Body.String(), "disk on fire") {
				t.Error("an internal error reached the body")
			}
			if tc.status != "" {
				var out v1.ReconcileResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
					t.Fatal(err)
				}
				if out.Ranges[0].Status != tc.status || out.Ranges[0].Count != nil {
					t.Errorf("result %+v, want status %s and no count", out.Ranges[0], tc.status)
				}
			}
		})
	}
}

func TestReconcileGate(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	clock := func() time.Time { return now }

	t.Run("rate limit answers 429 with Retry-After", func(t *testing.T) {
		f := newReconcileFixture(t, reconcileGateConfig{Rate: rate.Every(10 * time.Second), Burst: 1, Now: clock})
		req := v1.ReconcileRequest{Ranges: []v1.ReconcileRange{fpRange("ticket")}}
		if rec := f.post("alice", req); rec.Code != http.StatusOK {
			t.Fatalf("first: %d", rec.Code)
		}
		rec := f.post("alice", req)
		if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "10" {
			t.Errorf("second: %d Retry-After=%q, want 429 and 10", rec.Code, rec.Header().Get("Retry-After"))
		}
		// A refused request does not push the next one further out.
		if rec := f.post("alice", req); rec.Header().Get("Retry-After") != "10" {
			t.Errorf("third: Retry-After=%q, want 10", rec.Header().Get("Retry-After"))
		}
		// Bob has his own bucket.
		if rec := f.post("bob", req); rec.Code != http.StatusOK {
			t.Errorf("bob: %d", rec.Code)
		}
	})

	t.Run("slots", func(t *testing.T) {
		g := newReconcileGate(reconcileGateConfig{Slots: 2, PerPrincipal: 1, Now: clock})
		relA, ref := g.acquire("alice")
		if ref != nil {
			t.Fatal(ref.code)
		}
		if _, r2 := g.acquire("alice"); r2 == nil || r2.status != http.StatusServiceUnavailable {
			t.Error("alice took a second slot over her cap")
		}
		relB, ref := g.acquire("bob")
		if ref != nil {
			t.Fatal(ref.code)
		}
		if _, r2 := g.acquire("carol"); r2 == nil || r2.status != http.StatusServiceUnavailable {
			t.Error("carol got a slot on a full server")
		}
		relA()
		relA() // release is idempotent
		if len(g.slots) != 1 {
			t.Errorf("slots in use = %d, want 1", len(g.slots))
		}
		relC, ref := g.acquire("carol")
		if ref != nil {
			t.Fatalf("carol after a release: %s", ref.code)
		}
		relB()
		relC()
	})

	t.Run("a busy refusal spends no token", func(t *testing.T) {
		g := newReconcileGate(reconcileGateConfig{Slots: 1, Rate: rate.Every(time.Hour), Burst: 1, Now: clock})
		rel, ref := g.acquire("bob")
		if ref != nil {
			t.Fatal(ref.code)
		}
		for range 3 {
			if _, r2 := g.acquire("alice"); r2 == nil || r2.status != http.StatusServiceUnavailable {
				t.Fatal("alice got a slot on a full server")
			}
		}
		rel()
		relA, ref := g.acquire("alice")
		if ref != nil {
			t.Fatalf("alice after her busy refusals: %s", ref.code)
		}
		relA()
	})

	t.Run("a full peer table evicts the oldest idle principal", func(t *testing.T) {
		g := newReconcileGate(reconcileGateConfig{MaxPeers: 2, IdleTTL: time.Hour, Now: clock})
		for _, u := range []string{"alice", "bob"} {
			rel, ref := g.acquire(u)
			if ref != nil {
				t.Fatal(ref.code)
			}
			rel()
			now = now.Add(time.Second)
		}
		rel, ref := g.acquire("carol")
		if ref != nil {
			t.Fatalf("carol on a table of idle principals: %s", ref.code)
		}
		rel()
		if _, ok := g.peers["alice"]; ok {
			t.Error("alice, seen longest ago, was not evicted")
		}
		if _, ok := g.peers["bob"]; !ok {
			t.Error("bob was evicted before alice")
		}
	})

	t.Run("peer table evicts idle principals only", func(t *testing.T) {
		g := newReconcileGate(reconcileGateConfig{MaxPeers: 1, IdleTTL: time.Minute, Now: clock})
		rel, ref := g.acquire("alice")
		if ref != nil {
			t.Fatal(ref.code)
		}
		now = now.Add(2 * time.Minute)
		if _, r2 := g.acquire("bob"); r2 == nil {
			t.Error("bob evicted alice while she held a slot")
		}
		rel()
		now = now.Add(2 * time.Minute)
		relB, ref := g.acquire("bob")
		if ref != nil {
			t.Fatalf("bob after alice went idle: %s", ref.code)
		}
		relB()
	})
}

// blockingWriter blocks its first Write until unblock is closed, like a
// peer that stops reading.
type blockingWriter struct {
	*httptest.ResponseRecorder
	writing chan struct{}
	unblock chan struct{}
	once    sync.Once
}

func (w *blockingWriter) Write(b []byte) (int, error) {
	w.once.Do(func() { close(w.writing) })
	<-w.unblock
	return w.ResponseRecorder.Write(b)
}

// TestReconcile_SlowReaderHoldsNoSlot: the slot is free before the answer is
// written, so a peer that stops reading cannot pin it.
func TestReconcile_SlowReaderHoldsNoSlot(t *testing.T) {
	f := newReconcileFixture(t, reconcileGateConfig{Slots: 1})
	body, err := json.Marshal(v1.ReconcileRequest{Ranges: []v1.ReconcileRange{fpRange("ticket")}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := gateCtxFor(principalCtx("alice"), t, f.d)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, reconcilePath, bytes.NewReader(body))
	w := &blockingWriter{ResponseRecorder: httptest.NewRecorder(),
		writing: make(chan struct{}), unblock: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		defer close(done)
		f.h.handleV1Reconcile(w, req)
	}()
	<-w.writing
	if n := len(f.h.gate.slots); n != 0 {
		t.Errorf("%d slots held while the answer is written, want 0", n)
	}
	close(w.unblock)
	<-done
	if w.Code != http.StatusOK {
		t.Errorf("status %d", w.Code)
	}
}

func TestReconcile_DeadlineAnswersNotRun(t *testing.T) {
	f := newReconcileFixture(t, reconcileGateConfig{})
	f.h.elements = stubElements{delay: time.Hour}
	f.h.deadline = 20 * time.Millisecond
	res := f.answer("alice", fpRange("ticket"), fpRange("feature"))
	for i, r := range res {
		if r.Status != v1.ReconcileNotRun || r.Count != nil || r.Fingerprint != "" {
			t.Errorf("range %d: %+v, want not_run and nothing else", i, r)
		}
	}
}

// TestReconcile_ClientCancelIsNotNotRun: the client's own cancellation is
// not the deadline, so no range is answered not_run on its behalf.
func TestReconcile_ClientCancelIsNotNotRun(t *testing.T) {
	f := newReconcileFixture(t, reconcileGateConfig{})
	f.h.elements = stubElements{delay: time.Hour}
	ctx, cancel := context.WithTimeout(gateCtxFor(principalCtx("alice"), t, f.d), 20*time.Millisecond)
	defer cancel()
	body, err := json.Marshal(v1.ReconcileRequest{Ranges: []v1.ReconcileRange{fpRange("ticket")}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, reconcilePath, bytes.NewReader(body))
	rec := httptest.NewRecorder()
	f.h.handleV1Reconcile(rec, req)
	if rec.Body.Len() != 0 || rec.Code != statusClientClosedRequest {
		t.Errorf("a cancelled client got %d %s, want 499 and no body", rec.Code, rec.Body)
	}
}

func TestReconcile_RelationRange(t *testing.T) {
	f := newReconcileFixture(t, reconcileGateConfig{})
	seedRelation(f.app, &entity.Relation{From: "TKT-001", Type: "implements", To: "FEAT-001"})
	seedRelation(f.app, &entity.Relation{From: "TKT-001", Type: "blocks", To: "TKT-002"})
	rr := v1.ReconcileRange{Kind: "relation", Entities: []string{"ticket", "feature"},
		Relations: []string{"implements", "blocks"}, Op: "list", Limit: 10}
	for user, want := range map[string]int{"alice": 2, "bob": 1} {
		res := f.answer(user, rr)[0]
		if len(res.Elements) != want {
			t.Errorf("%s sees %d relations, want %d: %+v", user, len(res.Elements), want, res.Elements)
		}
		for _, e := range res.Elements {
			if user == "bob" && strings.Contains(e.Address, "FEAT-") {
				t.Errorf("bob sees %s, whose endpoint he may not read", e.Address)
			}
		}
	}
}

// TestReconcile_CrossOriginRefused: a cookie-bearing or cross-origin
// request keeps the same-origin check; the shape a peer sends (no cookie,
// no Origin) reaches the handler, which the fixture does not wire: 501.
func TestReconcile_CrossOriginRefused(t *testing.T) {
	h := newSecuredTestApp(t).NewRouter()
	send := func(hdr map[string]string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, reconcilePath, strings.NewReader(`{}`))
		r.Host = "127.0.0.1:8080"
		for k, v := range hdr {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := send(map[string]string{"Origin": "https://evil.example", "Cookie": "session=abc"}); w.Code != http.StatusForbidden {
		t.Errorf("cross-origin = %d %s, want 403", w.Code, w.Body)
	}
	if w := send(map[string]string{"Cookie": "session=abc"}); w.Code != http.StatusForbidden {
		t.Errorf("origin-less cookie = %d %s, want 403", w.Code, w.Body)
	}
	if w := send(nil); w.Code != http.StatusNotImplemented {
		t.Errorf("peer request = %d %s, want 501", w.Code, w.Body)
	}
}
