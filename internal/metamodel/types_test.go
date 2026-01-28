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
    id_prefixes: ["REQ-"]
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
			name: "empty id_type is valid (defaults to auto)",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefixes: ["REQ-"]
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
    id_prefixes: ["REQ-"]
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

func TestIsBuiltinType(t *testing.T) {
	tests := []struct {
		name string
		typ  string
		want bool
	}{
		{name: "string is builtin", typ: PropertyTypeString, want: true},
		{name: "date is builtin", typ: PropertyTypeDate, want: true},
		{name: "integer is builtin", typ: PropertyTypeInteger, want: true},
		{name: "boolean is builtin", typ: PropertyTypeBoolean, want: true},
		{name: "enum is builtin", typ: PropertyTypeEnum, want: true},
		{name: "custom type is not builtin", typ: "priority", want: false},
		{name: "empty is not builtin", typ: "", want: false},
		{name: "unknown is not builtin", typ: "unknown", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBuiltinType(tt.typ)
			if got != tt.want {
				t.Errorf("IsBuiltinType(%q) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
}

func TestPropertyDef_GetDateFormat(t *testing.T) {
	tests := []struct {
		name string
		prop PropertyDef
		want string
	}{
		{
			name: "default format",
			prop: PropertyDef{Type: PropertyTypeDate},
			want: DefaultDateFormat,
		},
		{
			name: "custom format",
			prop: PropertyDef{Type: PropertyTypeDate, Format: "2006-01-02 15:04:05"},
			want: "2006-01-02 15:04:05",
		},
		{
			name: "empty format uses default",
			prop: PropertyDef{Type: PropertyTypeDate, Format: ""},
			want: DefaultDateFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.prop.GetDateFormat()
			if got != tt.want {
				t.Errorf("GetDateFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEntityDef_GetPlural(t *testing.T) {
	tests := []struct {
		name string
		def  EntityDef
		want string
	}{
		{
			name: "explicit label_plural",
			def:  EntityDef{Label: "Policy", LabelPlural: "Policies"},
			want: "Policies",
		},
		{
			name: "no label_plural uses label + s",
			def:  EntityDef{Label: "Requirement"},
			want: "Requirements",
		},
		{
			name: "empty label",
			def:  EntityDef{Label: ""},
			want: "s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.def.GetPlural()
			if got != tt.want {
				t.Errorf("GetPlural() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEntityDef_HasPattern(t *testing.T) {
	tests := []struct {
		name    string
		def     EntityDef
		pattern string
		want    bool
	}{
		{
			name:    "matches single prefix",
			def:     EntityDef{IDPrefix: "REQ-"},
			pattern: "REQ-",
			want:    true,
		},
		{
			name:    "no match single prefix",
			def:     EntityDef{IDPrefix: "REQ-"},
			pattern: "DES-",
			want:    false,
		},
		{
			name:    "matches one of multiple prefixes",
			def:     EntityDef{IDPrefixes: []string{"REQ-", "RQ-"}},
			pattern: "RQ-",
			want:    true,
		},
		{
			name:    "no match multiple prefixes",
			def:     EntityDef{IDPrefixes: []string{"REQ-", "RQ-"}},
			pattern: "DES-",
			want:    false,
		},
		{
			name:    "no prefixes",
			def:     EntityDef{},
			pattern: "REQ-",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.def.HasPattern(tt.pattern)
			if got != tt.want {
				t.Errorf("HasPattern(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestEntityDef_MatchesID(t *testing.T) {
	tests := []struct {
		name string
		def  EntityDef
		id   string
		want bool
	}{
		{
			name: "matches single prefix",
			def:  EntityDef{IDPrefix: "REQ-"},
			id:   "REQ-001",
			want: true,
		},
		{
			name: "no match single prefix",
			def:  EntityDef{IDPrefix: "REQ-"},
			id:   "DES-001",
			want: false,
		},
		{
			name: "matches one of multiple prefixes",
			def:  EntityDef{IDPrefixes: []string{"REQ-", "RQ-"}},
			id:   "RQ-42",
			want: true,
		},
		{
			name: "no match multiple prefixes",
			def:  EntityDef{IDPrefixes: []string{"REQ-", "RQ-"}},
			id:   "DES-001",
			want: false,
		},
		{
			name: "no prefixes",
			def:  EntityDef{},
			id:   "REQ-001",
			want: false,
		},
		{
			name: "id shorter than prefix",
			def:  EntityDef{IDPrefix: "REQ-"},
			id:   "RE",
			want: false,
		},
		{
			name: "exact prefix match",
			def:  EntityDef{IDPrefix: "REQ-"},
			id:   "REQ-",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.def.MatchesID(tt.id)
			if got != tt.want {
				t.Errorf("MatchesID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestMetamodel_InferEntityType(t *testing.T) {
	meta := &Metamodel{
		Entities: map[string]EntityDef{
			"requirement": {IDPrefix: "REQ-"},
			"design":      {IDPrefix: "DES-"},
			"component":   {IDPrefixes: []string{"COMP-", "C-"}},
		},
	}

	tests := []struct {
		name string
		id   string
		want string
	}{
		{name: "infers requirement", id: "REQ-001", want: "requirement"},
		{name: "infers design", id: "DES-042", want: "design"},
		{name: "infers component from first prefix", id: "COMP-alpha", want: "component"},
		{name: "infers component from second prefix", id: "C-beta", want: "component"},
		{name: "no match returns empty", id: "UNKNOWN-123", want: ""},
		{name: "empty id returns empty", id: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := meta.InferEntityType(tt.id)
			if got != tt.want {
				t.Errorf("InferEntityType(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestMetamodel_EntityTypes(t *testing.T) {
	meta := &Metamodel{
		Entities: map[string]EntityDef{
			"requirement": {Label: "Requirement"},
			"design":      {Label: "Design"},
			"component":   {Label: "Component"},
		},
	}

	got := meta.EntityTypes()
	if len(got) != 3 {
		t.Errorf("EntityTypes() returned %d types, want 3", len(got))
	}

	// Check all expected types are present
	expected := map[string]bool{"requirement": true, "design": true, "component": true}
	for _, typ := range got {
		if !expected[typ] {
			t.Errorf("EntityTypes() contains unexpected type %q", typ)
		}
		delete(expected, typ)
	}
	if len(expected) > 0 {
		t.Errorf("EntityTypes() missing types: %v", expected)
	}
}

func TestMetamodel_RelationTypes(t *testing.T) {
	meta := &Metamodel{
		Relations: map[string]RelationDef{
			"implements":  {Label: "implements"},
			"dependsOn":   {Label: "depends on"},
			"allocatedTo": {Label: "allocated to"},
		},
	}

	got := meta.RelationTypes()
	if len(got) != 3 {
		t.Errorf("RelationTypes() returned %d types, want 3", len(got))
	}

	// Check all expected types are present
	expected := map[string]bool{"implements": true, "dependsOn": true, "allocatedTo": true}
	for _, typ := range got {
		if !expected[typ] {
			t.Errorf("RelationTypes() contains unexpected type %q", typ)
		}
		delete(expected, typ)
	}
	if len(expected) > 0 {
		t.Errorf("RelationTypes() missing types: %v", expected)
	}
}

func TestValidationRule_GetSeverity(t *testing.T) {
	tests := []struct {
		name string
		rule ValidationRule
		want string
	}{
		{
			name: "explicit error severity",
			rule: ValidationRule{Severity: "error"},
			want: "error",
		},
		{
			name: "explicit warning severity",
			rule: ValidationRule{Severity: "warning"},
			want: "warning",
		},
		{
			name: "empty defaults to warning",
			rule: ValidationRule{},
			want: "warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rule.GetSeverity()
			if got != tt.want {
				t.Errorf("GetSeverity() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidationRule_IsError(t *testing.T) {
	tests := []struct {
		name string
		rule ValidationRule
		want bool
	}{
		{name: "error severity is error", rule: ValidationRule{Severity: "error"}, want: true},
		{name: "warning severity is not error", rule: ValidationRule{Severity: "warning"}, want: false},
		{name: "empty defaults to warning is not error", rule: ValidationRule{}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rule.IsError()
			if got != tt.want {
				t.Errorf("IsError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInverseDef_GetID(t *testing.T) {
	tests := []struct {
		name string
		inv  InverseDef
		want string
	}{
		{name: "with ID", inv: InverseDef{ID: "addressedBy"}, want: "addressedBy"},
		{name: "empty ID", inv: InverseDef{}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.inv.GetID()
			if got != tt.want {
				t.Errorf("GetID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInverseDef_GetLabel(t *testing.T) {
	tests := []struct {
		name string
		inv  InverseDef
		want string
	}{
		{
			name: "explicit label",
			inv:  InverseDef{ID: "addressedBy", Label: "is addressed by"},
			want: "is addressed by",
		},
		{
			name: "auto-derived from camelCase",
			inv:  InverseDef{ID: "addressedBy"},
			want: "addressed by",
		},
		{
			name: "auto-derived from PascalCase",
			inv:  InverseDef{ID: "ImplementedBy"},
			want: "implemented by",
		},
		{
			name: "empty ID returns empty",
			inv:  InverseDef{},
			want: "",
		},
		{
			name: "single word",
			inv:  InverseDef{ID: "inverse"},
			want: "inverse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.inv.GetLabel()
			if got != tt.want {
				t.Errorf("GetLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMetamodel_GetterMethods(t *testing.T) {
	meta := &Metamodel{
		Version:   "1.0",
		Namespace: "test",
		Types: map[string]CustomType{
			"priority": {Values: []string{"high", "medium", "low"}},
		},
		Entities: map[string]EntityDef{
			"requirement": {Label: "Requirement"},
		},
		Relations: map[string]RelationDef{
			"implements": {Label: "implements"},
		},
	}

	t.Run("GetVersion", func(t *testing.T) {
		if got := meta.GetVersion(); got != "1.0" {
			t.Errorf("GetVersion() = %q, want %q", got, "1.0")
		}
	})

	t.Run("GetNamespace", func(t *testing.T) {
		if got := meta.GetNamespace(); got != "test" {
			t.Errorf("GetNamespace() = %q, want %q", got, "test")
		}
	})

	t.Run("GetEntities", func(t *testing.T) {
		got := meta.GetEntities()
		if got == nil {
			t.Error("GetEntities() returned nil")
		}
	})

	t.Run("GetRelations", func(t *testing.T) {
		got := meta.GetRelations()
		if got == nil {
			t.Error("GetRelations() returned nil")
		}
	})

	t.Run("GetTypes", func(t *testing.T) {
		got := meta.GetTypes()
		if got == nil {
			t.Error("GetTypes() returned nil")
		}
	})
}

func TestEntityDef_GetterMethods(t *testing.T) {
	def := EntityDef{
		Label:       "Requirement",
		Aliases:     []string{"req", "r"},
		RDFType:     "rela:Requirement",
		Color:       "#FF0000",
		BorderColor: "#AA0000",
		Properties: map[string]PropertyDef{
			"title": {Type: "string", Required: true},
		},
	}

	t.Run("GetLabel", func(t *testing.T) {
		if got := def.GetLabel(); got != "Requirement" {
			t.Errorf("GetLabel() = %q, want %q", got, "Requirement")
		}
	})

	t.Run("GetAliases", func(t *testing.T) {
		got := def.GetAliases()
		if len(got) != 2 {
			t.Errorf("GetAliases() returned %d aliases, want 2", len(got))
		}
	})

	t.Run("GetIDPatterns", func(t *testing.T) {
		defWithPrefixes := EntityDef{IDPrefixes: []string{"REQ-", "R-"}}
		got := defWithPrefixes.GetIDPatterns()
		if len(got) != 2 {
			t.Errorf("GetIDPatterns() returned %d patterns, want 2", len(got))
		}
	})

	t.Run("GetProperties", func(t *testing.T) {
		got := def.GetProperties()
		if got == nil {
			t.Error("GetProperties() returned nil")
		}
	})

	t.Run("GetRDFType", func(t *testing.T) {
		if got := def.GetRDFType(); got != "rela:Requirement" {
			t.Errorf("GetRDFType() = %q, want %q", got, "rela:Requirement")
		}
	})

	t.Run("GetColor", func(t *testing.T) {
		if got := def.GetColor(); got != "#FF0000" {
			t.Errorf("GetColor() = %q, want %q", got, "#FF0000")
		}
	})

	t.Run("GetBorderColor", func(t *testing.T) {
		if got := def.GetBorderColor(); got != "#AA0000" {
			t.Errorf("GetBorderColor() = %q, want %q", got, "#AA0000")
		}
	})
}

func TestRelationDef_GetterMethods(t *testing.T) {
	minOne := 1
	maxFive := 5
	rel := RelationDef{
		Label:       "implements",
		Description: "Implementation relationship",
		From:        []string{"design"},
		To:          []string{"requirement"},
		Symmetric:   false,
		SourceMin:   &minOne,
		SourceMax:   &maxFive,
		TargetMin:   &minOne,
		TargetMax:   &maxFive,
		Inverse:     &InverseDef{ID: "implementedBy"},
	}

	t.Run("GetLabel", func(t *testing.T) {
		if got := rel.GetLabel(); got != "implements" {
			t.Errorf("GetLabel() = %q, want %q", got, "implements")
		}
	})

	t.Run("GetDescription", func(t *testing.T) {
		if got := rel.GetDescription(); got != "Implementation relationship" {
			t.Errorf("GetDescription() = %q, want %q", got, "Implementation relationship")
		}
	})

	t.Run("GetFrom", func(t *testing.T) {
		got := rel.GetFrom()
		if len(got) != 1 || got[0] != "design" {
			t.Errorf("GetFrom() = %v, want [design]", got)
		}
	})

	t.Run("GetTo", func(t *testing.T) {
		got := rel.GetTo()
		if len(got) != 1 || got[0] != "requirement" {
			t.Errorf("GetTo() = %v, want [requirement]", got)
		}
	})

	t.Run("IsSymmetric", func(t *testing.T) {
		if got := rel.IsSymmetric(); got != false {
			t.Errorf("IsSymmetric() = %v, want false", got)
		}
	})

	t.Run("GetSourceMin", func(t *testing.T) {
		got := rel.GetSourceMin()
		if got == nil || *got != 1 {
			t.Errorf("GetSourceMin() = %v, want 1", got)
		}
	})

	t.Run("GetSourceMax", func(t *testing.T) {
		got := rel.GetSourceMax()
		if got == nil || *got != 5 {
			t.Errorf("GetSourceMax() = %v, want 5", got)
		}
	})

	t.Run("GetTargetMin", func(t *testing.T) {
		got := rel.GetTargetMin()
		if got == nil || *got != 1 {
			t.Errorf("GetTargetMin() = %v, want 1", got)
		}
	})

	t.Run("GetTargetMax", func(t *testing.T) {
		got := rel.GetTargetMax()
		if got == nil || *got != 5 {
			t.Errorf("GetTargetMax() = %v, want 5", got)
		}
	})

	t.Run("GetInverse", func(t *testing.T) {
		got := rel.GetInverse()
		if got == nil {
			t.Error("GetInverse() returned nil")
		}
	})

	t.Run("GetInverse nil", func(t *testing.T) {
		relNoInverse := RelationDef{Label: "test"}
		got := relNoInverse.GetInverse()
		if got != nil {
			t.Errorf("GetInverse() = %v, want nil", got)
		}
	})
}

func TestErrorTypes(t *testing.T) {
	t.Run("RelationNotFoundError", func(t *testing.T) {
		err := &RelationNotFoundError{Name: "unknown"}
		want := "unknown relation: unknown"
		if got := err.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("InvalidRelationError", func(t *testing.T) {
		err := &InvalidRelationError{
			Relation: "implements",
			From:     "component",
			To:       "design",
			Message:  "target entity type not allowed",
		}
		want := "invalid relation implements from component to design: target entity type not allowed"
		if got := err.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("InvalidIDTypeError", func(t *testing.T) {
		err := &InvalidIDTypeError{
			EntityType: "requirement",
			IDType:     "invalid",
		}
		want := "invalid id_type for entity requirement: invalid (must be 'auto' or 'manual')"
		if got := err.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("ReservedPropertyError", func(t *testing.T) {
		err := &ReservedPropertyError{
			EntityType:   "requirement",
			PropertyName: "id",
		}
		want := `entity requirement: property "id" is reserved and cannot be used`
		if got := err.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("WhitespacePropertyError", func(t *testing.T) {
		err := &WhitespacePropertyError{
			EntityType:   "requirement",
			PropertyName: " title ",
		}
		want := `entity requirement: property name " title " has leading or trailing whitespace`
		if got := err.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("ConflictingIDPrefixError", func(t *testing.T) {
		err := &ConflictingIDPrefixError{
			EntityType: "requirement",
		}
		want := "entity requirement specifies both id_prefix and id_prefixes; use only one"
		if got := err.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})
}

func TestParse_ValidationWhenThen(t *testing.T) {
	yaml := `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_patterns: ["REQ-"]
    properties:
      status:
        type: string
      priority:
        type: string
validations:
  - name: accepted-needs-priority
    description: "Accepted requirements must have priority"
    entity_type: requirement
    when:
      - "status=accepted"
    then:
      - "priority!="
    severity: error
`
	m, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if len(m.Validations) != 1 {
		t.Fatalf("expected 1 validation, got %d", len(m.Validations))
	}

	rule := m.Validations[0]
	if rule.Name != "accepted-needs-priority" {
		t.Errorf("Name = %q, want %q", rule.Name, "accepted-needs-priority")
	}

	if len(rule.When) != 1 || rule.When[0] != "status=accepted" {
		t.Errorf("When = %v, want [\"status=accepted\"]", rule.When)
	}

	if len(rule.Then) != 1 || rule.Then[0] != "priority!=" {
		t.Errorf("Then = %v, want [\"priority!=\"]", rule.Then)
	}
}
