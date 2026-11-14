package scalerun

import (
	"strings"
	"testing"
	"time"
)

const benchCSV = `goos: linux
pkg: github.com/x/pgstore
,old,,new,,,
,sec/op,CI,sec/op,CI,vs base,P
GraphQuery/n=1000-4,0.001,1%,0.0013,2%,+30.00%,p=0.002 n=8
GraphQuery/n=10000-4,0.01,1%,0.0105,2%,~,p=0.300 n=8
Search/n=1000-4,0.002,1%,0.0012,2%,-40.00%,p=0.002 n=8
geomean,0.003,,0.004,,+12.00%,

,old,,new,,,
,B/op,CI,B/op,CI,vs base,P
GraphQuery/n=1000-4,100,0%,200,0%,+100.00%,p=0.002 n=8
`

func TestParseBenchstatCSV(t *testing.T) {
	got, err := ParseBenchstatCSV(strings.NewReader(benchCSV))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d rows, want the 3 sec/op rows: %+v", len(got), got)
	}
	if got[0].Name != "github.com/x/pgstore.GraphQuery/n=1000-4" || !got[0].Significant || got[0].Delta < 0.299 {
		t.Errorf("row 0 %+v", got[0])
	}
	if got[1].Significant {
		t.Errorf("~ read as significant: %+v", got[1])
	}
	if got[2].Delta > -0.39 {
		t.Errorf("row 2 %+v", got[2])
	}
	if _, err := ParseBenchstatCSV(strings.NewReader(",sec/op,CI,sec/op,CI,vs base,P\nA,1,,2,,+x%,\n")); err == nil {
		t.Error("bad delta accepted")
	}
}

func testResults() *Results {
	return &Results{
		Started:    time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC),
		Iterations: 5,
		Scales: []ScaleResult{
			{Scale: 1, Entities: 100, Endpoints: []EndpointResult{
				{Endpoint: "list", Principal: "manager", P50MS: 15, P95MS: 20, QueriesP50: 4, QueriesMax: 4},
				{Endpoint: "list", Principal: "reader", P50MS: 25, P95MS: 30},
				{Endpoint: "create", Principal: "manager", P95MS: 5},
			}},
			{Scale: 5, Entities: 500, Endpoints: []EndpointResult{
				{Endpoint: "list", Principal: "manager", P50MS: 45, P95MS: 60, QueriesP50: 4, QueriesMax: 6},
				{Endpoint: "list", Principal: "reader", P50MS: 45, P95MS: 55},
				{Endpoint: "create", Principal: "manager", P95MS: 9, Error: "answered 500"},
			}},
		},
	}
}

func TestCheck(t *testing.T) {
	c, err := ParseConfig([]byte(validConfig))
	if err != nil {
		t.Fatal(err)
	}
	c.Endpoints[0].P95MS["5"] = 50
	bench := []BenchDelta{
		{Name: "slower", Unit: "sec/op", Delta: 0.3, Significant: true},
		{Name: "noise", Unit: "sec/op"},
		{Name: "small", Unit: "sec/op", Delta: 0.1, Significant: true},
	}
	got := Check(c, testResults(), bench)
	var kinds []string
	for _, f := range got {
		kinds = append(kinds, f.Kind)
	}
	// error (create), p95 (list manager 60 > 50, reader 55 > 50),
	// ratio on p50 (manager 3x; reader 1.8x passes), bench (slower).
	want := "error p95 p95 ratio bench"
	if strings.Join(kinds, " ") != want {
		t.Fatalf("kinds %v, want %s; findings %+v", kinds, want, got)
	}
	if !strings.Contains(got[3].Message, "3.0x") {
		t.Errorf("ratio finding %q", got[3].Message)
	}

	// A per-endpoint ratio and a floor above the p95 both silence the ratio.
	c.Endpoints[0].RatioMax = 4
	if f := Check(c, testResults(), nil); hasKind(f, "ratio") {
		t.Errorf("ratio_max ignored: %+v", f)
	}
	c.Endpoints[0].RatioMax = 0
	c.Ratio.FloorMS = 100
	if f := Check(c, testResults(), nil); hasKind(f, "ratio") {
		t.Errorf("floor ignored: %+v", f)
	}
}

func hasKind(fs []Finding, kind string) bool {
	for _, f := range fs {
		if f.Kind == kind {
			return true
		}
	}
	return false
}

func TestWriteReport(t *testing.T) {
	c, err := ParseConfig([]byte(validConfig))
	if err != nil {
		t.Fatal(err)
	}
	r := testResults()
	r.Commit = "abc123"
	var b strings.Builder
	in := ReportInput{Config: c, Results: r, Findings: Check(c, r, nil), RunURL: "https://example/run",
		BenchNote: "No baseline."}
	if err := WriteReport(&b, in); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		"**Result: fail.**", "Commit `abc123`", "[Workflow run](https://example/run)",
		"## Scale 1", "## Scale 5", "| list | manager | 15.0 | 20.0 | 50 |", "4 (max 6)",
		"create (failed)", "## Scale 5 against scale 1", "| list | manager | 3.00x | 2.0x | 4 -> 6 |",
		"No baseline.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}

	b.Reset()
	in.Findings, in.Benchstat = nil, "A 1 2"
	if err := WriteReport(&b, in); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "**Result: pass.**") || !strings.Contains(b.String(), "```\nA 1 2\n```") {
		t.Errorf("pass report:\n%s", b.String())
	}
	b.Reset()
	in.BenchNote = "Not gated: renamed."
	_ = WriteReport(&b, in)
	if !strings.Contains(b.String(), "Not gated: renamed.\n\nCompared") {
		t.Errorf("note with table:\n%s", b.String())
	}
	b.Reset()
	in.Benchstat, in.BenchNote = "", ""
	_ = WriteReport(&b, in)
	if !strings.Contains(b.String(), "Not run.") {
		t.Errorf("no-bench report:\n%s", b.String())
	}
}
