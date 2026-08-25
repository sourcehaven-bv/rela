package conditionlint

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// naMeta declares two types sharing `due` (so a cross-type condition is
// legitimate) and one that lacks it (so the every-type rule can be tested).
func naMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"task": {Properties: map[string]metamodel.PropertyDef{
				"due":    {Type: metamodel.PropertyTypeDate},
				"status": {Type: metamodel.PropertyTypeString},
			}},
			"bug": {Properties: map[string]metamodel.PropertyDef{
				"due": {Type: metamodel.PropertyTypeDate},
			}},
			"quip": {Properties: map[string]metamodel.PropertyDef{
				"text": {Type: metamodel.PropertyTypeString},
			}},
		},
	}
}

func naCfg(src dataentryconfig.NextActionSource) *dataentryconfig.Config {
	return &dataentryconfig.Config{
		NextActionBands: []dataentryconfig.NextActionBand{{ID: "stalled"}},
		NextActions:     map[string]dataentryconfig.NextActionSource{"s": src},
	}
}

func TestCompileNextActions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		src     dataentryconfig.NextActionSource
		wantErr string
		types   []string
	}{
		{
			name:  "date arithmetic compiles — the motivating S1/S4 case",
			src:   dataentryconfig.NextActionSource{Query: "type:task", Condition: "days_between(entity.due, today()) <= 7"},
			types: []string{"task"},
		},
		{
			name:  "compiles against every type the query names",
			src:   dataentryconfig.NextActionSource{Query: "type:task type:bug", Condition: "days_between(entity.due, today()) <= 7"},
			types: []string{"task", "bug"},
		},
		{
			name:  "a context source compiles against its declared type",
			src:   dataentryconfig.NextActionSource{Context: "task", Condition: "entity.status == 'todo'"},
			types: []string{"task"},
		},
		{
			// The every-type rule: valid on task, absent on quip. Accepting it
			// would silently drop every quip candidate.
			name:    "refused when it does not compile against one named type",
			src:     dataentryconfig.NextActionSource{Query: "type:task type:quip", Condition: "entity.status == 'todo'"},
			wantErr: `does not compile against entity type "quip"`,
		},
		{
			name:    "a typo'd property is a compile error, not a silent no-match",
			src:     dataentryconfig.NextActionSource{Query: "type:task", Condition: "entity.staus == 'todo'"},
			wantErr: "does not compile",
		},
		{
			name:    "a query naming no type has nothing to compile against",
			src:     dataentryconfig.NextActionSource{Query: "urgent", Condition: "entity.status == 'todo'"},
			wantErr: "requires the query to name at least one entity type",
		},
		{
			name:    "a count source has no entity to test",
			src:     dataentryconfig.NextActionSource{Count: "task == 0", Condition: "entity.status == 'todo'"},
			wantErr: "not supported on a count source",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, errs := CompileNextActions(naCfg(tc.src), naMeta())
			if tc.wantErr != "" {
				require.NotEmpty(t, errs)
				require.Contains(t, errs[0], tc.wantErr)
				require.Empty(t, got)
				return
			}
			require.Empty(t, errs)
			require.Len(t, got["s"], len(tc.types))
			for _, typ := range tc.types {
				require.Contains(t, got["s"], typ)
			}
		})
	}
}

// A source without a condition is absent from the result, which the engine
// reads as "keep every candidate".
func TestCompileNextActions_NoConditionIsAbsent(t *testing.T) {
	t.Parallel()
	got, errs := CompileNextActions(naCfg(dataentryconfig.NextActionSource{Query: "type:task"}), naMeta())
	require.Empty(t, errs)
	require.Empty(t, got)
}

func TestNextActionMatchers_EvaluatesTheCondition(t *testing.T) {
	t.Parallel()
	lookup, errs := NextActionMatchers(naCfg(dataentryconfig.NextActionSource{
		Query: "type:task", Condition: "entity.status == 'todo'",
	}), naMeta())
	require.Empty(t, errs)
	require.NotNil(t, lookup)

	m, ok := lookup("s")
	require.True(t, ok)

	match, err := m.Match(context.Background(), taskWith("T-1", "todo"))
	require.NoError(t, err)
	require.True(t, match)

	match, err = m.Match(context.Background(), taskWith("T-2", "done"))
	require.NoError(t, err)
	require.False(t, match)
}

// An entity whose type was never compiled must not match. Unreachable through
// the normal path, but refusing is the safe direction: a suggestion whose
// condition was never evaluated is worse than none.
func TestNextActionMatchers_UncompiledTypeDoesNotMatch(t *testing.T) {
	t.Parallel()
	lookup, errs := NextActionMatchers(naCfg(dataentryconfig.NextActionSource{
		Query: "type:task", Condition: "entity.status == 'todo'",
	}), naMeta())
	require.Empty(t, errs)

	m, _ := lookup("s")
	e := entity.New("Q-1", "quip")
	match, err := m.Match(context.Background(), e)
	require.NoError(t, err)
	require.False(t, match)
}

func TestNextActionMatchers_CompileErrorsPropagate(t *testing.T) {
	t.Parallel()
	lookup, errs := NextActionMatchers(naCfg(dataentryconfig.NextActionSource{
		Query: "type:task", Condition: "entity.nope == 1",
	}), naMeta())
	require.NotEmpty(t, errs)
	require.Nil(t, lookup)
}

func taskWith(id, status string) *entity.Entity {
	e := entity.New(id, "task")
	e.Properties["status"] = status
	return e
}
