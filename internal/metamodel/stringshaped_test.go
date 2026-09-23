package metamodel

import "testing"

func TestStringShaped(t *testing.T) {
	m := &Metamodel{Types: map[string]CustomType{"status": {Values: []string{"open"}}}}
	cases := []struct {
		name string
		m    *Metamodel
		pd   PropertyDef
		want bool
	}{
		{"string", m, PropertyDef{Type: PropertyTypeString}, true},
		{"enum", m, PropertyDef{Type: PropertyTypeEnum}, true},
		{"date", m, PropertyDef{Type: PropertyTypeDate}, true},
		{"datetime", m, PropertyDef{Type: PropertyTypeDatetime}, true},
		{"declared custom type", m, PropertyDef{Type: "status"}, true},
		{"undeclared custom type", m, PropertyDef{Type: "nope"}, false},
		{"integer", m, PropertyDef{Type: PropertyTypeInteger}, false},
		{"string list", m, PropertyDef{Type: PropertyTypeString, List: true}, false},
		{"nil metamodel, string", nil, PropertyDef{Type: PropertyTypeString}, true},
		{"nil metamodel, custom type", nil, PropertyDef{Type: "status"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := StringShaped(tc.m, tc.pd); got != tc.want {
				t.Errorf("StringShaped = %v, want %v", got, tc.want)
			}
		})
	}
}
