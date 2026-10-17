package acl

import (
	"reflect"
	"testing"
)

// reachMeta is a MetamodelView over a fixed relation table.
type reachMeta struct {
	MetamodelView
	rels map[string]RelationInfo
}

func (m reachMeta) FaceNames(t string) (canonical string, faces []string) { return t, nil }

func (m reachMeta) RelationInfo(rel string) RelationInfo { return m.rels[rel] }

func TestRoleRelationsNoMemberHolds(t *testing.T) {
	meta := reachMeta{rels: map[string]RelationInfo{
		"member-of":   {Exists: true, From: []string{"person", "team"}, To: []string{"team"}},
		"in-unit":     {Exists: true, From: []string{"team"}, To: []string{"unit"}},
		"owns":        {Exists: true, From: []string{"person"}, To: []string{"doc"}},
		"team-owns":   {Exists: true, From: []string{"team"}, To: []string{"doc"}},
		"assigned-to": {Exists: true, From: []string{"asset"}, To: []string{"person"}},
		"any-owns":    {Exists: true, From: []string{"*"}, To: []string{"doc"}},
		"gates":       {Exists: true, From: []string{"asset"}, To: []string{"doc"}},
	}}
	p := &Policy{
		UserEntityType: "person",
		RoleRelations: map[string]RoleRelationDef{
			"owns":        {Confers: "owner"},
			"team-owns":   {Confers: "owner"},
			"assigned-to": {Confers: "assignee"},
			"any-owns":    {Confers: "owner"},
			"gates":       {RequiresPermission: "x"},
			"undeclared":  {Confers: "owner"},
		},
	}
	if got, want := RoleRelationsNoMemberHolds(p, meta), []string{"assigned-to"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	p.UserEntityType = ""
	if got := RoleRelationsNoMemberHolds(p, meta); got != nil {
		t.Errorf("without user_entity_type any entity may be a principal; got %v", got)
	}
}
