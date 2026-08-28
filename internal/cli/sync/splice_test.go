package sync

import (
	"maps"
	"reflect"
	"testing"
)

// TestMergeProperties is the crux of TKT-8P1TM7's no-data-loss guarantee: a
// redacted fetch spliced onto the replica's raw local record must distinguish a
// HIDDEN field (named in _redacted → keep the local value) from a DELETED field
// (in neither properties nor _redacted → drop it), while upserting visible ones.
func TestMergeProperties(t *testing.T) {
	strs := func(ss ...string) *[]string { return &ss }

	tests := []struct {
		name     string
		prior    map[string]any
		visible  map[string]any
		redacted *[]string
		want     map[string]any
	}{
		{
			name:     "hidden field preserved from prior, not erased",
			prior:    map[string]any{"title": "old", "salary": 100},
			visible:  map[string]any{"title": "new"},
			redacted: strs("salary"),
			want:     map[string]any{"title": "new", "salary": 100},
		},
		{
			name:     "deleted field (in neither) dropped",
			prior:    map[string]any{"title": "old", "note": "gone"},
			visible:  map[string]any{"title": "new"},
			redacted: strs(), // evaluated, nothing hidden
			want:     map[string]any{"title": "new"},
		},
		{
			name:     "visible field upserted",
			prior:    map[string]any{"title": "old"},
			visible:  map[string]any{"title": "new", "added": 1},
			redacted: strs(),
			want:     map[string]any{"title": "new", "added": 1},
		},
		{
			name:     "hidden preserved AND sibling deleted in one splice",
			prior:    map[string]any{"title": "old", "salary": 100, "note": "gone"},
			visible:  map[string]any{"title": "new"},
			redacted: strs("salary"),
			want:     map[string]any{"title": "new", "salary": 100},
		},
		{
			name:     "first landing (nil prior) takes visible body",
			prior:    nil,
			visible:  map[string]any{"title": "new"},
			redacted: strs(),
			want:     map[string]any{"title": "new"},
		},
		{
			name:     "nil redacted (no field affordances) treats absence as delete",
			prior:    map[string]any{"title": "old", "note": "gone"},
			visible:  map[string]any{"title": "new"},
			redacted: nil,
			want:     map[string]any{"title": "new"},
		},
		{
			name:     "hidden field with no prior value stays absent (nothing to preserve)",
			prior:    map[string]any{"title": "old"},
			visible:  map[string]any{"title": "new"},
			redacted: strs("salary"), // hidden but replica never had it
			want:     map[string]any{"title": "new"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Snapshot prior BEFORE the call so we can prove it isn't mutated (the
			// splice reads the replica's own raw record — mutating it would be a bug).
			priorBefore := map[string]any{}
			maps.Copy(priorBefore, tc.prior)

			got := mergeProperties(tc.prior, tc.visible, tc.redacted)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("mergeProperties() = %#v, want %#v", got, tc.want)
			}
			// The merge must never mutate the caller's prior map.
			if tc.prior != nil && !reflect.DeepEqual(tc.prior, priorBefore) {
				t.Errorf("mergeProperties mutated prior: %#v, was %#v", tc.prior, priorBefore)
			}
		})
	}
}
