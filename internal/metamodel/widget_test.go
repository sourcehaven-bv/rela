package metamodel

import "testing"

func TestResolveWidgetFromType(t *testing.T) {
	meta := &Metamodel{Types: map[string]CustomType{
		"priority": {Values: []string{"low", "high"}},
		"note":     {},
	}}
	tests := []struct {
		name     string
		meta     *Metamodel
		propType string
		want     string
	}{
		{"String", meta, PropertyTypeString, "text"},
		{"Date", meta, PropertyTypeDate, "date"},
		{"Datetime", meta, PropertyTypeDatetime, "datetime"},
		{"Integer", meta, PropertyTypeInteger, "number"},
		{"Boolean", meta, PropertyTypeBoolean, "checkbox"},
		{"Enum", meta, PropertyTypeEnum, "select"},
		{"Rrule", meta, PropertyTypeRrule, "rrule"},
		{"ExternalRef", meta, PropertyTypeExternalRef, "external-ref"},
		{"priority", meta, "priority", "select"},
		{"note", meta, "note", "text"},
		{"nope", meta, "nope", "text"},
		{"nil meta/Date", nil, PropertyTypeDate, "date"},
		{"nil meta/priority", nil, "priority", "text"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveWidgetFromType(tc.meta, tc.propType); got != tc.want {
				t.Errorf("ResolveWidgetFromType(%q) = %q, want %q", tc.propType, got, tc.want)
			}
		})
	}
}
