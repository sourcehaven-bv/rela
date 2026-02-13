package transclusion

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []Transclusion
	}{
		{
			name:    "no transclusions",
			content: "Just some regular markdown text.",
			want:    nil,
		},
		{
			name:    "single transclusion",
			content: "See ![[REQ-001]] for details.",
			want: []Transclusion{
				{EntityID: "REQ-001", Section: "", Raw: "![[REQ-001]]", Start: 4, End: 16},
			},
		},
		{
			name:    "transclusion with section",
			content: "See ![[REQ-001#Rationale]] for why.",
			want: []Transclusion{
				{EntityID: "REQ-001", Section: "Rationale", Raw: "![[REQ-001#Rationale]]", Start: 4, End: 26},
			},
		},
		{
			name:    "multiple transclusions",
			content: "![[REQ-001]] and ![[REQ-002#Overview]]",
			want: []Transclusion{
				{EntityID: "REQ-001", Section: "", Raw: "![[REQ-001]]", Start: 0, End: 12},
				{EntityID: "REQ-002", Section: "Overview", Raw: "![[REQ-002#Overview]]", Start: 17, End: 38},
			},
		},
		{
			name:    "transclusion with spaces",
			content: "![[  REQ-001  #  Section Name  ]]",
			want: []Transclusion{
				{EntityID: "REQ-001", Section: "Section Name", Raw: "![[  REQ-001  #  Section Name  ]]", Start: 0, End: 33},
			},
		},
		{
			name:    "transclusion in code block - should be skipped",
			content: "```\n![[REQ-001]]\n```",
			want:    nil,
		},
		{
			name:    "transclusion after code block",
			content: "```\ncode\n```\n![[REQ-001]]",
			want: []Transclusion{
				{EntityID: "REQ-001", Section: "", Raw: "![[REQ-001]]", Start: 13, End: 25},
			},
		},
		{
			name:    "inline code does not protect",
			content: "`code` ![[REQ-001]]",
			want: []Transclusion{
				{EntityID: "REQ-001", Section: "", Raw: "![[REQ-001]]", Start: 7, End: 19},
			},
		},
		{
			name:    "entity ID with dashes and numbers",
			content: "![[my-entity-123]]",
			want: []Transclusion{
				{EntityID: "my-entity-123", Section: "", Raw: "![[my-entity-123]]", Start: 0, End: 18},
			},
		},
		{
			name:    "section with spaces and special chars",
			content: "![[REQ-001#My Section: Overview]]",
			want: []Transclusion{
				{EntityID: "REQ-001", Section: "My Section: Overview", Raw: "![[REQ-001#My Section: Overview]]", Start: 0, End: 33},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.content)
			if len(got) != len(tt.want) {
				t.Errorf("Parse() returned %d transclusions, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i].EntityID != tt.want[i].EntityID {
					t.Errorf("transclusion[%d].EntityID = %q, want %q", i, got[i].EntityID, tt.want[i].EntityID)
				}
				if got[i].Section != tt.want[i].Section {
					t.Errorf("transclusion[%d].Section = %q, want %q", i, got[i].Section, tt.want[i].Section)
				}
				if got[i].Raw != tt.want[i].Raw {
					t.Errorf("transclusion[%d].Raw = %q, want %q", i, got[i].Raw, tt.want[i].Raw)
				}
				if got[i].Start != tt.want[i].Start {
					t.Errorf("transclusion[%d].Start = %d, want %d", i, got[i].Start, tt.want[i].Start)
				}
				if got[i].End != tt.want[i].End {
					t.Errorf("transclusion[%d].End = %d, want %d", i, got[i].End, tt.want[i].End)
				}
			}
		})
	}
}

func TestHasTransclusions(t *testing.T) {
	tests := []struct {
		content string
		want    bool
	}{
		{"no transclusions", false},
		{"![[REQ-001]]", true},
		{"![[REQ-001#Section]]", true},
		{"[[REQ-001]]", false},           // Link, not transclusion
		{"```\n![[REQ-001]]\n```", true}, // HasTransclusions doesn't check code blocks
	}

	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			if got := HasTransclusions(tt.content); got != tt.want {
				t.Errorf("HasTransclusions(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

func TestReplace(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		replacer func(Transclusion) string
		want     string
	}{
		{
			name:    "no transclusions",
			content: "Just text.",
			replacer: func(_ Transclusion) string {
				return "[REPLACED]"
			},
			want: "Just text.",
		},
		{
			name:    "single replacement",
			content: "See ![[REQ-001]] for details.",
			replacer: func(t Transclusion) string {
				return "[" + t.EntityID + "]"
			},
			want: "See [REQ-001] for details.",
		},
		{
			name:    "multiple replacements",
			content: "![[A]] and ![[B]]",
			replacer: func(t Transclusion) string {
				return "<" + t.EntityID + ">"
			},
			want: "<A> and <B>",
		},
		{
			name:    "replacement with section",
			content: "![[REQ-001#Overview]]",
			replacer: func(t Transclusion) string {
				if t.Section != "" {
					return "[" + t.EntityID + ":" + t.Section + "]"
				}
				return "[" + t.EntityID + "]"
			},
			want: "[REQ-001:Overview]",
		},
		{
			name:    "replacement expands content",
			content: "Start ![[A]] End",
			replacer: func(_ Transclusion) string {
				return "MUCH LONGER REPLACEMENT TEXT"
			},
			want: "Start MUCH LONGER REPLACEMENT TEXT End",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Replace(tt.content, tt.replacer)
			if got != tt.want {
				t.Errorf("Replace() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsInsideCodeBlock(t *testing.T) {
	tests := []struct {
		content string
		pos     int
		want    bool
	}{
		{"no code", 3, false},
		{"```\ncode\n```", 5, true},
		{"```\ncode\n```\nafter", 14, false},
		{"before ```\ncode\n```", 12, true},
		{"``` a ``` b ```", 6, true},   // inside first block
		{"``` a ``` b ```", 10, false}, // between blocks
		{"``` a ``` b ```", 14, false}, // after third ``` (odd count = inside, but this is 3 = odd)
	}

	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			if got := isInsideCodeBlock(tt.content, tt.pos); got != tt.want {
				t.Errorf("isInsideCodeBlock(%q, %d) = %v, want %v", tt.content, tt.pos, got, tt.want)
			}
		})
	}
}
