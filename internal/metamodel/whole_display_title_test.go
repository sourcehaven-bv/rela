package metamodel

import "testing"

// TestWholeDisplayTitle pins that a title missing any of its properties
// falls back to the id, also for one placeholder of a template.
func TestWholeDisplayTitle(t *testing.T) {
	m := &Metamodel{Entities: map[string]EntityDef{
		"person": {DisplayProperty: "{first} {last}", Properties: map[string]PropertyDef{
			"first": {Type: PropertyTypeString}, "last": {Type: PropertyTypeString},
		}},
		"note": {DisplayProperty: "title", Properties: map[string]PropertyDef{
			"title": {Type: PropertyTypeString},
		}},
	}}
	tests := []struct {
		name, typ string
		props     map[string]any
		want      string
	}{
		{name: "template whole", typ: "person", props: map[string]any{"first": "Ada", "last": "Lovelace"},
			want: "Ada Lovelace"},
		{name: "template half", typ: "person", props: map[string]any{"first": "Ada"}, want: "ID-1"},
		{name: "property", typ: "note", props: map[string]any{"title": "Hello"}, want: "Hello"},
		{name: "property absent", typ: "note", props: map[string]any{}, want: "ID-1"},
		{name: "unknown type", typ: "other", props: map[string]any{"title": "x"}, want: "ID-1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.WholeDisplayTitle("ID-1", tc.typ, tc.props); got != tc.want {
				t.Errorf("WholeDisplayTitle = %q, want %q", got, tc.want)
			}
		})
	}
}
