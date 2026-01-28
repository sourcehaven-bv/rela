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

func TestNormalizeIDType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty defaults to auto", input: "", want: IDTypeAuto},
		{name: "auto returns auto", input: "auto", want: IDTypeAuto},
		{name: "manual returns manual", input: "manual", want: IDTypeManual},
		{name: "deprecated sequential normalizes to auto", input: "sequential", want: IDTypeAuto},
		{name: "deprecated string normalizes to manual", input: "string", want: IDTypeManual},
		{name: "invalid returns as-is", input: "invalid", want: "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeIDType(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeIDType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidIDType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "empty is valid", input: "", want: true},
		{name: "auto is valid", input: "auto", want: true},
		{name: "manual is valid", input: "manual", want: true},
		{name: "deprecated sequential is valid", input: "sequential", want: true},
		{name: "deprecated string is valid", input: "string", want: true},
		{name: "invalid is not valid", input: "invalid", want: false},
		{name: "uuid is not valid", input: "uuid", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidIDType(tt.input)
			if got != tt.want {
				t.Errorf("IsValidIDType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsDeprecatedIDType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "empty is not deprecated", input: "", want: false},
		{name: "auto is not deprecated", input: "auto", want: false},
		{name: "manual is not deprecated", input: "manual", want: false},
		{name: "sequential is deprecated", input: "sequential", want: true},
		{name: "string is deprecated", input: "string", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDeprecatedIDType(tt.input)
			if got != tt.want {
				t.Errorf("IsDeprecatedIDType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestEntityDef_GetIDType(t *testing.T) {
	tests := []struct {
		name string
		def  EntityDef
		want string
	}{
		{
			name: "empty defaults to auto",
			def:  EntityDef{},
			want: IDTypeAuto,
		},
		{
			name: "explicit auto",
			def:  EntityDef{IDType: IDTypeAuto},
			want: IDTypeAuto,
		},
		{
			name: "explicit manual",
			def:  EntityDef{IDType: IDTypeManual},
			want: IDTypeManual,
		},
		{
			name: "deprecated sequential normalizes to auto",
			def:  EntityDef{IDType: IDTypeSequential},
			want: IDTypeAuto,
		},
		{
			name: "deprecated string normalizes to manual",
			def:  EntityDef{IDType: IDTypeString},
			want: IDTypeManual,
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

func TestEntityDef_IsAutoID(t *testing.T) {
	tests := []struct {
		name string
		def  EntityDef
		want bool
	}{
		{name: "empty is auto", def: EntityDef{}, want: true},
		{name: "explicit auto", def: EntityDef{IDType: IDTypeAuto}, want: true},
		{name: "manual is not auto", def: EntityDef{IDType: IDTypeManual}, want: false},
		{name: "deprecated sequential is auto", def: EntityDef{IDType: IDTypeSequential}, want: true},
		{name: "deprecated string is not auto", def: EntityDef{IDType: IDTypeString}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.def.IsAutoID()
			if got != tt.want {
				t.Errorf("IsAutoID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntityDef_IsManualID(t *testing.T) {
	tests := []struct {
		name string
		def  EntityDef
		want bool
	}{
		{name: "empty is not manual", def: EntityDef{}, want: false},
		{name: "auto is not manual", def: EntityDef{IDType: IDTypeAuto}, want: false},
		{name: "explicit manual", def: EntityDef{IDType: IDTypeManual}, want: true},
		{name: "deprecated sequential is not manual", def: EntityDef{IDType: IDTypeSequential}, want: false},
		{name: "deprecated string is manual", def: EntityDef{IDType: IDTypeString}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.def.IsManualID()
			if got != tt.want {
				t.Errorf("IsManualID() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test that deprecated methods still work correctly (they delegate to new methods)
func TestEntityDef_IsSequentialID(t *testing.T) {
	tests := []struct {
		name string
		def  EntityDef
		want bool
	}{
		{name: "empty is sequential", def: EntityDef{}, want: true},
		{name: "auto is sequential", def: EntityDef{IDType: IDTypeAuto}, want: true},
		{name: "manual is not sequential", def: EntityDef{IDType: IDTypeManual}, want: false},
		{name: "deprecated sequential is sequential", def: EntityDef{IDType: IDTypeSequential}, want: true},
		{name: "deprecated string is not sequential", def: EntityDef{IDType: IDTypeString}, want: false},
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
		{name: "empty is not string", def: EntityDef{}, want: false},
		{name: "auto is not string", def: EntityDef{IDType: IDTypeAuto}, want: false},
		{name: "manual is string", def: EntityDef{IDType: IDTypeManual}, want: true},
		{name: "deprecated sequential is not string", def: EntityDef{IDType: IDTypeSequential}, want: false},
		{name: "deprecated string is string", def: EntityDef{IDType: IDTypeString}, want: true},
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
			name: "valid auto id_type",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_type: auto
    id_patterns: ["REQ-"]
`,
			wantErr: false,
		},
		{
			name: "valid manual id_type",
			yaml: `
version: "1.0"
entities:
  component:
    label: Component
    id_type: manual
`,
			wantErr: false,
		},
		{
			name: "deprecated sequential id_type still valid",
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
			name: "deprecated string id_type still valid",
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
			name: "empty id_type is valid (defaults to auto)",
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
