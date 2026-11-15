package scalerun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Fixtures are the ids and the search word the endpoint templates refer to.
// They are read from the running server, so the same config works at every
// scale and seed.
type Fixtures struct {
	Task    string `json:"task"`
	Project string `json:"project"`
	Person  string `json:"person"`
	Word    string `json:"word"`
}

// Driver sends the configured requests to one running server.
type Driver struct {
	// Base is the server's origin, e.g. http://127.0.0.1:18790. It is also
	// sent as Origin, which the server requires on API requests.
	Base string
	// PrincipalHeader names the header rela-server reads the identity from
	// (its -principal-header flag).
	PrincipalHeader string
	Client          *http.Client
	// RSS reports the server's resident set size in KiB. Nil skips it.
	RSS func() (int64, error)
	// RunID makes the ids {new} mints unique across runs against one
	// database, since a deleted id may stay reserved.
	RunID string
	// Log receives one line per measured endpoint. Nil discards them.
	Log io.Writer

	minted []string
	seq    int
}

// Sample is one measured request.
type Sample struct {
	Wall    time.Duration
	Queries int64
	DB      time.Duration
}

// EndpointResult aggregates the samples of one endpoint for one principal.
type EndpointResult struct {
	Endpoint   string  `json:"endpoint"`
	Principal  string  `json:"principal"`
	Method     string  `json:"method"`
	N          int     `json:"n"`
	P50MS      float64 `json:"p50_ms"`
	P95MS      float64 `json:"p95_ms"`
	MaxMS      float64 `json:"max_ms"`
	QueriesP50 int64   `json:"queries_p50"`
	QueriesMax int64   `json:"queries_max"`
	DBP50MS    float64 `json:"db_p50_ms"`
	RSSKB      int64   `json:"rss_kb,omitempty"`
	// Error is set when a request failed or answered an unexpected status.
	// The samples before it are kept.
	Error string `json:"error,omitempty"`
}

// ScaleResult is one scale's measurements.
type ScaleResult struct {
	Scale          float64          `json:"scale"`
	Entities       int              `json:"entities,omitempty"`
	Relations      int              `json:"relations,omitempty"`
	SeedSeconds    float64          `json:"seed_seconds,omitempty"`
	StartupSeconds float64          `json:"startup_seconds,omitempty"`
	RSSIdleKB      int64            `json:"rss_idle_kb,omitempty"`
	RSSPeakKB      int64            `json:"rss_peak_kb,omitempty"`
	Fixtures       Fixtures         `json:"fixtures"`
	Endpoints      []EndpointResult `json:"endpoints"`
}

// Results is what one run writes to results.json.
type Results struct {
	Commit     string        `json:"commit,omitempty"`
	Started    time.Time     `json:"started"`
	Iterations int           `json:"iterations"`
	Scales     []ScaleResult `json:"scales"`
}

// Scale returns the result for scale s.
func (r *Results) Scale(s float64) (*ScaleResult, bool) {
	for i := range r.Scales {
		if r.Scales[i].Scale == s {
			return &r.Scales[i], true
		}
	}
	return nil, false
}

// Find returns the result for an endpoint and principal.
func (s *ScaleResult) Find(endpoint, principal string) (EndpointResult, bool) {
	for _, e := range s.Endpoints {
		if e.Endpoint == endpoint && e.Principal == principal {
			return e, true
		}
	}
	return EndpointResult{}, false
}

// Fixtures reads the fixture ids from the server as principal: the first
// task, project and person by id, and the first word of the task's title.
// The person skips the principal's own row so its view shows someone else.
func (d *Driver) Fixtures(ctx context.Context, principal string) (Fixtures, error) {
	type row struct {
		ID         string         `json:"id"`
		Properties map[string]any `json:"properties"`
	}
	first := func(plural string, skip func(row) bool) (row, error) {
		var page struct {
			Data []row `json:"data"`
		}
		path := "/api/v1/" + plural + "?per_page=5&sort=id"
		status, body, err := d.do(ctx, http.MethodGet, path, "", principal)
		if err != nil {
			return row{}, err
		}
		if status != http.StatusOK {
			return row{}, fmt.Errorf("GET %s: status %d: %s", path, status, truncate(body))
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return row{}, fmt.Errorf("GET %s: %w", path, err)
		}
		for _, r := range page.Data {
			if skip == nil || !skip(r) {
				return r, nil
			}
		}
		return row{}, fmt.Errorf("GET %s: no usable row; is the database seeded?", path)
	}
	var f Fixtures
	task, err := first("tasks", nil)
	if err != nil {
		return f, err
	}
	project, err := first("projects", nil)
	if err != nil {
		return f, err
	}
	person, err := first("persons", func(r row) bool { return r.Properties["email"] == principal })
	if err != nil {
		return f, err
	}
	title, _ := task.Properties["title"].(string)
	word, _, _ := strings.Cut(title, " ")
	if word == "" {
		return f, fmt.Errorf("task %s has no title to search for", task.ID)
	}
	return Fixtures{Task: task.ID, Project: project.ID, Person: person.ID, Word: word}, nil
}

// Measure runs every endpoint for every principal it names, in config
// order, and returns one result per pair. It stops at the first transport
// error (the server is gone); an unexpected status is recorded on the
// result and the run continues.
func (d *Driver) Measure(ctx context.Context, c *Config, f Fixtures) ([]EndpointResult, error) {
	var out []EndpointResult
	for _, e := range c.Endpoints {
		for _, role := range e.As {
			res, err := d.measureOne(ctx, c, e, role, f)
			if err != nil {
				return out, err
			}
			out = append(out, res)
			if d.Log != nil {
				_, _ = fmt.Fprintf(d.Log, "  %-22s %-8s p50 %7.1f ms  p95 %7.1f ms  %4d queries  %s\n",
					res.Endpoint, res.Principal, res.P50MS, res.P95MS, res.QueriesMax, res.Error)
			}
		}
	}
	return out, nil
}

func (d *Driver) measureOne(
	ctx context.Context, c *Config, e Endpoint, role string, f Fixtures,
) (EndpointResult, error) {
	res := EndpointResult{Endpoint: e.Name, Principal: role, Method: e.Method}
	principal := c.Principals[role]
	var samples []Sample
	for i := range c.Warmup + c.Iterations {
		path, err := d.fill(e.Path, f, i, true)
		if err != nil {
			return res, fmt.Errorf("%s: %w", e.Name, err)
		}
		body, err := d.fill(e.Body, f, i, false)
		if err != nil {
			return res, fmt.Errorf("%s: %w", e.Name, err)
		}
		s, status, respBody, err := d.timed(ctx, e.Method, path, body, principal)
		if err != nil {
			return res, fmt.Errorf("%s as %s: %w", e.Name, role, err)
		}
		if status != e.Expect {
			res.Error = fmt.Sprintf("%s %s answered %d, want %d: %s", e.Method, path, status, e.Expect, truncate(respBody))
			break
		}
		if i >= c.Warmup {
			samples = append(samples, s)
		}
	}
	summarize(&res, samples)
	if d.RSS != nil {
		if kb, err := d.RSS(); err == nil {
			res.RSSKB = kb
		}
	}
	return res, nil
}

// summarize fills the percentile fields of res from samples.
func summarize(res *EndpointResult, samples []Sample) {
	res.N = len(samples)
	if len(samples) == 0 {
		return
	}
	walls := make([]float64, len(samples))
	dbs := make([]float64, len(samples))
	queries := make([]int64, len(samples))
	for i, s := range samples {
		walls[i] = ms(s.Wall)
		dbs[i] = ms(s.DB)
		queries[i] = s.Queries
	}
	slices.Sort(walls)
	slices.Sort(dbs)
	slices.Sort(queries)
	res.P50MS = round1(Percentile(walls, p50))
	res.P95MS = round1(Percentile(walls, p95))
	res.MaxMS = round1(walls[len(walls)-1])
	res.DBP50MS = round1(Percentile(dbs, p50))
	res.QueriesP50 = queries[len(queries)/2]
	res.QueriesMax = queries[len(queries)-1]
}

// The percentiles the results report.
const (
	p50 = 0.50
	p95 = 0.95
)

// Percentile returns the p-th percentile of sorted values by the nearest
// rank method: the smallest value with at least p of the values at or below
// it. With 20 samples p95 is the 19th value, so one outlier does not set it.
func Percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := max(int(math.Ceil(p*float64(len(sorted)))), 1)
	return sorted[min(rank, len(sorted))-1]
}

func (d *Driver) timed(
	ctx context.Context, method, path, body, principal string,
) (s Sample, status int, respBody []byte, err error) {
	start := time.Now()
	resp, err := d.send(ctx, method, path, body, principal)
	if err != nil {
		return Sample{}, 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	// The body is read before the clock stops: a client waits for all of it.
	respBody, err = io.ReadAll(resp.Body)
	wall := time.Since(start)
	if err != nil {
		return Sample{}, 0, nil, err
	}
	s = Sample{Wall: wall}
	q, db, ok := ParseServerTiming(resp.Header.Get("Server-Timing"))
	if !ok {
		return s, resp.StatusCode, respBody, errors.New("no Server-Timing statement count; is rela-server running with -verbose?")
	}
	s.Queries, s.DB = q, db
	return s, resp.StatusCode, respBody, nil
}

func (d *Driver) do(ctx context.Context, method, path, body, principal string) (status int, b []byte, err error) {
	resp, err := d.send(ctx, method, path, body, principal)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err = io.ReadAll(resp.Body)
	return resp.StatusCode, b, err
}

func (d *Driver) send(ctx context.Context, method, path, body, principal string) (*http.Response, error) {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, d.Base+path, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Origin", d.Base)
	req.Header.Set(d.PrincipalHeader, principal)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	client := d.Client
	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

var priorities = []string{"low", "medium", "high", "critical"}

// fill expands the placeholders in tmpl. In a path, fixture values are
// query-escaped; ids need no escaping in a JSON body.
func (d *Driver) fill(tmpl string, f Fixtures, iteration int, inPath bool) (string, error) {
	if tmpl == "" {
		return "", nil
	}
	esc := func(s string) string { return s }
	if inPath {
		esc = url.QueryEscape
	}
	out := strings.NewReplacer(
		"{task}", esc(f.Task),
		"{project}", esc(f.Project),
		"{person}", esc(f.Person),
		"{word}", esc(f.Word),
		"{priority}", priorities[iteration%len(priorities)],
	).Replace(tmpl)
	if strings.Contains(out, "{new}") {
		d.seq++
		id := fmt.Sprintf("TSK-SCALERUN-%s-%d", d.RunID, d.seq)
		d.minted = append(d.minted, id)
		out = strings.ReplaceAll(out, "{new}", id)
	}
	if strings.Contains(out, "{created}") {
		if len(d.minted) == 0 {
			return "", errors.New("{created} with no id left from {new}")
		}
		id := d.minted[0]
		d.minted = d.minted[1:]
		out = strings.ReplaceAll(out, "{created}", id)
	}
	return out, nil
}

func usesPlaceholder(e Endpoint, p string) bool {
	return strings.Contains(e.Path, p) || strings.Contains(e.Body, p)
}

var serverTiming = regexp.MustCompile(`(?:^|,)\s*db;dur=([0-9.]+);desc="(\d+) queries"`)

// ParseServerTiming reads the statement count and database time from the
// `db` metric rela-server sets under -verbose.
func ParseServerTiming(h string) (queries int64, db time.Duration, ok bool) {
	m := serverTiming.FindStringSubmatch(h)
	if m == nil {
		return 0, 0, false
	}
	dbMS, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, 0, false
	}
	q, err := strconv.ParseInt(m[2], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return q, time.Duration(dbMS * float64(time.Millisecond)), true
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func round1(f float64) float64 { return math.Round(f*10) / 10 }

func truncate(b []byte) string {
	const limit = 300
	s := strings.TrimSpace(string(b))
	if len(s) > limit {
		return s[:limit] + "..."
	}
	return s
}
