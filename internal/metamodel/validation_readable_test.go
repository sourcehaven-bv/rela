package metamodel

import (
	"reflect"
	"slices"
	"testing"
)

// ValidateReadable drops findings on hidden properties and names the field
// of every other property finding (BUG-7928QLO).
func TestValidateReadable(t *testing.T) {
	meta := &Metamodel{
		Entities: map[string]EntityDef{
			"ticket": {
				IDPrefix: "TKT-",
				Properties: map[string]PropertyDef{
					"title":   {Type: "string", Required: true},
					"owner":   {Type: "string", Required: true, Label: "Owner"},
					"summary": {Type: "string", Required: true, Label: "Summary"},
				},
			},
		},
	}
	v := NewValidator(meta)
	tests := []struct {
		name   string
		id     string
		props  map[string]any
		hidden []string
		want   []string
	}{
		{
			name:  "label leads, name when there is none",
			id:    "TKT-1",
			props: map[string]any{},
			want: []string{
				"Owner: This field is required",
				"Summary: This field is required",
				"title: This field is required",
			},
		},
		{
			name:   "hidden property dropped",
			id:     "TKT-1",
			props:  map[string]any{"title": "t"},
			hidden: []string{"owner"},
			want:   []string{"Summary: This field is required"},
		},
		{
			name:  "entity finding keeps its message",
			id:    "BAD-1",
			props: map[string]any{"title": "t", "owner": "o", "summary": "s"},
			want:  []string{"entity ID BAD-1 does not match any prefix for type ticket: [TKT-]"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, e := range v.ValidateReadable(tc.id, "ticket", tc.props, tc.hidden) {
				got = append(got, e.Message)
			}
			if !reflect.DeepEqual(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(tc.want))) {
				t.Errorf("messages = %q, want %q", got, tc.want)
			}
		})
	}
}
