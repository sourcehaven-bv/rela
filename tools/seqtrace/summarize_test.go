package seqtrace

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type entityLike struct {
	Type, ID string
	Body     string
}

type kind string

type sev int

func (sev) String() string { panic("String called") }

type mineError struct{}

func (*mineError) Error() string { return "mine" }

func TestSummarize(t *testing.T) {
	var nilErr *mineError
	tests := []struct {
		name string
		v    any
		want string
	}{
		{"nil", nil, "nil"},
		{"error", errors.New("boom"), "err: boom"},
		{"typed nil error", error(nilErr), "nil"},
		{"string", "TSK-1", `"TSK-1"`},
		{"long string", strings.Repeat("a", maxValue+8), `"` + strings.Repeat("a", maxValue) + `…"`},
		{"bytes", []byte("abc"), "[]byte len=3"},
		{"struct pointer", &entityLike{Type: "task", ID: "TSK-1", Body: "x"}, "*seqtrace.entityLike{Type:task ID:TSK-1}"},
		{"struct without ids", struct{ X int }{1}, "struct { X int }"},
		{"slice", []int{1, 2}, "[]int len=2"},
		{"map", map[string]int{"a": 1}, "map[string]int len=1"},
		{"func", func() {}, "func"},
		{"named string", kind("task"), `kind("task")`},
		{"int", 42, "42"},
		{"int with String", sev(3), "3"},
		{"float", 1.5, "1.5"},
		{"uint", uint8(7), "7"},
		{"bool", true, "true"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := summarize(tc.v); got != tc.want {
				t.Errorf("summarize = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSummarizeArgs(t *testing.T) {
	tests := []struct {
		name, fn string
		pairs    []any
		want     []string
	}{
		{"plain", "Get", []any{"ctx", context.Background(), "id", "T-1", "apiToken", "s3cret", "odd"},
			[]string{`id="T-1"`, "apiToken=‹redacted›"}},
		{"auth is not secret for functions", "(*Request).AuthorizeWrite", []any{"id", "T-1"}, []string{`id="T-1"`}},
		{"secret function", "stripBearer", []any{"v", "Bearer abc"}, []string{"v=‹redacted›"}},
		{"opaque", "normalize", []any{"v", Opaque(json.Number("110000"))}, []string{"v=json.Number"}},
		{"opaque nil", "normalize", []any{"v", Opaque(nil)}, []string{"v=nil"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := summarizeArgs(tc.fn, tc.pairs); strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("summarizeArgs = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSummarizeResults(t *testing.T) {
	if got := summarizeResults("(*Config).resolvePassword", []any{"hunter2"}); got[0] != redacted {
		t.Errorf("secret function result = %q", got)
	}
	got := summarizeResults("Get", []any{"x", errors.New("line1\nline2")})
	if got[0] != `"x"` || got[1] != `err: line1\nline2` {
		t.Errorf("summarizeResults = %q", got)
	}
}
