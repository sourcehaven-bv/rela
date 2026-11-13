package scalerun

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPercentile(t *testing.T) {
	vals := make([]float64, 20)
	for i := range vals {
		vals[i] = float64(i + 1)
	}
	tests := []struct {
		p    float64
		want float64
	}{{0.5, 10}, {0.95, 19}, {1, 20}, {0, 1}}
	for _, tc := range tests {
		if got := Percentile(vals, tc.p); got != tc.want {
			t.Errorf("p%.0f = %v, want %v", tc.p*100, got, tc.want)
		}
	}
	if Percentile(nil, 0.5) != 0 {
		t.Error("empty input")
	}
}

func TestParseServerTiming(t *testing.T) {
	q, db, ok := ParseServerTiming(`db;dur=12.5;desc="7 queries"`)
	if !ok || q != 7 || db != 12500*time.Microsecond {
		t.Fatalf("got %d %v %v", q, db, ok)
	}
	if _, _, ok := ParseServerTiming(`cache;dur=1`); ok {
		t.Fatal("parsed a header without the db metric")
	}
	if _, _, ok := ParseServerTiming(""); ok {
		t.Fatal("parsed an empty header")
	}
}

// fakeServer answers like rela-server under -verbose: every response
// carries the db Server-Timing metric. It records the requests it saw.
type fakeServer struct {
	mu       sync.Mutex
	requests []string
	noTiming bool
	status   int
}

func (f *fakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, fmt.Sprintf("%s %s %s %s", r.Method, r.URL.RequestURI(),
		r.Header.Get("X-Who"), r.Header.Get("Origin")))
	f.mu.Unlock()
	if !f.noTiming {
		w.Header().Set("Server-Timing", `db;dur=2.0;desc="4 queries"`)
	}
	switch {
	case f.status != 0:
		w.WriteHeader(f.status)
	case r.Method == http.MethodPost:
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodDelete:
		w.WriteHeader(http.StatusNoContent)
	}
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/") {
		plural := strings.TrimPrefix(r.URL.Path, "/api/v1/")
		rows := map[string]string{
			"tasks":    `[{"id":"TSK-1","properties":{"title":"Rotate keys"}}]`,
			"projects": `[{"id":"PRJ-1","properties":{}}]`,
			"persons":  `[{"id":"PERS-1","properties":{"email":"alice@x"}},{"id":"PERS-2","properties":{"email":"bob@x"}}]`,
		}[plural]
		if rows != "" {
			_, _ = fmt.Fprintf(w, `{"data":%s}`, rows)
		}
	}
}

func testDriver(t *testing.T, f *fakeServer) *Driver {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return &Driver{Base: srv.URL, PrincipalHeader: "X-Who", Client: srv.Client(), RunID: "r1",
		RSS: func() (int64, error) { return 2048, nil }}
}

func TestDriver_FixturesAndMeasure(t *testing.T) {
	f := &fakeServer{}
	d := testDriver(t, f)
	var log strings.Builder
	d.Log = &log
	c, err := ParseConfig([]byte(validConfig))
	if err != nil {
		t.Fatal(err)
	}
	c.Endpoints[0].Path = "/api/v1/_search?q={word}&p={priority}"
	fx, err := d.Fixtures(context.Background(), "alice@x")
	if err != nil {
		t.Fatal(err)
	}
	want := Fixtures{Task: "TSK-1", Project: "PRJ-1", Person: "PERS-2", Word: "Rotate"}
	if fx != want {
		t.Fatalf("fixtures %+v, want %+v", fx, want)
	}
	f.requests = nil
	res, err := d.Measure(context.Background(), c, fx)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 4 {
		t.Fatalf("got %d results, want 4 (list x2, create, delete)", len(res))
	}
	for _, r := range res {
		if r.Error != "" || r.N != c.Iterations || r.QueriesMax != 4 || r.DBP50MS != 2 || r.RSSKB != 2048 {
			t.Errorf("result %+v", r)
		}
	}
	// Six requests per pair (warmup 1 + iterations 5).
	if len(f.requests) != 24 {
		t.Fatalf("server saw %d requests, want 24", len(f.requests))
	}
	if !strings.HasPrefix(f.requests[0], "GET /api/v1/_search?q=Rotate&p=low alice@x http://127.0.0.1") {
		t.Errorf("first request %q", f.requests[0])
	}
	if !strings.HasPrefix(f.requests[1], "GET /api/v1/_search?q=Rotate&p=medium") {
		t.Errorf("priority does not cycle: %q", f.requests[1])
	}
	// Deletes take the minted ids in order.
	if !strings.HasPrefix(f.requests[18], "DELETE /api/v1/tasks/TSK-SCALERUN-r1-1 ") {
		t.Errorf("first delete %q", f.requests[18])
	}
	if !strings.Contains(log.String(), "create") {
		t.Errorf("log %q", log.String())
	}
}

func TestDriver_UnexpectedStatusIsRecorded(t *testing.T) {
	f := &fakeServer{status: http.StatusForbidden}
	d := testDriver(t, f)
	c, err := ParseConfig([]byte(validConfig))
	if err != nil {
		t.Fatal(err)
	}
	c.Endpoints = c.Endpoints[:1]
	res, err := d.Measure(context.Background(), c, Fixtures{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res[0].Error, "answered 403, want 200") || res[0].N != 0 {
		t.Fatalf("result %+v", res[0])
	}
	if _, err := d.Fixtures(context.Background(), "alice@x"); err == nil {
		t.Fatal("fixtures accepted a 403")
	}
}

func TestDriver_MissingServerTimingIsAnError(t *testing.T) {
	d := testDriver(t, &fakeServer{noTiming: true})
	c, err := ParseConfig([]byte(validConfig))
	if err != nil {
		t.Fatal(err)
	}
	_, err = d.Measure(context.Background(), c, Fixtures{})
	if err == nil || !strings.Contains(err.Error(), "-verbose") {
		t.Fatalf("got %v", err)
	}
}

func TestDriver_TransportErrorStops(t *testing.T) {
	d := &Driver{Base: "http://127.0.0.1:1", PrincipalHeader: "X-Who"}
	c, err := ParseConfig([]byte(validConfig))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Measure(context.Background(), c, Fixtures{}); err == nil {
		t.Fatal("no error for an unreachable server")
	}
}

func TestDriver_CreatedWithoutNew(t *testing.T) {
	d := &Driver{}
	if _, err := d.fill("/x/{created}", Fixtures{}, 0, true); err == nil {
		t.Fatal("{created} with an empty pool accepted")
	}
	got, err := d.fill("/x?q={word}", Fixtures{Word: "a b"}, 0, true)
	if err != nil || got != "/x?q=a+b" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestResults_JSONRoundTrip(t *testing.T) {
	r := Results{Iterations: 3, Scales: []ScaleResult{{Scale: 1, Endpoints: []EndpointResult{{Endpoint: "a", Principal: "m"}}}}}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var back Results
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	s, ok := back.Scale(1)
	if !ok {
		t.Fatal("scale 1 missing")
	}
	if _, ok := s.Find("a", "m"); !ok {
		t.Fatal("endpoint missing")
	}
	if _, ok := back.Scale(5); ok {
		t.Fatal("scale 5 found")
	}
}
