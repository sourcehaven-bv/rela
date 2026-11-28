package affordances_test

import (
	"maps"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// rollupMeta is testMeta with one roll-up property per roll-up form on
// ticket.
func rollupMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m := testMeta(t)
	ticket := m.Entities["ticket"]
	props := map[string]metamodel.PropertyDef{
		"blocker_count": {Type: metamodel.PropertyTypeInteger, Rollup: &metamodel.RollupDef{
			Via: metamodel.StringOrSlice{"blocks"}, Count: true,
		}},
		"has_blocker": {Type: metamodel.PropertyTypeBoolean, Rollup: &metamodel.RollupDef{
			Via: metamodel.StringOrSlice{"blocks"}, Exists: true,
		}},
		"first_due": {Type: metamodel.PropertyTypeDate, Rollup: &metamodel.RollupDef{
			Via: metamodel.StringOrSlice{"blocks"}, Min: "due",
		}},
		"last_due": {Type: metamodel.PropertyTypeDate, Rollup: &metamodel.RollupDef{
			Via: metamodel.StringOrSlice{"blocks"}, Max: "due",
		}},
		"latest_status": {
			Type: metamodel.PropertyTypeEnum, Values: []string{"open", "review", "done"},
			Rollup: &metamodel.RollupDef{Via: metamodel.StringOrSlice{"blocks"}, Pick: "status", LatestBy: "due"},
		},
		"logged": {Type: metamodel.PropertyTypeString, Rollup: &metamodel.RollupDef{
			Records: "worklog", LatestOf: "note",
		}},
		"tasks_open": {Type: metamodel.PropertyTypeInteger, Rollup: &metamodel.RollupDef{BodyTasks: "open"}},
	}
	merged := maps.Clone(ticket.Properties)
	if merged == nil {
		merged = map[string]metamodel.PropertyDef{}
	}
	maps.Copy(merged, props)
	ticket.Properties = merged
	m.Entities["ticket"] = ticket
	return m
}

// TestPolicyLoad_RefusesRollupInVisibleWhen: a `visible:` `when:` that reads
// a roll-up fails the policy load with an error naming the property, for
// every roll-up form. A string-shaped roll-up (a date min or max, an enum
// pick, a records latest) would otherwise lower as an equality on a stored
// property that is always unset.
func TestPolicyLoad_RefusesRollupInVisibleWhen(t *testing.T) {
	t.Parallel()
	cases := []struct {
		form, property, when string
	}{
		{"count", "blocker_count", "entity.blocker_count == 0"},
		{"exists", "has_blocker", "entity.has_blocker == true"},
		{"min date", "first_due", "entity.first_due == '2026-01-01'"},
		{"max date", "last_due", "entity.last_due == '2026-01-01'"},
		{"latest_by pick enum", "latest_status", "entity.latest_status == 'open'"},
		{"records", "logged", "entity.logged == 'x'"},
		{"body_tasks", "tasks_open", "entity.tasks_open == 0"},
	}
	for _, tc := range cases {
		t.Run(tc.form, func(t *testing.T) {
			t.Parallel()
			p := policyFromYAML(t, `
roles:
  triager:
    visible:
      ticket:
        - field: assignee
          when: `+quoteYAML(tc.when)+`
`)
			_, err := affordances.New(rollupMeta(t), newStubLookup(), declFor(t, p),
				affordances.WithTraversals(&stubBinder{}))
			if err == nil {
				t.Fatal("New: want a load error")
			}
			for _, part := range []string{"roles.triager.visible.ticket[0]", `"` + tc.property + `"`, "roll-up"} {
				if !strings.Contains(err.Error(), part) {
					t.Errorf("error %q does not name %s", err, part)
				}
			}
		})
	}
}
