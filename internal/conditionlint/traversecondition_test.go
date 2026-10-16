package conditionlint

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// tcMeta has a relation from project to tasks and sub-projects, and a note
// type without a status.
func tcMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"project": {Properties: map[string]metamodel.PropertyDef{"status": {Type: metamodel.PropertyTypeString}}},
			"taak":    {Properties: map[string]metamodel.PropertyDef{"status": {Type: metamodel.PropertyTypeString}}},
			"note":    {Properties: map[string]metamodel.PropertyDef{"title": {Type: metamodel.PropertyTypeString}}},
		},
		Relations: map[string]metamodel.RelationDef{
			"bestaat_uit": {From: []string{"project"}, To: []string{"project", "taak", "note"}},
		},
	}
}

func traverseCfg(t dataentryconfig.ViewTraverse) *dataentryconfig.Config {
	return &dataentryconfig.Config{Views: map[string]dataentryconfig.ViewConfig{
		"pv": {Entry: dataentryconfig.ViewEntry{Type: "project"}, Traverse: []dataentryconfig.ViewTraverse{t}},
	}}
}

func TestCompileViewConditions_Traverse(t *testing.T) {
	step := dataentryconfig.ViewTraverse{From: "entry", Follow: "bestaat_uit", CollectAs: "werk"}
	tests := []struct {
		name      string
		condition string
		wantProb  string
	}{
		{name: "only finished tasks", condition: "entity.type == 'taak' and entity.status == 'gereed'"},
		{name: "unfinished tasks and projects",
			condition: "entity.type == 'project' or (entity.type == 'taak' and entity.status ~= 'gereed')"},
		{name: "a typo in a property is a problem naming the step", condition: "entity.stauts == 'gereed'",
			wantProb: `views["pv"].traverse[0]: condition does not compile against entity type`},
		{name: "the filter dialect is refused", condition: "status = gereed",
			wantProb: "condition does not compile"},
		{name: "related() is refused", condition: "related(entity, 'bestaat_uit')",
			wantProb: "related(...) is not available in a traverse condition"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := step
			s.Condition = tc.condition
			progs, problems := CompileViewConditions(traverseCfg(s), tcMeta())
			if tc.wantProb != "" {
				require.NotEmpty(t, problems)
				require.Contains(t, problems[0], tc.wantProb)
				return
			}
			require.Empty(t, problems)
			types := []string{"project", "taak", "note"}
			require.Len(t, progs, len(types), "one program per collected type")
			for _, et := range types {
				require.Contains(t, progs, ViewConditionKey{ViewConditionTraverse,
					dataentryconfig.TraverseConditionID(et, types, tc.condition)})
			}
		})
	}
}

// The matcher lookup recovers each program's entity type from its key.
func TestViewConditionMatchers_TraverseEntityType(t *testing.T) {
	cond := "entity.type == 'taak'"
	cfg := traverseCfg(dataentryconfig.ViewTraverse{From: "entry", Follow: "bestaat_uit", CollectAs: "werk", Condition: cond})
	lookup, problems := ViewConditionMatchers(cfg, tcMeta())
	require.Empty(t, problems)
	types := []string{"note", "project", "taak"}
	m, ok := lookup(string(ViewConditionTraverse), dataentryconfig.TraverseConditionID("taak", types, cond))
	require.True(t, ok)
	require.Equal(t, "taak", m.EntityType())
}

// A where: on type narrows the types the condition compiles for, so an
// ordered comparison on a property the excluded type lacks is accepted.
func TestCompileViewConditions_TraverseWhereNarrowsTypes(t *testing.T) {
	step := dataentryconfig.ViewTraverse{From: "entry", Follow: "bestaat_uit", CollectAs: "werk",
		Condition: "entity.status < 'b'"}
	_, problems := CompileViewConditions(traverseCfg(step), tcMeta())
	require.NotEmpty(t, problems, "note has no status, so the comparison cannot compile for it")

	step.Where = "type != note"
	progs, problems := CompileViewConditions(traverseCfg(step), tcMeta())
	require.Empty(t, problems)
	require.Len(t, progs, 2)
}
