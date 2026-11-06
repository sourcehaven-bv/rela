package metamodel

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestParseDecimal(t *testing.T) {
	tests := []struct {
		name      string
		in        any
		wantRat   string // big.Rat RatString; empty: error
		wantScale int
	}{
		{"string with trailing zero", "12.50", "25/2", 2},
		{"plain integer string", "7", "7", 0},
		{"negative", "-0.125", "-1/8", 3},
		{"plus sign", "+3.0", "3", 1},
		{"int", 42, "42", 0},
		{"int64", int64(-5), "-5", 0},
		{"float reads its shortest text", 0.1, "1/10", 1},
		{"json number", json.Number("19.99"), "1999/100", 2},
		{"exponent refused", "1e3", "", 0},
		{"leading point refused", ".5", "", 0},
		{"trailing point refused", "5.", "", 0},
		{"comma refused", "1,5", "", 0},
		{"two signs refused", "--1", "", 0},
		{"space refused", " 1", "", 0},
		{"empty refused", "", "", 0},
		{"too long refused", strings.Repeat("9", maxDecimalDigits+1), "", 0},
		{"infinity refused", math.Inf(1), "", 0},
		{"NaN refused", math.NaN(), "", 0},
		{"bool refused", true, "", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, scale, err := ParseDecimal(tc.in)
			if tc.wantRat == "" {
				if err == nil {
					t.Fatalf("want error, got %s", r.RatString())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r.RatString() != tc.wantRat || scale != tc.wantScale {
				t.Fatalf("got %s scale %d, want %s scale %d", r.RatString(), scale, tc.wantRat, tc.wantScale)
			}
		})
	}
}

func TestCompareDecimals(t *testing.T) {
	tests := []struct {
		a, b any
		want int
	}{
		{"12.50", "12.5", 0},
		{"9", "10", -1},
		{"0.3", 0.3, 0},
		{"-1", "-0.5", -1},
		{"100.001", 100, 1},
	}
	for _, tc := range tests {
		got, ok := CompareDecimals(tc.a, tc.b)
		if !ok || got != tc.want {
			t.Errorf("CompareDecimals(%v, %v) = %d, %v; want %d", tc.a, tc.b, got, ok, tc.want)
		}
	}
	if _, ok := CompareDecimals("x", "1"); ok {
		t.Error("CompareDecimals accepted a non-decimal")
	}
}

func TestParse_DecimalOptions(t *testing.T) {
	tests := []struct {
		name    string
		props   string
		wantErr string // empty: loads
	}{
		{"plain decimal", "        type: decimal\n", ""},
		{"range and scale", "        type: decimal\n        min: 0\n        max: 100\n        scale: 2\n", ""},
		{"scale zero", "        type: decimal\n        scale: 0\n", ""},
		{"decimal list", "        type: decimal\n        list: true\n        scale: 2\n", ""},
		{"currency", "        type: decimal\n        display:\n          currency: EUR\n", ""},
		{"scale negative", "        type: decimal\n        scale: -1\n", "must be from 0 to 30"},
		{"scale too large", "        type: decimal\n        scale: 31\n", "must be from 0 to 30"},
		{"scale on integer", "        type: integer\n        scale: 2\n", `scale applies to type "decimal" only`},
		{"min above max", "        type: decimal\n        min: 5\n        max: 1\n", "min 5 above max 1"},
		{"range on list", "        type: decimal\n        list: true\n        min: 1\n", "not supported on a list"},
		{"unique refused", "        type: decimal\n        unique: true\n", "unique is only supported on string-valued"},
		{"float suggests decimal", "        type: float\n", `use "integer" or "decimal" instead`},
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

func TestValidatePropertyValue_Decimal(t *testing.T) {
	v := NewValidator(&Metamodel{})
	plain := &PropertyDef{Type: PropertyTypeDecimal}
	priced := &PropertyDef{Type: PropertyTypeDecimal, Min: new(0), Max: new(100), Scale: new(2)}
	list := &PropertyDef{Type: PropertyTypeDecimal, List: true, Scale: new(1)}
	tests := []struct {
		name     string
		def      *PropertyDef
		val      any
		wantErr  string // empty: valid
		wantType ValidationErrorType
	}{
		{"string", plain, "12.50", "", ""},
		{"int", plain, 3, "", ""},
		{"float", plain, 2.25, "", ""},
		{"exponent", plain, "1e3", "Invalid decimal", ValidationErrorInvalidValue},
		{"letters", plain, "abc", "Invalid decimal", ValidationErrorInvalidValue},
		{"bool", plain, true, "Must be a decimal number", ValidationErrorInvalidType},
		{"in range", priced, "99.99", "", ""},
		{"upper bound inclusive", priced, "100.00", "", ""},
		{"above max", priced, "100.01", "Out of range (must be from 0 to 100)", ValidationErrorInvalidValue},
		{"below min", priced, "-0.01", "Out of range", ValidationErrorInvalidValue},
		{"scale exceeded", priced, "1.005", "Too many digits after the point (at most 2 allowed)", ValidationErrorInvalidValue},
		{"scale with trailing zero counts", priced, "1.500", "digits after the point", ValidationErrorInvalidValue},
		{"list elements", list, []any{"1.5", "2"}, "", ""},
		{"list element over scale", list, []any{"1.5", "2.25"}, "digits after the point", ValidationErrorInvalidValue},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.validatePropertyValue("price", tc.def, tc.val)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want valid, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Message, tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
			if err.Type != tc.wantType {
				t.Errorf("type = %s, want %s", err.Type, tc.wantType)
			}
		})
	}
}

func TestResolveWidgetFromType_Decimal(t *testing.T) {
	if got := ResolveWidgetFromType(nil, PropertyTypeDecimal); got != "number" {
		t.Fatalf("widget = %q, want number", got)
	}
}
