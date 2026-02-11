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

	meta, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
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

	meta, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
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

	meta, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
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

	_, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
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

	_, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertError(t, err)

	var circularErr *CircularIncludeError
	if !errors.As(err, &circularErr) {
		t.Fatalf("expected CircularIncludeError, got %T: %v", err, err)
	}
}

func TestLoadWithIncludes_DuplicateEntity(t *testing.T) {
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - a.yaml
  - b.yaml
`)

	createFile(t, filepath.Join(tmpDir, "a.yaml"), `
entities:
  control:
    label: Control
    id_prefix: "CTL-"
    id_type: sequential
    properties:
      title:
        type: string
`)

	createFile(t, filepath.Join(tmpDir, "b.yaml"), `
entities:
  control:
    label: Control Duplicate
    id_prefix: "CTL-"
    id_type: sequential
    properties:
      title:
        type: string
`)

	_, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertError(t, err)

	var dupErr *DuplicateDefinitionError
	if !errors.As(err, &dupErr) {
		t.Fatalf("expected DuplicateDefinitionError, got %T: %v", err, err)
	}
	assertEqual(t, dupErr.Kind, "entity")
	assertEqual(t, dupErr.Name, "control")
	assertEqual(t, dupErr.File1, "a.yaml")
	assertEqual(t, dupErr.File2, "b.yaml")
}

func TestLoadWithIncludes_DuplicateType(t *testing.T) {
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - a.yaml
types:
  severity:
    values: [low, medium, high]
`)

	createFile(t, filepath.Join(tmpDir, "a.yaml"), `
types:
  severity:
    values: [low, high, critical]
`)

	_, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertError(t, err)

	var dupErr *DuplicateDefinitionError
	if !errors.As(err, &dupErr) {
		t.Fatalf("expected DuplicateDefinitionError, got %T: %v", err, err)
	}
	assertEqual(t, dupErr.Kind, "type")
	assertEqual(t, dupErr.Name, "severity")
}

func TestLoadWithIncludes_DuplicateRelation(t *testing.T) {
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - a.yaml
  - b.yaml
`)

	createFile(t, filepath.Join(tmpDir, "a.yaml"), `
relations:
  mitigates:
    label: mitigates
    from: [decision]
    to: [risk]
`)

	createFile(t, filepath.Join(tmpDir, "b.yaml"), `
relations:
  mitigates:
    label: mitigates
    from: [solution]
    to: [risk]
`)

	_, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertError(t, err)

	var dupErr *DuplicateDefinitionError
	if !errors.As(err, &dupErr) {
		t.Fatalf("expected DuplicateDefinitionError, got %T: %v", err, err)
	}
	assertEqual(t, dupErr.Kind, "relation")
	assertEqual(t, dupErr.Name, "mitigates")
}

func TestLoadWithIncludes_DuplicateValidation(t *testing.T) {
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - a.yaml
validations:
  - name: needs-title
    description: "Must have title"
    then:
      - "title!="
`)

	createFile(t, filepath.Join(tmpDir, "a.yaml"), `
validations:
  - name: needs-title
    description: "Duplicate rule"
    then:
      - "title!="
`)

	_, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertError(t, err)

	var dupErr *DuplicateDefinitionError
	if !errors.As(err, &dupErr) {
		t.Fatalf("expected DuplicateDefinitionError, got %T: %v", err, err)
	}
	assertEqual(t, dupErr.Kind, "validation")
	assertEqual(t, dupErr.Name, "needs-title")
}

func TestLoadWithIncludes_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - nonexistent.yaml
`)

	_, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertError(t, err)

	var notFoundErr *IncludeNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected IncludeNotFoundError, got %T: %v", err, err)
	}
	assertEqual(t, notFoundErr.Path, "nonexistent.yaml")
}

func TestLoadWithIncludes_IncludeHasVersion(t *testing.T) {
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - bad.yaml
`)

	createFile(t, filepath.Join(tmpDir, "bad.yaml"), `
version: "1.0"
entities:
  control:
    label: Control
    id_prefix: "CTL-"
    id_type: sequential
    properties:
      title:
        type: string
`)

	_, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertError(t, err)

	var rootFieldErr *IncludeHasRootFieldError
	if !errors.As(err, &rootFieldErr) {
		t.Fatalf("expected IncludeHasRootFieldError, got %T: %v", err, err)
	}
	assertEqual(t, rootFieldErr.Field, "version")
}

func TestLoadWithIncludes_IncludeHasNamespace(t *testing.T) {
	tmpDir := t.TempDir()

	createFile(t, filepath.Join(tmpDir, "metamodel.yaml"), `
version: "1.0"
includes:
  - bad.yaml
`)

	createFile(t, filepath.Join(tmpDir, "bad.yaml"), `
namespace: "https://example.org"
entities:
  control:
    label: Control
    id_prefix: "CTL-"
    id_type: sequential
    properties:
      title:
        type: string
`)

	_, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertError(t, err)

	var rootFieldErr *IncludeHasRootFieldError
	if !errors.As(err, &rootFieldErr) {
		t.Fatalf("expected IncludeHasRootFieldError, got %T: %v", err, err)
	}
	assertEqual(t, rootFieldErr.Field, "namespace")
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

	meta, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
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

	meta, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
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

	meta, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
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

	meta, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
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

	meta, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
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

	_, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
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

	meta, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
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

	meta, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
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

	meta, err := Load(filepath.Join(tmpDir, "metamodel.yaml"), testMetaFS)
	assertNoError(t, err)

	if _, ok := meta.Entities["requirement"]; !ok {
		t.Error("expected requirement entity")
	}
}

// Error message tests

func TestDuplicateDefinitionError_Error(t *testing.T) {
	err := &DuplicateDefinitionError{
		Kind: "entity", Name: "control",
		File1: "compliance.yaml", File2: "risk.yaml",
	}
	expected := `duplicate entity "control": defined in both compliance.yaml and risk.yaml`
	assertEqual(t, err.Error(), expected)
}

func TestCircularIncludeError_Error(t *testing.T) {
	err := &CircularIncludeError{
		Chain: []string{"metamodel.yaml", "a.yaml", "b.yaml", "a.yaml"},
	}
	expected := "circular include detected: metamodel.yaml → a.yaml → b.yaml → a.yaml"
	assertEqual(t, err.Error(), expected)
}

func TestIncludeNotFoundError_Error(t *testing.T) {
	err := &IncludeNotFoundError{
		Path:         "compliance.yaml",
		IncludedFrom: "metamodel.yaml",
	}
	expected := "include file not found: compliance.yaml (included from metamodel.yaml)"
	assertEqual(t, err.Error(), expected)
}

func TestIncludeHasRootFieldError_Error(t *testing.T) {
	err := &IncludeHasRootFieldError{
		Path:  "compliance.yaml",
		Field: "version",
	}
	expected := `included file compliance.yaml must not contain "version" (only allowed in root metamodel.yaml)`
	assertEqual(t, err.Error(), expected)
}
