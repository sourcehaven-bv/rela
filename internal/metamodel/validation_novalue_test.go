package metamodel

import (
	"strings"
	"testing"
)

// TestValidatePropertyValue_MessageNeverQuotesValue pins BUG-197XPAU: a
// validation message reaches the writer, who may not be allowed to see the
// value, so no message quotes it. Each value carries the marker 9137, which
// a message about the rule never contains.
func TestValidatePropertyValue_MessageNeverQuotesValue(t *testing.T) {
	const marker = "9137"
	v := NewValidator(&Metamodel{Types: map[string]CustomType{
		"status": {Values: []string{"open", "done"}},
	}})
	bounded := &PropertyDef{Type: PropertyTypeInteger, Min: new(0), Max: new(5)}
	priced := &PropertyDef{Type: PropertyTypeDecimal, Min: new(0), Max: new(5), Scale: new(1)}
	tests := []struct {
		name string
		def  *PropertyDef
		val  any
	}{
		{"date", &PropertyDef{Type: PropertyTypeDate}, "x" + marker},
		{"datetime", &PropertyDef{Type: PropertyTypeDatetime}, "x" + marker},
		{"boolean", &PropertyDef{Type: PropertyTypeBoolean}, "x" + marker},
		{"enum", &PropertyDef{Type: PropertyTypeEnum, Values: []string{"a"}}, "x" + marker},
		{"custom enum", &PropertyDef{Type: "status"}, "x" + marker},
		{"custom enum list", &PropertyDef{Type: "status"}, []any{"open", "x" + marker}},
		{"custom enum string list", &PropertyDef{Type: "status"}, []string{"x" + marker}},
		{"integer string", &PropertyDef{Type: PropertyTypeInteger}, "x" + marker},
		{"integer fraction", &PropertyDef{Type: PropertyTypeInteger}, 9137.5},
		{"integer out of range", bounded, 9137},
		{"decimal", &PropertyDef{Type: PropertyTypeDecimal}, "x" + marker},
		{"decimal scale", priced, "1.9137"},
		{"decimal out of range", priced, "9137"},
		{"rrule", &PropertyDef{Type: PropertyTypeRrule}, "FREQ=X" + marker},
		{"external ref key", &PropertyDef{Type: PropertyTypeExternalRef}, map[string]any{"id": "a", "k" + marker: "1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.validatePropertyValue("p", tc.def, tc.val)
			if err == nil {
				t.Fatalf("want a validation error for %v", tc.val)
			}
			if strings.Contains(err.Message, marker) {
				t.Errorf("message quotes the value: %q", err.Message)
			}
		})
	}
}
