package metamodel

import (
	"strings"
	"testing"
)

// TestRelationScope_Parse pins the `scope:` declaration (TKT-DOFYR1,
// design doc §2.2): absent = identity (the compat default), explicit
// identity and content parse, anything else is a load error.
func TestRelationScope_Parse(t *testing.T) {
	t.Parallel()

	const schema = `version: "1.0"
entities:
  page:
    label: Page
    plural: pages
    id_prefix: "PAGE-"
    properties:
      title:
        type: string
relations:
  owned-by:
    label: owned by
    from: [page]
    to: [page]
  references:
    label: references
    from: [page]
    to: [page]
    scope: content
  contains:
    label: contains
    from: [page]
    to: [page]
    scope: identity
`
	m, err := Parse([]byte(schema))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got := m.Relations["owned-by"].Scope; got.IsContent() {
		t.Errorf("absent scope = %q, want identity default", got)
	}
	if got := m.Relations["references"].Scope; !got.IsContent() {
		t.Errorf("scope: content parsed as %q", got)
	}
	if got := m.Relations["contains"].Scope; got.IsContent() || !got.IsValid() {
		t.Errorf("scope: identity parsed as %q", got)
	}
}

func TestRelationScope_RejectsUnknownValue(t *testing.T) {
	t.Parallel()

	const schema = `version: "1.0"
entities:
  page:
    label: Page
    plural: pages
    id_prefix: "PAGE-"
    properties:
      title:
        type: string
relations:
  references:
    label: references
    from: [page]
    to: [page]
    scope: per-state
`
	_, err := Parse([]byte(schema))
	if err == nil || !strings.Contains(err.Error(), "invalid scope") {
		t.Fatalf("Parse err = %v, want invalid-scope load error", err)
	}
}
