package metamodel

import "testing"

func TestRankingTitleProperty(t *testing.T) {
	str := PropertyDef{Type: PropertyTypeString, Required: true}
	for name, tc := range map[string]struct {
		def  EntityDef
		want string
	}{
		"declared display property":  {EntityDef{DisplayProperty: "summary", Properties: map[string]PropertyDef{"title": str}}, "summary"},
		"template names no property": {EntityDef{DisplayProperty: "{first} {last}", Properties: map[string]PropertyDef{"title": str}}, ""},
		"conventional title":         {EntityDef{Properties: map[string]PropertyDef{"title": str, "name": str}}, "title"},
		"conventional name":          {EntityDef{Properties: map[string]PropertyDef{"name": str}}, "name"},
		"optional title is skipped":  {EntityDef{Properties: map[string]PropertyDef{"title": {Type: PropertyTypeString}}}, ""},
		"arbitrary required string":  {EntityDef{Properties: map[string]PropertyDef{"code": str}}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			if got := tc.def.RankingTitleProperty(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
