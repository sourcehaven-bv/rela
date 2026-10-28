package metamodel

import (
	"math"
	"strings"
	"testing"
)

// rangeSchema is a schema whose `risk.score` property carries the given
// option lines, indented under the property.
func rangeSchema(propLines string) string {
	return `
version: "1.0"
entities:
  risk:
    label: Risk
    id_prefix: "RSK-"
    properties:
      title:
        type: string
      score:
` + propLines
}

func TestParse_RangeOptions(t *testing.T) {
	tests := []struct {
		name    string
		props   string
		wantErr string // empty: loads
	}{
		{"integer range", "        type: integer\n        min: 1\n        max: 5\n", ""},
		{"min only", "        type: integer\n        min: 0\n", ""},
		{"max zero", "        type: integer\n        min: -5\n        max: 0\n", ""},
		{"equal bounds", "        type: integer\n        min: 3\n        max: 3\n", ""},
		{"min above max", "        type: integer\n        min: 5\n        max: 1\n", "min 5 above max 1"},
		{"min on string", "        type: string\n        min: 1\n", `min or max but is type "string"`},
		{"max on boolean", "        type: boolean\n        max: 1\n", `min or max but is type "boolean"`},
		{"min on file", "        type: file\n        min: 1\n", `sets "min" but is type "file"`},
		{"file max", "        type: file\n        max: 3\n", ""},
		{"file max zero means one file", "        type: file\n        max: 0\n", ""},
		{"file max negative", "        type: file\n        max: -1\n", "must be >= 1"},
		{"non-integer bound", "        type: integer\n        min: abc\n", "expected a number"},
		{"list integer", "        type: integer\n        list: true\n        max: 5\n", "not supported on a list"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(rangeSchema(tc.props)))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want load, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidatePropertyValue_IntegerRange(t *testing.T) {
	v := NewValidator(&Metamodel{})
	bounded := &PropertyDef{Type: PropertyTypeInteger, Min: new(1), Max: new(5)}
	minOnly := &PropertyDef{Type: PropertyTypeInteger, Min: new(0)}
	maxOnly := &PropertyDef{Type: PropertyTypeInteger, Max: new(0)}
	tests := []struct {
		name    string
		def     *PropertyDef
		val     any
		wantErr string // empty: valid
	}{
		{"int in range", bounded, 3, ""},
		{"lower bound", bounded, 1, ""},
		{"upper bound", bounded, 5, ""},
		{"int above", bounded, 7, "7 is out of range (must be from 1 to 5)"},
		{"int64 below", bounded, int64(0), "out of range"},
		{"float above", bounded, 6.0, "out of range"},
		{"huge float", bounded, 1e30, "out of range"},
		{"infinity", &PropertyDef{Type: PropertyTypeInteger}, math.Inf(1), "Invalid integer +Inf"},
		{"string above", bounded, "9", "out of range"},
		{"string in range", bounded, "2", ""},
		{"min only below", minOnly, -1, "must be at least 0"},
		{"min only far above", minOnly, 1 << 40, ""},
		{"max only above", maxOnly, 1, "must be at most 0"},
		{"max only negative", maxOnly, -100, ""},
		{"no range", &PropertyDef{Type: PropertyTypeInteger}, -1 << 40, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.validatePropertyValue("score", tc.def, tc.val)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want valid, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Message, tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
			// Out of range is soft: a write warns rather than rejects.
			if err.Type != ValidationErrorInvalidValue || !err.IsSoft() {
				t.Errorf("type = %s soft = %v, want soft invalid_value", err.Type, err.IsSoft())
			}
		})
	}
}

func TestPropertyDef_InRange(t *testing.T) {
	p := PropertyDef{Min: new(-2), Max: new(2)}
	for n, want := range map[int64]bool{-3: false, -2: true, 0: true, 2: true, 3: false} {
		if got := p.InRange(n); got != want {
			t.Errorf("InRange(%d) = %v, want %v", n, got, want)
		}
	}
}
