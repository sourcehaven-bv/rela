package browser

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/tools/seqtrace/diagram"
)

func TestSourceMap(t *testing.T) {
	// Line 1: column 0 from a.ts, column 5 from axios. Line 2: column 0,
	// still axios (the source index is relative to the previous segment).
	// Line 3: column 0 from axios, then an unmapped range from column 10.
	m, err := ParseSourceMap([]byte(`{"version":3,"sourceRoot":"","sources":["../src/a.ts",` +
		`"../node_modules/axios/x.js"],"mappings":"AAAA,KCAA;AADA;AAAA,U"}`))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		line, col int
		want      string
	}{
		{1, 1, "../src/a.ts"},
		{1, 5, "../src/a.ts"},
		{1, 6, "../node_modules/axios/x.js"},
		{2, 9, "../node_modules/axios/x.js"},
		{3, 5, "../node_modules/axios/x.js"},
	}
	for _, tc := range tests {
		if got, _ := m.Source(tc.line, tc.col); got != tc.want {
			t.Errorf("Source(%d, %d) = %q, want %q", tc.line, tc.col, got, tc.want)
		}
	}
	if _, ok := m.Source(3, 12); ok {
		t.Error("Source in an unmapped range should fail")
	}
	if _, ok := m.Source(4, 1); ok {
		t.Error("Source past the last line should fail")
	}
	if _, err := ParseSourceMap([]byte(`{"version":3,"sources":[],"mappings":"A!"}`)); err == nil {
		t.Error("bad mapping character should fail")
	}
}

// stack is a V8 stack as the recorder captures it from inside axios.
const stack = `Error
    at span (http://localhost:1/recorder.js:20:5)
    at XMLHttpRequest.send (http://localhost:1/recorder.js:44:16)
    at dispatchXhrRequest (http://localhost:1/assets/index.js:3:10)
    at ApiClient.get (http://localhost:1/assets/index.js:5:1)
    at fetchEntity (http://localhost:1/assets/index.js:7:1)
    at async onOpen (http://localhost:1/assets/index.js:9:1)
    at http://localhost:1/assets/index.js:11:1`

var sources = map[int]string{
	3:  "../../frontend/node_modules/axios/lib/xhr.js",
	5:  "../../frontend/src/api/client.ts",
	7:  "../../frontend/src/api/entities.ts",
	9:  "../../frontend/src/components/entity/EntityDetail.vue",
	11: "../../frontend/src/components/entity/EntityDetail.vue",
}

func resolve(script string, line, _ int) (string, bool) {
	if strings.HasSuffix(script, "recorder.js") {
		return "", false
	}
	s, ok := sources[line]
	return s, ok
}

func TestFrames(t *testing.T) {
	got := Frames(stack, resolve)
	want := []Frame{
		{"EntityDetail.vue", "src/components/entity/EntityDetail.vue"},
		{"onOpen", "src/components/entity/EntityDetail.vue"},
		{"fetchEntity", "src/api/entities.ts"},
		{"ApiClient.get", "src/api/client.ts"},
	}
	if len(got) != len(want) {
		t.Fatalf("Frames = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("frame %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestStitch(t *testing.T) {
	server := &diagram.Diagram{
		Participants: []string{"dataentry", "acl"},
		Root: diagram.Step{From: "Caller", To: "dataentry", Fn: "(*App).handleV1GetEntity", Outcome: "status:404",
			Children: []diagram.Step{{From: "dataentry", To: "acl", Fn: "(*Request).PermitsRead", Outcome: "false"}}},
	}
	rec := Record{Span: "p-1", Method: "GET", Path: "/api/v1/tasks/TSK-0001"}
	sc := Stitch("detail › opens a task", rec, Frames(stack, resolve), server, "internal/dataentry.(*App).handleV1GetEntity")

	if sc.Name != "detail › opens a task: GET /api/v1/tasks/{id}" {
		t.Errorf("Name = %q", sc.Name)
	}
	if got := strings.Join(sc.Diagram.Participants, ","); got != "web/components/entity,web/api,dataentry,acl" {
		t.Errorf("Participants = %s", got)
	}
	// Caller -> component -> api (one participant for entities.ts and
	// client.ts) -> dataentry over HTTP -> acl.
	var chain []string
	for s := &sc.Diagram.Root; ; s = &s.Children[0] {
		chain = append(chain, s.From+">"+s.To+":"+s.Fn+"/"+s.Via+"/"+s.Outcome)
		if len(s.Children) == 0 {
			break
		}
	}
	want := "Caller>web/components/entity:EntityDetail.vue//," +
		"web/components/entity>web/api:fetchEntity//," +
		"web/api>dataentry:(*App).handleV1GetEntity/http/status:404," +
		"dataentry>acl:(*Request).PermitsRead//false"
	if got := strings.Join(chain, ","); got != want {
		t.Errorf("chain =\n%s\nwant\n%s", got, want)
	}
	hop := sc.Diagram.Root.Children[0].Children[0]
	if hop.Span != "p-1" || hop.Route == nil || *hop.Route != (diagram.Route{Method: "GET", Path: "/api/v1/tasks/{id}"}) {
		t.Errorf("hop span %q, route %+v", hop.Span, hop.Route)
	}
}

func TestRouteShape(t *testing.T) {
	tests := map[string]string{
		"/api/v1/tasks/TSK-0001":           "/api/v1/tasks/{id}",
		"/api/v1/_comments/tasks/TSK-0001": "/api/v1/_comments/tasks/{id}",
		"/api/v1/_schema":                  "/api/v1/_schema",
		"/api/v1/tasks":                    "/api/v1/tasks",
	}
	for in, want := range tests {
		if got := RouteShape(in); got != want {
			t.Errorf("RouteShape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParticipant(t *testing.T) {
	tests := map[string]string{
		"src/main.ts":                            "web/src",
		"src/api/client.ts":                      "web/api",
		"src/components/entity/EntityDetail.vue": "web/components/entity",
	}
	for in, want := range tests {
		if got := Participant(in); got != want {
			t.Errorf("Participant(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSpan(t *testing.T) {
	root := &diagram.Call{Event: diagram.Event{Args: []string{"w=*http.response", "r=*http.Request{Span:p-7}"}}}
	if got := Span(root); got != "p-7" {
		t.Errorf("Span = %q", got)
	}
}
