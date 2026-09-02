package lua

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestWithRequest_Absent pins that rela.request is nil when the caller did not
// pass one. Request-scoped execution is opt-in per action, and a script that
// checks `if rela.request then` must be able to tell the difference.
func TestWithRequest_Absent(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := NewWriter(ws.services("/tmp"), &buf)
	defer r.Close()

	if err := r.RunString(`rela.output({present = rela.request ~= nil})`); err != nil {
		t.Fatalf("RunString failed: %v", err)
	}
	var result map[string]bool
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if result["present"] {
		t.Error("rela.request must be absent when no request was supplied")
	}
}

// TestWithRequest_Exposed checks every field a request-scoped action sees.
func TestWithRequest_Exposed(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	req := Request{
		Method:      "POST",
		Path:        "/api/v1/_action/icinga",
		ContentType: "application/json",
		Raw:         `{"host":"web1","nested":{"state":2},"tags":["a","b"]}`,
		Body: map[string]any{
			"host":   "web1",
			"nested": map[string]any{"state": float64(2)},
			"tags":   []any{"a", "b"},
		},
		Query:   map[string][]string{"dry_run": {"1"}, "tag": {"x", "y"}},
		Headers: map[string]string{"x-event-type": "alert"},
	}

	r := NewWriter(ws.services("/tmp"), &buf, WithRequest(&req))
	defer r.Close()

	script := `
		rela.output({
			method       = rela.request.method,
			path         = rela.request.path,
			content_type = rela.request.content_type,
			host         = rela.request.body.host,
			state        = rela.request.body.nested.state,
			tag1         = rela.request.body.tags[1],
			raw_len      = #rela.request.raw,
			dry_run      = rela.request.query.dry_run,
			tag_first    = rela.request.query.tag,
			header       = rela.request.headers["x-event-type"],
		})
	`
	if err := r.RunString(script); err != nil {
		t.Fatalf("RunString failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("parse output: %v", err)
	}

	for _, tc := range []struct {
		key  string
		want any
	}{
		{"method", "POST"},
		{"path", "/api/v1/_action/icinga"},
		{"content_type", "application/json"},
		{"host", "web1"},
		{"state", float64(2)},
		{"tag1", "a"},
		{"raw_len", float64(len(req.Raw))},
		{"dry_run", "1"},
		{"tag_first", "x"},
		{"header", "alert"},
	} {
		if got := result[tc.key]; got != tc.want {
			t.Errorf("%s = %#v, want %#v", tc.key, got, tc.want)
		}
	}
}

// TestWithRequest_ReadOnly pins that a script cannot mutate rela.request.
// The table is the record of what actually arrived; a script that rewrote it
// would make a later read (an error envelope, an audit line) disagree with the
// wire, which is the class of bug that makes an incident report untrustworthy.
func TestWithRequest_ReadOnly(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	var buf bytes.Buffer

	r := NewWriter(ws.services("/tmp"), &buf,
		WithRequest(&Request{Method: "POST", Headers: map[string]string{"x-a": "1"}}))
	defer r.Close()

	err := r.RunString(`rela.request.method = "GET"`)
	if err == nil {
		t.Fatal("expected assigning to rela.request to raise an error")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected a read-only error, got: %v", err)
	}
}
