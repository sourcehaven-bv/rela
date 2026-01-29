package mcp

import "testing"

func TestIsRelevantFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"entities/requirement/REQ-001.md", true},
		{"metamodel.yaml", true},
		{"config.yml", true},
		{"main.go", false},
		{"README.txt", false},
		{"file.json", false},
		{"", false},
		{"path/to/file.MD", false}, // case sensitive
	}

	for _, tt := range tests {
		got := isRelevantFile(tt.path)
		if got != tt.want {
			t.Errorf("isRelevantFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
