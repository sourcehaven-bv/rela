package mcp

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestPropertySchemas_IntegerRange pins that an integer's declared bounds
// reach the schema tool, and that a file's max (an attachment count) does
// not pass for a value bound (TKT-BRAM1A).
func TestPropertySchemas_IntegerRange(t *testing.T) {
	h := schemaResourceHandler{meta: &metamodel.Metamodel{}}
	got := h.propertySchemas(map[string]metamodel.PropertyDef{
		"impact": {Type: metamodel.PropertyTypeInteger, Min: new(1), Max: new(5)},
		"files":  {Type: metamodel.PropertyTypeFile, Max: new(3)},
	})
	impact := got["impact"]
	if impact.Min == nil || *impact.Min != 1 || impact.Max == nil || *impact.Max != 5 {
		t.Errorf("impact range = %v..%v, want 1..5", impact.Min, impact.Max)
	}
	if got["files"].Max != nil {
		t.Errorf("file max surfaced as a value bound: %d", *got["files"].Max)
	}
}
