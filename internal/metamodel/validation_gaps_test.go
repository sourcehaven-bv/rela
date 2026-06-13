package metamodel

import (
	"strings"
	"testing"
)

// TestValidatePropertyValue_IntegerRejectsFractional pins that an
// integer property rejects a float with a fractional part rather than
// silently truncating it (count: 3.5 must error, not become 3). An
// integral float (3.0) is still accepted — YAML emits whole numbers as
// int but a hand-edited "3.0" arrives as float64.
func TestValidatePropertyValue_IntegerRejectsFractional(t *testing.T) {
	meta := &Metamodel{}
	propDef := &PropertyDef{Type: PropertyTypeInteger}

	if err := meta.ValidatePropertyValue("count", propDef, 3.5); err == nil {
		t.Error("expected error for fractional float 3.5 on an integer property")
	}
	if err := meta.ValidatePropertyValue("count", propDef, 3.0); err != nil {
		t.Errorf("integral float 3.0 should be accepted, got: %v", err)
	}
}

// TestParseIntegerValue_RejectsFractional pins the same rule at the
// parse helper used by filter sort/match — a fractional float errors
// rather than truncating.
func TestParseIntegerValue_RejectsFractional(t *testing.T) {
	if _, err := ParseIntegerValue(3.5); err == nil {
		t.Error("expected error parsing fractional 3.5 as integer")
	}
	got, err := ParseIntegerValue(7.0)
	if err != nil || got != 7 {
		t.Errorf("ParseIntegerValue(7.0) = (%d, %v), want (7, nil)", got, err)
	}
}

// TestValidateRelationReferences_RejectsEmptyFromTo pins that a relation
// declaring no 'from' or no 'to' entity types is a load error — such a
// relation is meaningless (no entity can be a valid endpoint) and any
// cardinality constraint on it would be a silent no-op.
func TestValidateRelationReferences_RejectsEmptyFromTo(t *testing.T) {
	tests := []struct {
		name    string
		rel     RelationDef
		wantSub string
	}{
		{
			name:    "empty from",
			rel:     RelationDef{From: nil, To: []string{"feature"}},
			wantSub: "'from'",
		},
		{
			name:    "empty to",
			rel:     RelationDef{From: []string{"ticket"}, To: nil},
			wantSub: "'to'",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &Metamodel{
				Entities: map[string]EntityDef{
					"ticket":  {},
					"feature": {},
				},
				Relations: map[string]RelationDef{"rel": tc.rel},
			}
			errs := validateRelationReferences(m)
			found := false
			for _, e := range errs {
				if strings.Contains(e, "at least one") && strings.Contains(e, tc.wantSub) {
					found = true
				}
			}
			if !found {
				t.Errorf("expected an 'at least one %s' error, got: %v", tc.wantSub, errs)
			}
		})
	}
}

// TestValidateRelationReferences_AllowsPopulatedFromTo guards against
// over-rejection: a relation with both endpoints declared (and existing)
// produces no error.
func TestValidateRelationReferences_AllowsPopulatedFromTo(t *testing.T) {
	m := &Metamodel{
		Entities: map[string]EntityDef{"ticket": {}, "feature": {}},
		Relations: map[string]RelationDef{
			"implements": {From: []string{"ticket"}, To: []string{"feature"}},
		},
	}
	if errs := validateRelationReferences(m); len(errs) != 0 {
		t.Errorf("a fully-populated relation should produce no errors, got: %v", errs)
	}
}
