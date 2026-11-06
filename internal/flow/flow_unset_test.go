package flow_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/flow"
)

// TestStepSchedule_UnsetKeepsNilVersusEmpty pins the one shape change of
// the extraction: [flow.Read] carries the stored reminder records of an
// unset instance, and a step without a move must keep nil (keep the stored
// records) apart from empty (store none).
func TestStepSchedule_UnsetKeepsNilVersusEmpty(t *testing.T) {
	t.Parallel()
	d := review(t)

	unset := flow.Read(d, flow.Instance{}, t0)
	require.Nil(t, unset.Reminders)
	require.Nil(t, flow.StepSchedule(d, unset, unset).Reminders, "nil keeps the stored records")

	empty := flow.Read(d, flow.Instance{Reminders: map[string]flow.ReminderRecord{}}, t0)
	sch := flow.StepSchedule(d, empty, empty)
	require.NotNil(t, sch.Reminders, "empty stays empty: the step stores none")
	require.Empty(t, sch.Reminders)

	posted := map[string]flow.ReminderRecord{"nudge": {Since: t0, Last: t0, Count: 1}}
	withPost := flow.Read(d, flow.Instance{Reminders: posted}, t0)
	require.Equal(t, posted, flow.StepSchedule(d, withPost, withPost).Reminders,
		"a reminder posted before the first move keeps its record")
}

func TestNewDefinition_SortsByName(t *testing.T) {
	t.Parallel()
	b, a := flow.NewReminder("b", []string{"s"}), flow.NewReminder("a", []string{"s"})
	d := flow.NewDefinition("s", []flow.State{
		flow.NewState("s", false, []flow.Timer{{Name: "z", To: "s"}, {Name: "y", To: "s"}}),
	}, []flow.Reminder{b, a})
	require.Equal(t, "a", d.Reminders()[0].Name)
	st, _ := d.State("s")
	require.Equal(t, "y", st.Timers()[0].Name)
}
