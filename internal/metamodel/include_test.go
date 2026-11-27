package metamodel

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestLoadWithIncludes_Basic(t *testing.T) {
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
namespace: "https://example.org/ontology#"
includes:
  - compliance.yaml
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
`)

	createFile(t, filepath.Join(tmpDir, "compliance.yaml"), `
entities:
  control:
    label: Control
    id_prefix: "CTL-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
`)

	meta, _, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertNoError(t, err)

	if _, ok := meta.Entities["requirement"]; !ok {
		t.Error("expected requirement entity from root")
	}
	if _, ok := meta.Entities["control"]; !ok {
		t.Error("expected control entity from included file")
	}
	if len(meta.Includes) != 0 {
		t.Error("expected includes to be cleared after merging")
	}
}

func TestLoadWithIncludes_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - types.yaml
  - entities.yaml
  - relations.yaml
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
`)

	createFile(t, filepath.Join(tmpDir, "types.yaml"), `
types:
  severity:
    values: [low, medium, high, critical]
`)

	createFile(t, filepath.Join(tmpDir, "entities.yaml"), `
entities:
  risk:
    label: Risk
    id_prefix: "RISK-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
      severity:
        type: severity
`)

	createFile(t, filepath.Join(tmpDir, "relations.yaml"), `
relations:
  mitigates:
    label: mitigates
    from: [requirement]
    to: [risk]
    inverse: mitigatedBy
`)

	meta, _, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertNoError(t, err)

	// Types from types.yaml
	if _, ok := meta.Types["severity"]; !ok {
		t.Error("expected severity type from included file")
	}

	// Entities from root + entities.yaml
	if _, ok := meta.Entities["requirement"]; !ok {
		t.Error("expected requirement entity from root")
	}
	if _, ok := meta.Entities["risk"]; !ok {
		t.Error("expected risk entity from included file")
	}

	// Relations from relations.yaml
	if _, ok := meta.Relations["mitigates"]; !ok {
		t.Error("expected mitigates relation from included file")
	}
}

func TestLoadWithIncludes_Nested(t *testing.T) {
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - a.yaml
entities:
  root_entity:
    label: Root
    id_prefix: "ROOT-"
    id_type: sequential
    properties:
      title:
        type: string
`)

	createFile(t, filepath.Join(tmpDir, "a.yaml"), `
includes:
  - b.yaml
entities:
  entity_a:
    label: Entity A
    id_prefix: "A-"
    id_type: sequential
    properties:
      title:
        type: string
`)

	createFile(t, filepath.Join(tmpDir, "b.yaml"), `
entities:
  entity_b:
    label: Entity B
    id_prefix: "B-"
    id_type: sequential
    properties:
      title:
        type: string
`)

	meta, _, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertNoError(t, err)

	if _, ok := meta.Entities["root_entity"]; !ok {
		t.Error("expected root_entity from root")
	}
	if _, ok := meta.Entities["entity_a"]; !ok {
		t.Error("expected entity_a from a.yaml")
	}
	if _, ok := meta.Entities["entity_b"]; !ok {
		t.Error("expected entity_b from b.yaml (nested include)")
	}
}

func TestLoadWithIncludes_CircularDetection(t *testing.T) {
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - a.yaml
`)

	createFile(t, filepath.Join(tmpDir, "a.yaml"), `
includes:
  - b.yaml
entities:
  entity_a:
    label: A
    id_prefix: "A-"
    id_type: sequential
    properties:
      title:
        type: string
`)

	createFile(t, filepath.Join(tmpDir, "b.yaml"), `
includes:
  - a.yaml
entities:
  entity_b:
    label: B
    id_prefix: "B-"
    id_type: sequential
    properties:
      title:
        type: string
`)

	_, _, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertError(t, err)

	var circularErr *CircularIncludeError
	if !errors.As(err, &circularErr) {
		t.Fatalf("expected CircularIncludeError, got %T: %v", err, err)
	}
}

func TestLoadWithIncludes_SelfInclude(t *testing.T) {
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - metamodel.yaml
`)

	_, _, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertError(t, err)

	var circularErr *CircularIncludeError
	if !errors.As(err, &circularErr) {
		t.Fatalf("expected CircularIncludeError, got %T: %v", err, err)
	}
}

func TestLoadWithIncludes_Duplicates(t *testing.T) {
	tests := []struct {
		name     string
		root     string
		includes map[string]string
		wantKind string
		wantName string
		// wantFile1 and wantFile2 are checked only when set.
		wantFile1, wantFile2 string
	}{
		{
			name: "DuplicateEntity",
			root: `
version: "1.0"
includes:
  - a.yaml
  - b.yaml
`,
			includes: map[string]string{
				"a.yaml": `
entities:
  control:
    label: Control
    id_prefix: "CTL-"
    id_type: sequential
    properties:
      title:
        type: string
`,
				"b.yaml": `
entities:
  control:
    label: Control Duplicate
    id_prefix: "CTL-"
    id_type: sequential
    properties:
      title:
        type: string
`,
			},
			wantKind:  "entity",
			wantName:  "control",
			wantFile1: "a.yaml",
			wantFile2: "b.yaml",
		},
		{
			name: "DuplicateType",
			root: `
version: "1.0"
includes:
  - a.yaml
types:
  severity:
    values: [low, medium, high]
`,
			includes: map[string]string{
				"a.yaml": `
types:
  severity:
    values: [low, high, critical]
`,
			},
			wantKind: "type",
			wantName: "severity",
		},
		{
			name: "DuplicateRelation",
			root: `
version: "1.0"
includes:
  - a.yaml
  - b.yaml
`,
			includes: map[string]string{
				"a.yaml": `
relations:
  mitigates:
    label: mitigates
    from: [decision]
    to: [risk]
`,
				"b.yaml": `
relations:
  mitigates:
    label: mitigates
    from: [solution]
    to: [risk]
`,
			},
			wantKind: "relation",
			wantName: "mitigates",
		},
		{
			name: "DuplicateValidation",
			root: `
version: "1.0"
includes:
  - a.yaml
validations:
  - name: needs-title
    description: "Must have title"
    then:
      - "title!="
`,
			includes: map[string]string{
				"a.yaml": `
validations:
  - name: needs-title
    description: "Duplicate rule"
    then:
      - "title!="
`,
			},
			wantKind: "validation",
			wantName: "needs-title",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), tc.root)
			for name, content := range tc.includes {
				createFile(t, filepath.Join(tmpDir, name), content)
			}

			_, _, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
			assertError(t, err)

			var dupErr *DuplicateDefinitionError
			if !errors.As(err, &dupErr) {
				t.Fatalf("expected DuplicateDefinitionError, got %T: %v", err, err)
			}
			assertEqual(t, dupErr.Kind, tc.wantKind)
			assertEqual(t, dupErr.Name, tc.wantName)
			if tc.wantFile1 != "" {
				assertEqual(t, dupErr.File1, tc.wantFile1)
			}
			if tc.wantFile2 != "" {
				assertEqual(t, dupErr.File2, tc.wantFile2)
			}
		})
	}
}

func TestLoadWithIncludes_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - nonexistent.yaml
`)

	_, _, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertError(t, err)

	var notFoundErr *IncludeNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected IncludeNotFoundError, got %T: %v", err, err)
	}
	assertEqual(t, notFoundErr.Path, "nonexistent.yaml")
}

// loadRootFieldError loads a root schema that includes one file and returns
// the IncludeHasRootFieldError the load must fail with.
func loadRootFieldError(t *testing.T, includeName, includeYAML string) *IncludeHasRootFieldError {
	t.Helper()
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - `+includeName+`
`)
	createFile(t, filepath.Join(tmpDir, includeName), includeYAML)

	_, _, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertError(t, err)

	var rootFieldErr *IncludeHasRootFieldError
	if !errors.As(err, &rootFieldErr) {
		t.Fatalf("expected IncludeHasRootFieldError, got %T: %v", err, err)
	}
	return rootFieldErr
}

func TestLoadWithIncludes_IncludeHasRootField(t *testing.T) {
	const control = `
entities:
  control:
    label: Control
    id_prefix: "CTL-"
    id_type: sequential
    properties:
      title:
        type: string
`
	tests := []struct {
		name   string
		header string
		field  string
	}{
		{"IncludeHasVersion", `version: "1.0"`, "version"},
		{"IncludeHasIRI", `iri: "https://example.org"`, "iri"},
		// description is a deployment-wide, root-only field (TKT-0YBFT8): like
		// version and namespace it must be rejected in an included partial,
		// not silently dropped.
		{"IncludeHasDescription", "description: A partial should not carry the deployment description.", "description"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := loadRootFieldError(t, "bad.yaml", "\n"+tc.header+control)
			assertEqual(t, err.Field, tc.field)
		})
	}
}

func TestLoadWithIncludes_EmptyIncludes(t *testing.T) {
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes: []
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
`)

	meta, _, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertNoError(t, err)

	if _, ok := meta.Entities["requirement"]; !ok {
		t.Error("expected requirement entity")
	}
}

func TestLoadWithIncludes_CrossFileTypeReferences(t *testing.T) {
	tmpDir := t.TempDir()

	// Root defines entities that use types from an included file
	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - types.yaml
entities:
  risk:
    label: Risk
    id_prefix: "RISK-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
      severity:
        type: severity
`)

	createFile(t, filepath.Join(tmpDir, "types.yaml"), `
types:
  severity:
    values: [low, medium, high, critical]
`)

	meta, _, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertNoError(t, err)

	// Verify cross-file reference works: entity uses type from included file
	if _, ok := meta.Types["severity"]; !ok {
		t.Error("expected severity type from included file")
	}
	riskDef, ok := meta.Entities["risk"]
	if !ok {
		t.Fatal("expected risk entity")
	}
	severityProp, ok := riskDef.Properties["severity"]
	if !ok {
		t.Fatal("expected severity property on risk entity")
	}
	assertEqual(t, severityProp.Type, "severity")
}

func TestLoadWithIncludes_CrossFileRelationReferences(t *testing.T) {
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - entities.yaml
  - relations.yaml
`)

	createFile(t, filepath.Join(tmpDir, "entities.yaml"), `
entities:
  decision:
    label: Decision
    id_prefix: "DEC-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
  risk:
    label: Risk
    id_prefix: "RISK-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
`)

	createFile(t, filepath.Join(tmpDir, "relations.yaml"), `
relations:
  mitigates:
    label: mitigates
    from: [decision]
    to: [risk]
    inverse: mitigatedBy
`)

	meta, _, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertNoError(t, err)

	// Verify relation references entities from another included file
	rel, ok := meta.Relations["mitigates"]
	if !ok {
		t.Fatal("expected mitigates relation")
	}
	if len(rel.From) != 1 || rel.From[0] != "decision" {
		t.Errorf("expected from=[decision], got %v", rel.From)
	}
	if len(rel.To) != 1 || rel.To[0] != "risk" {
		t.Errorf("expected to=[risk], got %v", rel.To)
	}
}

func TestLoadWithIncludes_DiamondInclude(t *testing.T) {
	tmpDir := t.TempDir()

	// A includes B and C, both B and C include D
	// D should be loaded once, not cause a duplicate error
	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - b.yaml
  - c.yaml
`)

	createFile(t, filepath.Join(tmpDir, "b.yaml"), `
includes:
  - d.yaml
entities:
  entity_b:
    label: B
    id_prefix: "B-"
    id_type: sequential
    properties:
      title:
        type: string
`)

	createFile(t, filepath.Join(tmpDir, "c.yaml"), `
includes:
  - d.yaml
entities:
  entity_c:
    label: C
    id_prefix: "C-"
    id_type: sequential
    properties:
      title:
        type: string
`)

	createFile(t, filepath.Join(tmpDir, "d.yaml"), `
types:
  shared_type:
    values: [a, b, c]
`)

	meta, _, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertNoError(t, err)

	// All entities should be present
	if _, ok := meta.Entities["entity_b"]; !ok {
		t.Error("expected entity_b from b.yaml")
	}
	if _, ok := meta.Entities["entity_c"]; !ok {
		t.Error("expected entity_c from c.yaml")
	}
	// Shared type from d.yaml should be loaded once
	if _, ok := meta.Types["shared_type"]; !ok {
		t.Error("expected shared_type from d.yaml")
	}
}

func TestLoadWithIncludes_Subdirectory(t *testing.T) {
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - modules/compliance.yaml
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
`)

	createFile(t, filepath.Join(tmpDir, "modules", "compliance.yaml"), `
entities:
  control:
    label: Control
    id_prefix: "CTL-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
`)

	meta, _, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertNoError(t, err)

	if _, ok := meta.Entities["control"]; !ok {
		t.Error("expected control entity from modules/compliance.yaml")
	}
}

func TestLoadWithIncludes_DuplicateEntityWithRoot(t *testing.T) {
	tmpDir := t.TempDir()

	// Entity defined in root AND in included file
	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - extra.yaml
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
`)

	createFile(t, filepath.Join(tmpDir, "extra.yaml"), `
entities:
  requirement:
    label: Requirement Duplicate
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
`)

	_, _, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertError(t, err)

	var dupErr *DuplicateDefinitionError
	if !errors.As(err, &dupErr) {
		t.Fatalf("expected DuplicateDefinitionError, got %T: %v", err, err)
	}
	assertEqual(t, dupErr.Kind, "entity")
	assertEqual(t, dupErr.Name, "requirement")
}

func TestLoadWithIncludes_ValidationsMerged(t *testing.T) {
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - rules.yaml
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
      priority:
        type: string
validations:
  - name: root-rule
    description: "Root validation"
    then:
      - "title!="
`)

	createFile(t, filepath.Join(tmpDir, "rules.yaml"), `
validations:
  - name: included-rule
    description: "Included validation"
    entity_type: requirement
    then:
      - "priority!="
    severity: warning
`)

	meta, _, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertNoError(t, err)

	if len(meta.Validations) != 2 {
		t.Fatalf("expected 2 validations, got %d", len(meta.Validations))
	}

	names := map[string]bool{}
	for _, v := range meta.Validations {
		names[v.Name] = true
	}
	if !names["root-rule"] {
		t.Error("expected root-rule validation")
	}
	if !names["included-rule"] {
		t.Error("expected included-rule validation")
	}
}

func TestLoadWithIncludes_AliasMapBuilt(t *testing.T) {
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - extra.yaml
entities:
  requirement:
    label: Requirement
    aliases: [req]
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
`)

	createFile(t, filepath.Join(tmpDir, "extra.yaml"), `
entities:
  control:
    label: Control
    aliases: [ctl, ctrl]
    id_prefix: "CTL-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
`)

	meta, _, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertNoError(t, err)

	// Aliases from root
	assertEqual(t, meta.ResolveAlias("req"), "requirement")

	// Aliases from included file
	assertEqual(t, meta.ResolveAlias("ctl"), "control")
	assertEqual(t, meta.ResolveAlias("ctrl"), "control")
}

func TestLoadWithIncludes_NoIncludes(t *testing.T) {
	// A regular metamodel without includes should work exactly as before
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
`)

	meta, _, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertNoError(t, err)

	if _, ok := meta.Entities["requirement"]; !ok {
		t.Error("expected requirement entity")
	}
}

func TestIncludeErrors_Error(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "DuplicateDefinitionError",
			err: &DuplicateDefinitionError{
				Kind: "entity", Name: "control",
				File1: "compliance.yaml", File2: "risk.yaml",
			},
			want: `duplicate entity "control": defined in both compliance.yaml and risk.yaml`,
		},
		{
			name: "CircularIncludeError",
			err: &CircularIncludeError{
				Chain: []string{"metamodel.yaml", "a.yaml", "b.yaml", "a.yaml"},
			},
			want: "circular include detected: metamodel.yaml → a.yaml → b.yaml → a.yaml",
		},
		{
			name: "IncludeNotFoundError",
			err: &IncludeNotFoundError{
				Path:         "compliance.yaml",
				IncludedFrom: "metamodel.yaml",
			},
			want: "include file not found: compliance.yaml (included from metamodel.yaml)",
		},
		{
			name: "IncludeHasRootFieldError",
			err: &IncludeHasRootFieldError{
				Path:  "compliance.yaml",
				Field: "version",
			},
			want: `included file compliance.yaml must not contain "version" (only allowed in root schema file)`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertEqual(t, tc.err.Error(), tc.want)
		})
	}
}

// TestLoadWithIncludes_IncludeHasWorlds pins that `worlds:` is root-only
// (TKT-WAV8XP). An included file MAY declare `entities:` with `faces:`,
// and that half merges — so a silently dropped `worlds:` would leave a
// project holding content states with no world and no diagnostic.
func TestLoadWithIncludes_IncludeHasWorlds(t *testing.T) {
	err := loadRootFieldError(t, "pages.yaml", `
entities:
  page:
    label: Page
    id_prefix: "PAGE-"
    id_type: sequential
    properties:
      title:
        type: string
    faces:
      draft: {}
      published: {}
worlds:
  published:
    select: published
    otherwise: exclude
`)
	assertEqual(t, err.Field, "worlds")
}

// TestLoadWithIncludes_IncludeHasCopies pins that `copies:` is root-only too.
// Before this the key parsed into nothing: the definitions vanished with no
// diagnostic, and the symptom was a promote button that never appeared for a
// definition the operator could see in their file.
func TestLoadWithIncludes_IncludeHasCopies(t *testing.T) {
	err := loadRootFieldError(t, "pages.yaml", `
entities:
  page:
    label: Page
    id_prefix: "PAGE-"
    id_type: sequential
    properties:
      title:
        type: string
    faces:
      draft: {}
      published: {}
copies:
  promote:
    from: page@draft
    to: page@published
    fields: all
    guard:
      permission: promote
`)
	assertEqual(t, err.Field, "copies")
}
