package metamodel

import (
	"errors"
	"sort"
	"testing"
)

func TestEntityDef_GetDirPlural(t *testing.T) {
	tests := []struct {
		name     string
		def      EntityDef
		typeName string
		want     string
	}{
		{
			name:     "with explicit plural",
			def:      EntityDef{Plural: "policies"},
			typeName: "policy",
			want:     "policies",
		},
		{
			name:     "without plural falls back to naive",
			def:      EntityDef{},
			typeName: "requirement",
			want:     "requirements",
		},
		{
			name:     "nonconformity without plural",
			def:      EntityDef{},
			typeName: "nonconformity",
			want:     "nonconformitys",
		},
		{
			name:     "nonconformity with proper plural",
			def:      EntityDef{Plural: "nonconformities"},
			typeName: "nonconformity",
			want:     "nonconformities",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.def.GetDirPlural(tt.typeName)
			if got != tt.want {
				t.Errorf("GetDirPlural() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEntityDef_GetDefaultStatus(t *testing.T) {
	tests := []struct {
		name string
		def  EntityDef
		meta *Metamodel
		want string
	}{
		{
			name: "no status property uses draft",
			def:  EntityDef{Properties: map[string]PropertyDef{}},
			meta: &Metamodel{},
			want: "draft",
		},
		{
			name: "standard status type uses draft",
			def: EntityDef{
				Properties: map[string]PropertyDef{
					"status": {Type: "status"},
				},
			},
			meta: &Metamodel{},
			want: "draft",
		},
		{
			name: "explicit default in property",
			def: EntityDef{
				Properties: map[string]PropertyDef{
					"status": {Type: "status", Default: "proposed"},
				},
			},
			meta: &Metamodel{},
			want: "proposed",
		},
		{
			name: "inline enum values uses first value",
			def: EntityDef{
				Properties: map[string]PropertyDef{
					"status": {Type: "enum", Values: []string{"open", "closed", "resolved"}},
				},
			},
			meta: &Metamodel{},
			want: "open",
		},
		{
			name: "custom type uses its default",
			def: EntityDef{
				Properties: map[string]PropertyDef{
					"status": {Type: "nc_status"},
				},
			},
			meta: &Metamodel{
				Types: map[string]CustomType{
					"nc_status": {
						Values:  []string{"open", "investigating", "correcting", "closed"},
						Default: "open",
					},
				},
			},
			want: "open",
		},
		{
			name: "custom type without default uses first value",
			def: EntityDef{
				Properties: map[string]PropertyDef{
					"status": {Type: "issue_status"},
				},
			},
			meta: &Metamodel{
				Types: map[string]CustomType{
					"issue_status": {
						Values: []string{"new", "triaged", "fixed", "wontfix"},
					},
				},
			},
			want: "new",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.def.GetDefaultStatus(tt.meta)
			if got != tt.want {
				t.Errorf("GetDefaultStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEntityDef_GetPrimaryProperty(t *testing.T) {
	tests := []struct {
		name string
		def  EntityDef
		want string
	}{
		{
			name: "title is primary when required",
			def: EntityDef{
				Properties: map[string]PropertyDef{
					"title":       {Type: "string", Required: true},
					"description": {Type: "string"},
				},
			},
			want: "title",
		},
		{
			name: "name is primary when required and no title",
			def: EntityDef{
				Properties: map[string]PropertyDef{
					"name":        {Type: "string", Required: true},
					"description": {Type: "string"},
				},
			},
			want: "name",
		},
		{
			name: "title takes priority over name when both required",
			def: EntityDef{
				Properties: map[string]PropertyDef{
					"title": {Type: "string", Required: true},
					"name":  {Type: "string", Required: true},
				},
			},
			want: "title",
		},
		{
			name: "label takes priority over other properties",
			def: EntityDef{
				Properties: map[string]PropertyDef{
					"label":      {Type: "string", Required: true},
					"identifier": {Type: "string", Required: true},
				},
			},
			want: "label",
		},
		{
			name: "falls back to any required string property",
			def: EntityDef{
				Properties: map[string]PropertyDef{
					"identifier": {Type: "string", Required: true},
					"status":     {Type: "status", Required: true},
				},
			},
			want: "identifier",
		},
		{
			name: "returns empty if no required string properties",
			def: EntityDef{
				Properties: map[string]PropertyDef{
					"status":   {Type: "status", Required: true},
					"priority": {Type: "priority"},
				},
			},
			want: "",
		},
		{
			name: "empty type treated as string",
			def: EntityDef{
				Properties: map[string]PropertyDef{
					"title": {Required: true}, // Type defaults to string
				},
			},
			want: "title",
		},
		{
			name: "non-required title is not primary",
			def: EntityDef{
				Properties: map[string]PropertyDef{
					"title":       {Type: "string", Required: false},
					"description": {Type: "string", Required: true},
				},
			},
			want: "description",
		},
		{
			name: "empty properties returns empty",
			def:  EntityDef{Properties: map[string]PropertyDef{}},
			want: "",
		},
		{
			name: "nil properties returns empty",
			def:  EntityDef{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.def.GetPrimaryProperty()
			if got != tt.want {
				t.Errorf("GetPrimaryProperty() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetPrimaryPropertyDeterministic(t *testing.T) {
	// When multiple non-priority required string properties exist,
	// the result should be deterministic (we expect some result)
	def := EntityDef{
		Properties: map[string]PropertyDef{
			"foo": {Type: "string", Required: true},
			"bar": {Type: "string", Required: true},
			"baz": {Type: "string", Required: true},
		},
	}

	// Run multiple times to check consistency
	results := make([]string, 10)
	for i := 0; i < 10; i++ {
		results[i] = def.GetPrimaryProperty()
	}

	// All results should be the same (deterministic)
	first := results[0]
	for _, r := range results[1:] {
		if r != first {
			t.Errorf("GetPrimaryProperty() not deterministic: got %q and %q", first, r)
		}
	}

	// The result should be one of the valid properties
	validProps := []string{"foo", "bar", "baz"}
	sort.Strings(validProps)
	found := false
	for _, v := range validProps {
		if first == v {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("GetPrimaryProperty() = %q, expected one of %v", first, validProps)
	}
}

func TestEntityDef_GetIDType(t *testing.T) {
	tests := []struct {
		name string
		def  EntityDef
		want string
	}{
		{
			name: "empty defaults to sequential",
			def:  EntityDef{},
			want: IDTypeSequential,
		},
		{
			name: "explicit sequential",
			def:  EntityDef{IDType: IDTypeSequential},
			want: IDTypeSequential,
		},
		{
			name: "explicit string",
			def:  EntityDef{IDType: IDTypeString},
			want: IDTypeString,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.def.GetIDType()
			if got != tt.want {
				t.Errorf("GetIDType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEntityDef_IsSequentialID(t *testing.T) {
	tests := []struct {
		name string
		def  EntityDef
		want bool
	}{
		{
			name: "empty is sequential",
			def:  EntityDef{},
			want: true,
		},
		{
			name: "explicit sequential",
			def:  EntityDef{IDType: IDTypeSequential},
			want: true,
		},
		{
			name: "string is not sequential",
			def:  EntityDef{IDType: IDTypeString},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.def.IsSequentialID()
			if got != tt.want {
				t.Errorf("IsSequentialID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntityDef_IsStringID(t *testing.T) {
	tests := []struct {
		name string
		def  EntityDef
		want bool
	}{
		{
			name: "empty is not string",
			def:  EntityDef{},
			want: false,
		},
		{
			name: "sequential is not string",
			def:  EntityDef{IDType: IDTypeSequential},
			want: false,
		},
		{
			name: "string is string",
			def:  EntityDef{IDType: IDTypeString},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.def.IsStringID()
			if got != tt.want {
				t.Errorf("IsStringID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParse_IDTypeValidation(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		errType error
	}{
		{
			name: "valid sequential id_type",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_type: sequential
    id_patterns: ["REQ-"]
`,
			wantErr: false,
		},
		{
			name: "valid string id_type",
			yaml: `
version: "1.0"
entities:
  component:
    label: Component
    id_type: string
`,
			wantErr: false,
		},
		{
			name: "empty id_type is valid (defaults to sequential)",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_patterns: ["REQ-"]
`,
			wantErr: false,
		},
		{
			name: "invalid id_type",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_type: invalid
    id_patterns: ["REQ-"]
`,
			wantErr: true,
			errType: &InvalidIDTypeError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if !tt.wantErr {
				if err != nil {
					t.Errorf("Parse() unexpected error: %v", err)
				}
				return
			}
			// wantErr is true
			if err == nil {
				t.Errorf("Parse() expected error, got nil")
				return
			}
			if tt.errType == nil {
				return
			}
			var idTypeErr *InvalidIDTypeError
			if !errors.As(err, &idTypeErr) {
				t.Errorf("Parse() error type = %T, want %T", err, tt.errType)
			}
		})
	}
}

func TestParseCardinality(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMin *int
		wantMax *int
	}{
		{name: "empty", input: "", wantMin: nil, wantMax: nil},
		{name: "star", input: "*", wantMin: intPtr(0), wantMax: nil},
		{name: "single 1", input: "1", wantMin: intPtr(1), wantMax: intPtr(1)},
		{name: "single 0", input: "0", wantMin: intPtr(0), wantMax: intPtr(0)},
		{name: "range 0..1", input: "0..1", wantMin: intPtr(0), wantMax: intPtr(1)},
		{name: "range 1..*", input: "1..*", wantMin: intPtr(1), wantMax: nil},
		{name: "range 0..*", input: "0..*", wantMin: intPtr(0), wantMax: nil},
		{name: "range 2..5", input: "2..5", wantMin: intPtr(2), wantMax: intPtr(5)},
		{name: "invalid", input: "abc", wantMin: nil, wantMax: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMin, gotMax := ParseCardinality(tt.input)
			if !intPtrEqual(gotMin, tt.wantMin) {
				t.Errorf("ParseCardinality(%q) min = %v, want %v", tt.input, intPtrVal(gotMin), intPtrVal(tt.wantMin))
			}
			if !intPtrEqual(gotMax, tt.wantMax) {
				t.Errorf("ParseCardinality(%q) max = %v, want %v", tt.input, intPtrVal(gotMax), intPtrVal(tt.wantMax))
			}
		})
	}
}

func TestFormatCardinality(t *testing.T) {
	tests := []struct {
		name string
		min  *int
		max  *int
		want string
	}{
		{name: "nil nil", min: nil, max: nil, want: ""},
		{name: "0 nil", min: intPtr(0), max: nil, want: "*"},
		{name: "1 nil", min: intPtr(1), max: nil, want: "1..*"},
		{name: "1 1", min: intPtr(1), max: intPtr(1), want: "1"},
		{name: "0 1", min: intPtr(0), max: intPtr(1), want: "0..1"},
		{name: "2 5", min: intPtr(2), max: intPtr(5), want: "2..5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatCardinality(tt.min, tt.max)
			if got != tt.want {
				t.Errorf("FormatCardinality(%v, %v) = %q, want %q",
					intPtrVal(tt.min), intPtrVal(tt.max), got, tt.want)
			}
		})
	}
}

func TestValidateCardinalityNotation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty", input: "", wantErr: false},
		{name: "star", input: "*", wantErr: false},
		{name: "single digit", input: "1", wantErr: false},
		{name: "range 0..1", input: "0..1", wantErr: false},
		{name: "range 1..*", input: "1..*", wantErr: false},
		{name: "range 0..*", input: "0..*", wantErr: false},
		{name: "range 2..5", input: "2..5", wantErr: false},
		{name: "invalid text", input: "many", wantErr: true},
		{name: "invalid min", input: "a..5", wantErr: true},
		{name: "invalid max", input: "1..b", wantErr: true},
		{name: "min > max", input: "5..2", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCardinalityNotation(tt.input)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateCardinalityNotation(%q) expected error, got nil", tt.input)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateCardinalityNotation(%q) unexpected error: %v", tt.input, err)
			}
		})
	}
}

func TestRelationDef_GetFromToCardinality(t *testing.T) {
	// Test new cardinality syntax takes precedence
	rel := RelationDef{
		Cardinality: &CardinalityDef{
			From: "1..*",
			To:   "0..1",
		},
		SourceMin: intPtr(99), // Should be ignored
		TargetMax: intPtr(99), // Should be ignored
	}

	// GetFromMin should return 1 (from "1..*")
	if fromMin := rel.GetFromMin(); fromMin == nil || *fromMin != 1 {
		t.Errorf("GetFromMin() = %v, want 1", intPtrVal(fromMin))
	}

	// GetFromMax should return nil (unbounded from "1..*")
	if fromMax := rel.GetFromMax(); fromMax != nil {
		t.Errorf("GetFromMax() = %v, want nil", *fromMax)
	}

	// GetToMin should return 0 (from "0..1")
	if toMin := rel.GetToMin(); toMin == nil || *toMin != 0 {
		t.Errorf("GetToMin() = %v, want 0", intPtrVal(toMin))
	}

	// GetToMax should return 1 (from "0..1")
	if toMax := rel.GetToMax(); toMax == nil || *toMax != 1 {
		t.Errorf("GetToMax() = %v, want 1", intPtrVal(toMax))
	}
}

func TestRelationDef_GetFromToCardinality_Fallback(t *testing.T) {
	// Test fallback to deprecated fields
	rel := RelationDef{
		SourceMin: intPtr(1),
		SourceMax: intPtr(5),
		TargetMin: intPtr(0),
		TargetMax: intPtr(1),
	}

	if fromMin := rel.GetFromMin(); fromMin == nil || *fromMin != 1 {
		t.Errorf("GetFromMin() = %v, want 1", intPtrVal(fromMin))
	}
	if fromMax := rel.GetFromMax(); fromMax == nil || *fromMax != 5 {
		t.Errorf("GetFromMax() = %v, want 5", intPtrVal(fromMax))
	}
	if toMin := rel.GetToMin(); toMin == nil || *toMin != 0 {
		t.Errorf("GetToMin() = %v, want 0", intPtrVal(toMin))
	}
	if toMax := rel.GetToMax(); toMax == nil || *toMax != 1 {
		t.Errorf("GetToMax() = %v, want 1", intPtrVal(toMax))
	}
}

func TestParse_CardinalityValidation(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		errType string // "conflict" or "invalid"
	}{
		{
			name: "valid new cardinality",
			yaml: `
version: "1.0"
relations:
  addresses:
    label: addresses
    from: [decision]
    to: [requirement]
    cardinality:
      from: "1..*"
      to: "0..1"
`,
			wantErr: false,
		},
		{
			name: "valid deprecated cardinality",
			yaml: `
version: "1.0"
relations:
  addresses:
    label: addresses
    from: [decision]
    to: [requirement]
    source_min: 1
    target_max: 1
`,
			wantErr: false,
		},
		{
			name: "conflict: both styles",
			yaml: `
version: "1.0"
relations:
  addresses:
    label: addresses
    from: [decision]
    to: [requirement]
    cardinality:
      from: "1..*"
    source_min: 1
`,
			wantErr: true,
			errType: "conflict",
		},
		{
			name: "invalid from notation",
			yaml: `
version: "1.0"
relations:
  addresses:
    label: addresses
    from: [decision]
    to: [requirement]
    cardinality:
      from: "invalid"
`,
			wantErr: true,
			errType: "invalid",
		},
		{
			name: "invalid to notation",
			yaml: `
version: "1.0"
relations:
  addresses:
    label: addresses
    from: [decision]
    to: [requirement]
    cardinality:
      to: "5..2"
`,
			wantErr: true,
			errType: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if !tt.wantErr {
				if err != nil {
					t.Errorf("Parse() unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("Parse() expected error, got nil")
				return
			}

			switch tt.errType {
			case "conflict":
				var conflictErr *ConflictingCardinalityError
				if !errors.As(err, &conflictErr) {
					t.Errorf("Parse() error type = %T, want *ConflictingCardinalityError", err)
				}
			case "invalid":
				var invalidErr *InvalidCardinalityError
				if !errors.As(err, &invalidErr) {
					t.Errorf("Parse() error type = %T, want *InvalidCardinalityError", err)
				}
			}
		})
	}
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func intPtrEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func intPtrVal(p *int) string {
	if p == nil {
		return "nil"
	}
	return intToString(*p)
}
