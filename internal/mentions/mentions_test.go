package mentions

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTargets(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "empty", body: "", want: nil},
		{name: "plain text id", body: "see TKT-1 here", want: nil},
		{name: "single span", body: "see `TKT-1` here", want: []string{"TKT-1"}},
		{name: "sorted and deduplicated", body: "`B-2` and `A-1` and `B-2`", want: []string{"A-1", "B-2"}},
		{name: "span with extra text", body: "`see TKT-1`", want: nil},
		{name: "fenced block", body: "```\nTKT-1\n```", want: nil},
		{name: "indented block", body: "para\n\n    `TKT-1`\n", want: nil},
		{name: "face address is not an id", body: "`TKT-1@draft`", want: nil},
		{name: "path traversal rejected", body: "`../etc`", want: nil},
		{name: "in table cell", body: "| a |\n|---|\n| `TKT-9` |\n", want: []string{"TKT-9"}},
		{name: "in link text", body: "[`TKT-3`](http://example.com)", want: nil},
		{name: "in emphasis", body: "*`TKT-4`*", want: []string{"TKT-4"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Targets(tc.body))
		})
	}
}

func TestTargets_Cap(t *testing.T) {
	var b strings.Builder
	for i := range MaxTargets + 10 {
		fmt.Fprintf(&b, "`X-%05d` ", i)
	}
	got := Targets(b.String())
	assert.Len(t, got, MaxTargets)
	assert.Equal(t, "X-00000", got[0])
	assert.Equal(t, fmt.Sprintf("X-%05d", MaxTargets-1), got[len(got)-1])
}

// TestAll_Uncapped: forward links are not capped; only the index is.
func TestAll_Uncapped(t *testing.T) {
	var b strings.Builder
	for i := range MaxTargets + 10 {
		fmt.Fprintf(&b, "`X-%05d` ", i)
	}
	assert.Len(t, All(b.String()), MaxTargets+10)
	assert.Len(t, Index(b.String()), MaxTargets)
}

func TestSpans(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "empty body", body: "", want: nil},
		{name: "no spans", body: "plain text", want: nil},
		{name: "order kept", body: "`b` then `a` then `c`", want: []string{"b", "a", "c"}},
		{name: "duplicates once", body: "`a` `b` `a`", want: []string{"a", "b"}},
		{name: "span in link text skipped", body: "[`USR-7`](http://example.com) `x`", want: []string{"x"}},
		{name: "fenced block skipped", body: "```\n`USR-7`\n```", want: nil},
		{name: "indented block skipped", body: "para\n\n    `USR-7`\n", want: nil},
		{name: "span in table cell found", body: "| a |\n|---|\n| `USR-9` |\n", want: []string{"USR-9"}},
		{name: "emphasis markup inside a span is literal text", body: "`*USR-7*`", want: []string{"*USR-7*"}},
		{name: "line break refused", body: "`USR-\n7`", want: nil},
		{name: "hard line break refused", body: "`a  \nb`", want: nil},
		{name: "job identity", body: "ask `integration:triage-bot`", want: []string{"integration:triage-bot"}},
		{name: "login form", body: "`@ann@example.org`", want: []string{"@ann@example.org"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Spans(tc.body))
		})
	}
}

func TestAll(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "empty", body: "", want: nil},
		{name: "single span", body: "see `TKT-1` here", want: []string{"TKT-1"}},
		{name: "sorted and deduplicated", body: "`B-2` and `A-1` and `B-2`", want: []string{"A-1", "B-2"}},
		{name: "non-id spans dropped", body: "`integration:x` `@ann` `TKT-2`", want: []string{"TKT-2"}},
		{name: "in link text", body: "[`TKT-3`](http://example.com)", want: nil},
		{name: "line break in span", body: "`TKT-\n1`", want: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, All(tc.body))
		})
	}
}
