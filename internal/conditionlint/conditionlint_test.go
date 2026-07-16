package conditionlint

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func testMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Label: "Ticket",
				Properties: map[string]metamodel.PropertyDef{
					"title":          {Type: "string", Required: true},
					"status":         {Type: "status"},
					"has_processors": {Type: "boolean"},
					"count":          {Type: "integer"},
				},
			},
		},
	}
}

func runLint(t *testing.T, form dataentryconfig.Form) []string {
	t.Helper()
	cfg := &dataentryconfig.Config{Forms: map[string]dataentryconfig.Form{"f": form}}
	return Lint(cfg, testMeta())
}

func TestLint_ValidConditions(t *testing.T) {
	errs := runLint(t, dataentryconfig.Form{
		EntityType: "ticket",
		Steps: []dataentryconfig.FormStep{
			{Title: "A", Fields: []dataentryconfig.FormField{{Property: "title"}}},
			{
				Title:       "B",
				VisibleWhen: "form.has_processors == true",
				Fields: []dataentryconfig.FormField{
					{Property: "status", RequiredWhen: "form.has_processors == true"},
				},
			},
		},
	})
	if len(errs) != 0 {
		t.Fatalf("expected no lint errors, got: %v", errs)
	}
}

func TestLint_ParseError(t *testing.T) {
	errs := runLint(t, dataentryconfig.Form{
		EntityType: "ticket",
		Steps:      []dataentryconfig.FormStep{{Title: "A", VisibleWhen: "form.status =="}},
	})
	if len(errs) == 0 {
		t.Fatal("expected a parse error for a malformed condition")
	}
	if !strings.Contains(errs[0], "step[0] visible_when") {
		t.Errorf("expected step-scoped location, got: %v", errs)
	}
}

func TestLint_UnknownFieldReference(t *testing.T) {
	errs := runLint(t, dataentryconfig.Form{
		EntityType: "ticket",
		Steps: []dataentryconfig.FormStep{
			{Title: "A", Fields: []dataentryconfig.FormField{
				{Property: "title", RequiredWhen: "form.nonexistent == true"},
			}},
		},
	})
	if len(errs) == 0 {
		t.Fatal("expected an error for referencing an unknown field")
	}
	if !strings.Contains(errs[0], "required_when") {
		t.Errorf("expected required_when location, got: %v", errs)
	}
}

func TestLint_FlatFormConditions(t *testing.T) {
	errs := runLint(t, dataentryconfig.Form{
		EntityType: "ticket",
		Fields: []dataentryconfig.FormField{
			{Property: "status", VisibleWhen: "form.bogus == 1"},
		},
	})
	if len(errs) == 0 {
		t.Fatal("expected an error for a flat-form condition referencing an unknown field")
	}
}

func TestLint_UnknownEntityTypeSkipped(t *testing.T) {
	// An unknown entity type is reported elsewhere; conditions can't be grounded
	// so this package stays silent rather than emitting confusing errors.
	errs := runLint(t, dataentryconfig.Form{
		EntityType: "does-not-exist",
		Steps:      []dataentryconfig.FormStep{{Title: "A", VisibleWhen: "form.x =="}},
	})
	if len(errs) != 0 {
		t.Fatalf("expected no errors for unknown entity type, got: %v", errs)
	}
}

func TestLint_EmptyConditionsIgnored(t *testing.T) {
	errs := runLint(t, dataentryconfig.Form{
		EntityType: "ticket",
		Steps:      []dataentryconfig.FormStep{{Title: "A", Fields: []dataentryconfig.FormField{{Property: "title"}}}},
	})
	if len(errs) != 0 {
		t.Fatalf("expected no errors when no conditions are set, got: %v", errs)
	}
}
