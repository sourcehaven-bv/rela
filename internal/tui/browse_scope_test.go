package tui

import (
	"testing"
)

func TestNewBrowseScope(t *testing.T) {
	tests := []struct {
		name   string
		ids    []string
		label  string
		origin Screen
		want   bool // nil expected?
	}{
		{
			name:   "creates scope with valid IDs",
			ids:    []string{"REQ-001", "REQ-002", "REQ-003"},
			label:  "3 requirements",
			origin: ScreenSearch,
			want:   false,
		},
		{
			name:   "returns nil for empty IDs",
			ids:    []string{},
			label:  "empty",
			origin: ScreenSearch,
			want:   true,
		},
		{
			name:   "returns nil for nil IDs",
			ids:    nil,
			label:  "nil",
			origin: ScreenBrowser,
			want:   true,
		},
		{
			name:   "single item scope is valid",
			ids:    []string{"REQ-001"},
			label:  "1 requirement",
			origin: ScreenBrowser,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope := NewBrowseScope(tt.ids, tt.label, tt.origin)
			if tt.want && scope != nil {
				t.Errorf("expected nil, got %+v", scope)
			}
			if !tt.want && scope == nil {
				t.Error("expected non-nil scope, got nil")
			}
			if scope != nil {
				if scope.Label != tt.label {
					t.Errorf("label = %q, want %q", scope.Label, tt.label)
				}
				if scope.Origin != tt.origin {
					t.Errorf("origin = %v, want %v", scope.Origin, tt.origin)
				}
				if scope.Index != 0 {
					t.Errorf("initial index = %d, want 0", scope.Index)
				}
			}
		})
	}
}

func TestBrowseScope_Current(t *testing.T) {
	tests := []struct {
		name  string
		scope *BrowseScope
		want  string
	}{
		{
			name:  "returns current ID",
			scope: &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 1},
			want:  "B",
		},
		{
			name:  "returns first ID at index 0",
			scope: &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 0},
			want:  "A",
		},
		{
			name:  "returns empty for nil scope",
			scope: nil,
			want:  "",
		},
		{
			name:  "returns empty for empty IDs",
			scope: &BrowseScope{IDs: []string{}, Index: 0},
			want:  "",
		},
		{
			name:  "returns empty for out of bounds index",
			scope: &BrowseScope{IDs: []string{"A"}, Index: 5},
			want:  "",
		},
		{
			name:  "returns empty for negative index",
			scope: &BrowseScope{IDs: []string{"A"}, Index: -1},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scope.Current()
			if got != tt.want {
				t.Errorf("Current() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBrowseScope_HasNext(t *testing.T) {
	tests := []struct {
		name  string
		scope *BrowseScope
		want  bool
	}{
		{
			name:  "true when not at end",
			scope: &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 0},
			want:  true,
		},
		{
			name:  "true in middle",
			scope: &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 1},
			want:  true,
		},
		{
			name:  "false at last item",
			scope: &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 2},
			want:  false,
		},
		{
			name:  "false for single item",
			scope: &BrowseScope{IDs: []string{"A"}, Index: 0},
			want:  false,
		},
		{
			name:  "false for nil scope",
			scope: nil,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scope.HasNext()
			if got != tt.want {
				t.Errorf("HasNext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBrowseScope_HasPrev(t *testing.T) {
	tests := []struct {
		name  string
		scope *BrowseScope
		want  bool
	}{
		{
			name:  "false at first item",
			scope: &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 0},
			want:  false,
		},
		{
			name:  "true in middle",
			scope: &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 1},
			want:  true,
		},
		{
			name:  "true at last item",
			scope: &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 2},
			want:  true,
		},
		{
			name:  "false for single item",
			scope: &BrowseScope{IDs: []string{"A"}, Index: 0},
			want:  false,
		},
		{
			name:  "false for nil scope",
			scope: nil,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scope.HasPrev()
			if got != tt.want {
				t.Errorf("HasPrev() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBrowseScope_Next(t *testing.T) {
	tests := []struct {
		name      string
		scope     *BrowseScope
		wantOK    bool
		wantIndex int
	}{
		{
			name:      "advances index",
			scope:     &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 0},
			wantOK:    true,
			wantIndex: 1,
		},
		{
			name:      "fails at end",
			scope:     &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 2},
			wantOK:    false,
			wantIndex: 2,
		},
		{
			name:      "fails for nil scope",
			scope:     nil,
			wantOK:    false,
			wantIndex: 0, // N/A
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scope.Next()
			if got != tt.wantOK {
				t.Errorf("Next() = %v, want %v", got, tt.wantOK)
			}
			if tt.scope != nil && tt.scope.Index != tt.wantIndex {
				t.Errorf("Index = %d, want %d", tt.scope.Index, tt.wantIndex)
			}
		})
	}
}

func TestBrowseScope_Prev(t *testing.T) {
	tests := []struct {
		name      string
		scope     *BrowseScope
		wantOK    bool
		wantIndex int
	}{
		{
			name:      "decrements index",
			scope:     &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 2},
			wantOK:    true,
			wantIndex: 1,
		},
		{
			name:      "fails at start",
			scope:     &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 0},
			wantOK:    false,
			wantIndex: 0,
		},
		{
			name:      "fails for nil scope",
			scope:     nil,
			wantOK:    false,
			wantIndex: 0, // N/A
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scope.Prev()
			if got != tt.wantOK {
				t.Errorf("Prev() = %v, want %v", got, tt.wantOK)
			}
			if tt.scope != nil && tt.scope.Index != tt.wantIndex {
				t.Errorf("Index = %d, want %d", tt.scope.Index, tt.wantIndex)
			}
		})
	}
}

func TestBrowseScope_Progress(t *testing.T) {
	tests := []struct {
		name  string
		scope *BrowseScope
		want  string
	}{
		{
			name:  "first of three",
			scope: &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 0},
			want:  "[1/3]",
		},
		{
			name:  "middle of three",
			scope: &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 1},
			want:  "[2/3]",
		},
		{
			name:  "last of three",
			scope: &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 2},
			want:  "[3/3]",
		},
		{
			name:  "single item",
			scope: &BrowseScope{IDs: []string{"A"}, Index: 0},
			want:  "[1/1]",
		},
		{
			name:  "nil scope",
			scope: nil,
			want:  "",
		},
		{
			name:  "empty IDs",
			scope: &BrowseScope{IDs: []string{}, Index: 0},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scope.Progress()
			if got != tt.want {
				t.Errorf("Progress() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBrowseScope_Count(t *testing.T) {
	tests := []struct {
		name  string
		scope *BrowseScope
		want  int
	}{
		{
			name:  "returns count",
			scope: &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 0},
			want:  3,
		},
		{
			name:  "nil scope returns 0",
			scope: nil,
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scope.Count()
			if got != tt.want {
				t.Errorf("Count() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBrowseScope_SetIndex(t *testing.T) {
	tests := []struct {
		name      string
		scope     *BrowseScope
		idx       int
		wantOK    bool
		wantIndex int
	}{
		{
			name:      "sets valid index",
			scope:     &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 0},
			idx:       2,
			wantOK:    true,
			wantIndex: 2,
		},
		{
			name:      "fails for negative index",
			scope:     &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 0},
			idx:       -1,
			wantOK:    false,
			wantIndex: 0,
		},
		{
			name:      "fails for out of bounds",
			scope:     &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 0},
			idx:       5,
			wantOK:    false,
			wantIndex: 0,
		},
		{
			name:      "fails for nil scope",
			scope:     nil,
			idx:       0,
			wantOK:    false,
			wantIndex: 0, // N/A
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scope.SetIndex(tt.idx)
			if got != tt.wantOK {
				t.Errorf("SetIndex(%d) = %v, want %v", tt.idx, got, tt.wantOK)
			}
			if tt.scope != nil && tt.scope.Index != tt.wantIndex {
				t.Errorf("Index = %d, want %d", tt.scope.Index, tt.wantIndex)
			}
		})
	}
}

func TestBrowseScope_IndexOf(t *testing.T) {
	tests := []struct {
		name  string
		scope *BrowseScope
		id    string
		want  int
	}{
		{
			name:  "finds existing ID",
			scope: &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 0},
			id:    "B",
			want:  1,
		},
		{
			name:  "returns -1 for missing ID",
			scope: &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 0},
			id:    "D",
			want:  -1,
		},
		{
			name:  "returns -1 for nil scope",
			scope: nil,
			id:    "A",
			want:  -1,
		},
		{
			name:  "finds first ID",
			scope: &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 0},
			id:    "A",
			want:  0,
		},
		{
			name:  "finds last ID",
			scope: &BrowseScope{IDs: []string{"A", "B", "C"}, Index: 0},
			id:    "C",
			want:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scope.IndexOf(tt.id)
			if got != tt.want {
				t.Errorf("IndexOf(%q) = %d, want %d", tt.id, got, tt.want)
			}
		})
	}
}

func TestBrowseScope_Navigation(t *testing.T) {
	// Integration test: navigate through a scope
	scope := NewBrowseScope([]string{"A", "B", "C", "D"}, "test", ScreenSearch)
	if scope == nil {
		t.Fatal("expected non-nil scope")
	}

	// Start at first
	if scope.Current() != "A" {
		t.Errorf("initial current = %q, want A", scope.Current())
	}
	if scope.Progress() != "[1/4]" {
		t.Errorf("initial progress = %q, want [1/4]", scope.Progress())
	}

	// Navigate forward
	if !scope.Next() {
		t.Error("Next() should succeed")
	}
	if scope.Current() != "B" {
		t.Errorf("after Next() current = %q, want B", scope.Current())
	}

	// Continue to end
	scope.Next() // C
	scope.Next() // D
	if scope.Current() != "D" {
		t.Errorf("at end current = %q, want D", scope.Current())
	}
	if scope.HasNext() {
		t.Error("HasNext() should be false at end")
	}
	if !scope.HasPrev() {
		t.Error("HasPrev() should be true at end")
	}

	// Can't go past end
	if scope.Next() {
		t.Error("Next() should fail at end")
	}
	if scope.Current() != "D" {
		t.Errorf("current should still be D after failed Next(), got %q", scope.Current())
	}

	// Navigate backward
	scope.Prev() // C
	scope.Prev() // B
	scope.Prev() // A
	if scope.Current() != "A" {
		t.Errorf("at start current = %q, want A", scope.Current())
	}
	if scope.HasPrev() {
		t.Error("HasPrev() should be false at start")
	}

	// Can't go past start
	if scope.Prev() {
		t.Error("Prev() should fail at start")
	}
}
