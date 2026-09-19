package metamodel

import (
	"strings"
	"testing"
)

// keysMeta declares a `gaat_over` relation reaching two source types, plus a
// symmetric relation, so each load-time check has something real to resolve
// against.
func keysMeta(c RelationConstraint, relType string) *Metamodel {
	return &Metamodel{
		Entities: map[string]EntityDef{
			"procedure":   {Properties: map[string]PropertyDef{"status": {Type: "string"}}},
			"taak":        {Properties: map[string]PropertyDef{"status": {Type: "string"}}},
			"terugkerend": {Properties: map[string]PropertyDef{"actief": {Type: "string"}}},
		},
		Relations: map[string]RelationDef{
			"gaat_over": {From: []string{"taak", "terugkerend"}, To: []string{"procedure"}},
			"hangt_samen_met": {
				From: []string{"procedure"}, To: []string{"procedure"}, Symmetric: true,
			},
		},
		Validations: []ValidationRule{{
			Name:       "r",
			EntityType: "procedure",
			Relations:  map[string]RelationConstraint{relType: c},
		}},
	}
}

// Each of these mistakes makes a constraint match nothing or count the wrong
// edges, and every one of them is silent at runtime. They have to fail at
// load, where the operator is looking.
func TestValidateValidationRelations_NewKeys(t *testing.T) {
	tests := []struct {
		name     string
		relType  string
		c        RelationConstraint
		wantErr  bool
		contains string
	}{
		{
			name:    "incoming with a reachable target_type is fine",
			relType: "gaat_over",
			c: RelationConstraint{
				Direction: RelationDirectionIncoming, TargetType: "taak",
				Where: []string{"status!=gereed"}, Min: new(1),
			},
		},
		{
			name:    "outgoing default with no new keys is fine",
			relType: "gaat_over",
			c:       RelationConstraint{Min: new(1)},
		},
		{
			name:     "an unrecognized direction is refused",
			relType:  "gaat_over",
			c:        RelationConstraint{Direction: "sideways", Min: new(1)},
			wantErr:  true,
			contains: "is not valid",
		},
		{
			// Otherwise the two endpoints of one relationship get different
			// counts depending on which way round the edge was written.
			name:     "direction on a symmetric relation is refused",
			relType:  "hangt_samen_met",
			c:        RelationConstraint{Direction: RelationDirectionIncoming, Min: new(1)},
			wantErr:  true,
			contains: "symmetric",
		},
		{
			name:     "an undeclared target_type is refused",
			relType:  "gaat_over",
			c:        RelationConstraint{TargetType: "nonexistent", Min: new(1)},
			wantErr:  true,
			contains: "not a declared entity type",
		},
		{
			// `procedure` exists, but gaat_over never points AT one from the
			// incoming side — so this counts nothing and passes forever.
			name:    "a target_type the relation cannot reach is refused",
			relType: "gaat_over",
			c: RelationConstraint{
				Direction: RelationDirectionIncoming, TargetType: "procedure", Min: new(1),
			},
			wantErr:  true,
			contains: "not reachable",
		},
		{
			// Direction decides which side `target_type` is checked against:
			// procedure IS reachable outgoing, so the same value is fine here.
			name:     "the same target_type is fine on the other side",
			relType:  "gaat_over",
			c:        RelationConstraint{TargetType: "procedure", Min: new(1)},
			wantErr:  false,
			contains: "",
		},
		{
			name:    "a where property the target_type lacks is refused",
			relType: "gaat_over",
			c: RelationConstraint{
				Direction: RelationDirectionIncoming, TargetType: "taak",
				Where: []string{"actief=ja"}, Min: new(1),
			},
			wantErr:  true,
			contains: "which none of",
		},
		{
			// Without target_type the relation reaches taak AND terugkerend.
			// `actief` exists on one of them, so the clause is legitimate and
			// must not be rejected.
			name:    "a where property only one reachable type declares is allowed",
			relType: "gaat_over",
			c: RelationConstraint{
				Direction: RelationDirectionIncoming,
				Where:     []string{"actief=ja"}, Min: new(1),
			},
		},
		{
			name:    "a where property NO reachable type declares is refused",
			relType: "gaat_over",
			c: RelationConstraint{
				Direction: RelationDirectionIncoming,
				Where:     []string{"nonsense=1"}, Min: new(1),
			},
			wantErr:  true,
			contains: "which none of",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateValidationRelations(keysMeta(tc.c, tc.relType))
			got := len(errs) > 0
			if got != tc.wantErr {
				t.Fatalf("wantErr=%v got=%v: %v", tc.wantErr, got, errs)
			}
			if tc.contains == "" {
				return
			}
			joined := strings.Join(errs, "\n")
			if !strings.Contains(joined, tc.contains) {
				t.Errorf("error should mention %q; got:\n%s", tc.contains, joined)
			}
		})
	}
}

// An aliased target_type must resolve, not be read as an unknown type.
// Rejecting it would refuse a legitimate rule; matching it literally at check
// time would count nothing.
func TestValidateValidationRelations_TargetTypeAlias(t *testing.T) {
	m := keysMeta(RelationConstraint{
		Direction: RelationDirectionIncoming, TargetType: "task", Min: new(1),
	}, "gaat_over")
	def := m.Entities["taak"]
	def.Aliases = []string{"task"}
	m.Entities["taak"] = def
	// Parse() does this automatically; a hand-built metamodel must ask.
	m.InitAliases()

	if errs := validateValidationRelations(m); len(errs) > 0 {
		t.Errorf("an aliased target_type must resolve; got: %v", errs)
	}
}

// An aliased `from:`/`to:` must not make target_type unwritable.
//
// Comparing a resolved target_type against a raw From/To list rejects EVERY
// spelling — including the alias the schema itself used — with a message that
// names the alias it just refused. Both sides are canonicalized so load and
// check agree by construction.
func TestValidateValidationRelations_AliasedReachableType(t *testing.T) {
	m := keysMeta(RelationConstraint{
		Direction: RelationDirectionIncoming, TargetType: "taak", Min: new(1),
	}, "gaat_over")
	def := m.Entities["taak"]
	def.Aliases = []string{"task"}
	m.Entities["taak"] = def
	// The relation reaches the type by its ALIAS.
	rel := m.Relations["gaat_over"]
	rel.From = []string{"task", "terugkerend"}
	m.Relations["gaat_over"] = rel
	m.InitAliases()

	if errs := validateValidationRelations(m); len(errs) > 0 {
		t.Errorf("an aliased from: must still make target_type reachable; got: %v", errs)
	}
}
