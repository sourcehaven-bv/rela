package conditionlint

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// rollupVCMeta is vcMeta with a count roll-up and a body_tasks roll-up on
// taak.
func rollupVCMeta() *metamodel.Metamodel {
	meta := vcMeta()
	taak := meta.Entities["taak"]
	taak.Properties["blockers"] = metamodel.PropertyDef{
		Type:   metamodel.PropertyTypeInteger,
		Rollup: &metamodel.RollupDef{Via: metamodel.StringOrSlice{"blocks"}, Count: true},
	}
	taak.Properties["open_tasks"] = metamodel.PropertyDef{
		Type:   metamodel.PropertyTypeInteger,
		Rollup: &metamodel.RollupDef{BodyTasks: "open"},
	}
	meta.Entities["taak"] = taak
	return meta
}

// TestCompileViewConditions_ListRollups pins that only a list `condition:`
// may read a roll-up of its type, and never a body_tasks one: a list
// condition is judged over every candidate, and a body is never loaded for a
// candidate.
func TestCompileViewConditions_ListRollups(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *dataentryconfig.Config
		wantProb string
	}{
		{
			name: "a list condition reads a roll-up",
			cfg: &dataentryconfig.Config{Lists: map[string]dataentryconfig.List{
				"free": {EntityType: "taak", Condition: "entity.blockers == 0"},
			}},
		},
		{
			name: "a list condition over body_tasks is refused, naming it",
			cfg: &dataentryconfig.Config{Lists: map[string]dataentryconfig.List{
				"todo": {EntityType: "taak", Condition: "entity.open_tasks > 0"},
			}},
			wantProb: `roll-up "open_tasks" counts body tasks`,
		},
		{
			name: "a kanban condition still cannot read a roll-up",
			cfg: &dataentryconfig.Config{Kanbans: map[string]dataentryconfig.Kanban{
				"board": {EntityType: "taak", Condition: "entity.blockers == 0"},
			}},
			wantProb: `kanbans["board"]: condition does not compile`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			progs, problems := CompileViewConditions(tc.cfg, rollupVCMeta())
			if tc.wantProb != "" {
				require.NotEmpty(t, problems)
				require.Contains(t, problems[0], tc.wantProb)
				return
			}
			require.Empty(t, problems)
			require.Len(t, progs, 1)
		})
	}
}
