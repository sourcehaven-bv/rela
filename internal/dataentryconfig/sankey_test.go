package dataentryconfig

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func sankeyMeta() *metamodel.Metamodel {
	def := func() metamodel.EntityDef {
		return metamodel.EntityDef{
			DisplayProperty: "title",
			Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string"}, "status": {Type: "string"},
			},
		}
	}
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{"a": def(), "b": def(), "c": def()},
		Relations: map[string]metamodel.RelationDef{
			"rel": {From: []string{"a"}, To: []string{"b"}},
			"ev":  {From: []string{"a"}, To: []string{"c"}},
		},
	}
}

func TestValidateSankey(t *testing.T) {
	good := Sankey{Columns: [][]string{{"a"}, {"b"}}, Edges: []string{"rel"}}
	tests := []struct {
		name   string
		mutate func(*Sankey)
		want   string // substring of the one expected error, "" for none
	}{
		{"valid", func(*Sankey) {}, ""},
		{"one column", func(s *Sankey) { s.Columns = [][]string{{"a"}} }, "at least two columns"},
		{"unknown type", func(s *Sankey) { s.Columns = [][]string{{"a"}, {"zzz"}} }, `unknown entity type "zzz"`},
		{"type twice", func(s *Sankey) { s.Columns = [][]string{{"a"}, {"a"}} }, "appears twice"},
		{"empty column", func(s *Sankey) { s.Columns = [][]string{{"a"}, {}} }, "is empty"},
		{"no edges", func(s *Sankey) { s.Edges = nil }, "at least one relation"},
		{"unknown edge", func(s *Sankey) { s.Edges = []string{"nope"} }, `unknown relation type "nope"`},
		{"duplicate edge", func(s *Sankey) { s.Edges = []string{"rel", "rel"} }, "duplicates"},
		{"type not in columns", func(s *Sankey) {
			s.Types = map[string]SankeyType{"zzz": {}}
		}, "not in columns"},
		{"bad label", func(s *Sankey) {
			s.Types = map[string]SankeyType{"a": {Label: "nope"}}
		}, `label property "nope"`},
		{"bad order", func(s *Sankey) {
			s.Types = map[string]SankeyType{"a": {Order: "nope"}}
		}, `order property "nope"`},
		{"order id", func(s *Sankey) { s.Types = map[string]SankeyType{"a": {Order: "id"}} }, ""},
		{"bad where", func(s *Sankey) {
			s.Types = map[string]SankeyType{"a": {Where: []string{"status ??"}}}
		}, "where[0]"},
		{"bad color", func(s *Sankey) { s.Types = map[string]SankeyType{"a": {Color: "pink"}} }, "unknown color"},
		{"negative max", func(s *Sankey) { s.MaxNodes = -1 }, "max_nodes"},
		{"pips valid", func(s *Sankey) {
			s.Types = map[string]SankeyType{"a": {Pips: &SankeyPips{Relation: "ev", Type: "c", ColorBy: "status"}}}
		}, ""},
		{"pips valid reversed", func(s *Sankey) {
			s.Types = map[string]SankeyType{"b": {Pips: &SankeyPips{Relation: "rel", Type: "a"}}}
		}, ""},
		{"pips unknown relation", func(s *Sankey) {
			s.Types = map[string]SankeyType{"a": {Pips: &SankeyPips{Relation: "nope", Type: "c"}}}
		}, `relation "nope" is not in the metamodel`},
		{"pips unknown type", func(s *Sankey) {
			s.Types = map[string]SankeyType{"a": {Pips: &SankeyPips{Relation: "ev", Type: "zzz"}}}
		}, `type "zzz" is not in the metamodel`},
		{"pips same type", func(s *Sankey) {
			s.Types = map[string]SankeyType{"a": {Pips: &SankeyPips{Relation: "ev", Type: "a"}}}
		}, "must differ"},
		{"pips relation does not join", func(s *Sankey) {
			s.Types = map[string]SankeyType{"b": {Pips: &SankeyPips{Relation: "ev", Type: "c"}}}
		}, "does not join"},
		{"pips max too big", func(s *Sankey) {
			s.Types = map[string]SankeyType{"a": {Pips: &SankeyPips{Relation: "ev", Type: "c", Max: 51}}}
		}, "max must be between"},
		{"pips bad color_by", func(s *Sankey) {
			s.Types = map[string]SankeyType{"a": {Pips: &SankeyPips{Relation: "ev", Type: "c", ColorBy: "nope"}}}
		}, `color_by property "nope"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := good
			tc.mutate(&s)
			errs := validateSankey("x", s, sankeyMeta())
			if tc.want == "" {
				if len(errs) != 0 {
					t.Fatalf("unexpected errors: %v", errs)
				}
				return
			}
			if len(errs) == 0 || !strings.Contains(strings.Join(errs, "\n"), tc.want) {
				t.Fatalf("errors = %v, want one containing %q", errs, tc.want)
			}
		})
	}
}

func TestNormalizeSankeys(t *testing.T) {
	cfg := &Config{Sankeys: map[string]Sankey{"flow": {}, "other": {Title: "T", MaxNodes: 5}}}
	NormalizeSankeys(cfg)
	if s := cfg.Sankeys["flow"]; s.MaxNodes != defaultSankeyMaxNodes || s.Title != "flow" {
		t.Errorf("flow = %+v", s)
	}
	if s := cfg.Sankeys["other"]; s.MaxNodes != 5 || s.Title != "T" {
		t.Errorf("other = %+v", s)
	}
}

func TestNormalizeSankeysDefaultsPipsMax(t *testing.T) {
	cfg := &Config{Sankeys: map[string]Sankey{"flow": {Types: map[string]SankeyType{
		"a": {Pips: &SankeyPips{Relation: "ev", Type: "c"}},
		"b": {Pips: &SankeyPips{Relation: "ev", Type: "c", Max: 3}},
	}}}}
	NormalizeSankeys(cfg)
	if got := cfg.Sankeys["flow"].Types["a"].Pips.Max; got != defaultSankeyPipsMax {
		t.Errorf("default max = %d", got)
	}
	if got := cfg.Sankeys["flow"].Types["b"].Pips.Max; got != 3 {
		t.Errorf("explicit max = %d", got)
	}
}
