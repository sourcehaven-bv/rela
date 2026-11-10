package flowrun_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
)

var t0 = time.Date(2026, 11, 7, 9, 0, 0, 123456789, time.UTC)

func validNew() flowrun.NewRun {
	return flowrun.NewRun{
		ID: "FR-1", Flow: "onboard", Version: "0123456789abcdef", Definition: []byte("d"),
		Subject: entity.Ref{ID: "EMP-1"}, At: t0,
		Instance: flow.Instance{State: "open", Generation: 1, EnteredAt: t0},
	}
}

func TestCheckNewRun(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*flowrun.NewRun)
		ok     bool
	}{
		{"valid", func(*flowrun.NewRun) {}, true},
		{"valid with face", func(nr *flowrun.NewRun) { nr.Subject.Face = "draft" }, true},
		{"empty id", func(nr *flowrun.NewRun) { nr.ID = "" }, false},
		{"long id", func(nr *flowrun.NewRun) { nr.ID = flowrun.RunID(strings.Repeat("a", flowrun.MaxIDLen+1)) }, false},
		{"bad flow", func(nr *flowrun.NewRun) { nr.Flow = "Bad Flow" }, false},
		{"short version", func(nr *flowrun.NewRun) { nr.Version = "abc" }, false},
		{"upper version", func(nr *flowrun.NewRun) { nr.Version = "0123456789ABCDEF" }, false},
		{"no definition", func(nr *flowrun.NewRun) { nr.Definition = nil }, false},
		{"huge definition", func(nr *flowrun.NewRun) {
			nr.Definition = make([]byte, flowrun.MaxDefinitionBytes+1)
		}, false},
		{"empty subject", func(nr *flowrun.NewRun) { nr.Subject.ID = "" }, false},
		{"long subject", func(nr *flowrun.NewRun) {
			nr.Subject.ID = strings.Repeat("a", flowrun.MaxIDLen+1)
		}, false},
		{"bad face", func(nr *flowrun.NewRun) { nr.Subject.Face = "Bad Face!" }, false},
		{"zero time", func(nr *flowrun.NewRun) { nr.At = time.Time{} }, false},
		{"epoch time", func(nr *flowrun.NewRun) { nr.At = time.Unix(0, 0) }, false},
		{"due at epoch", func(nr *flowrun.NewRun) { nr.Instance.NextDueAt = time.Unix(0, 0) }, false},
		{"due before epoch", func(nr *flowrun.NewRun) { nr.Instance.NextDueAt = time.Unix(-5, 0) }, false},
		{"bad state", func(nr *flowrun.NewRun) { nr.Instance.State = "Bad" }, false},
		{"bad previous", func(nr *flowrun.NewRun) { nr.Instance.Previous = "Bad" }, false},
		{"negative generation", func(nr *flowrun.NewRun) { nr.Instance.Generation = -1 }, false},
		{"negative cause", func(nr *flowrun.NewRun) { nr.Instance.Cause = -1 }, false},
		{"bad timer", func(nr *flowrun.NewRun) { nr.Instance.Restarts = map[string]time.Time{"Bad": t0} }, false},
		{"too many timers", func(nr *flowrun.NewRun) {
			nr.Instance.Restarts = map[string]time.Time{}
			for i := range flow.MaxTimers + 1 {
				nr.Instance.Restarts["t"+strings.Repeat("x", i)] = t0
			}
		}, false},
		{"bad reminder", func(nr *flowrun.NewRun) {
			nr.Instance.Reminders = map[string]flow.ReminderRecord{"Bad": {}}
		}, false},
		{"negative reminder", func(nr *flowrun.NewRun) {
			nr.Instance.Reminders = map[string]flow.ReminderRecord{"r": {Count: -1}}
		}, false},
		{"too many reminders", func(nr *flowrun.NewRun) {
			nr.Instance.Reminders = map[string]flow.ReminderRecord{}
			for i := range flow.MaxReminders + 1 {
				nr.Instance.Reminders["r"+strings.Repeat("x", i)] = flow.ReminderRecord{}
			}
		}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nr := validNew()
			tc.mutate(&nr)
			err := flowrun.CheckNewRun(nr)
			if tc.ok {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, flowrun.ErrInvalid)
		})
	}
}

func TestCheckArguments(t *testing.T) {
	inst := flow.Instance{State: "open"}
	tests := []struct {
		name string
		err  error
		ok   bool
	}{
		{"update", flowrun.CheckUpdate("FR-1", 1, flowrun.State{Instance: inst}, t0), true},
		{"update bad id", flowrun.CheckUpdate("", 1, flowrun.State{Instance: inst}, t0), false},
		{"update rev 0", flowrun.CheckUpdate("FR-1", 0, flowrun.State{Instance: inst}, t0), false},
		{"update zero time", flowrun.CheckUpdate("FR-1", 1, flowrun.State{Instance: inst}, time.Time{}), false},
		{"update epoch time", flowrun.CheckUpdate("FR-1", 1, flowrun.State{Instance: inst}, time.Unix(0, 0)), false},
		{"update pending", flowrun.CheckUpdate("FR-1", 1, flowrun.State{Instance: inst,
			Pending: &flowrun.Pending{State: "open", Generation: 1, Done: 2}}, t0), true},
		{"update pending bad state", flowrun.CheckUpdate("FR-1", 1, flowrun.State{Instance: inst,
			Pending: &flowrun.Pending{State: "a b"}}, t0), false},
		{"update pending negative", flowrun.CheckUpdate("FR-1", 1, flowrun.State{Instance: inst,
			Pending: &flowrun.Pending{State: "open", Done: -1}}, t0), false},
		{"update pending over cap", flowrun.CheckUpdate("FR-1", 1, flowrun.State{Instance: inst,
			Pending: &flowrun.Pending{State: "open", Done: flowrun.MaxPendingDone + 1}}, t0), false},
		{"update pending negative generation", flowrun.CheckUpdate("FR-1", 1, flowrun.State{Instance: inst,
			Pending: &flowrun.Pending{State: "open", Generation: -1}}, t0), false},
		{"update bad instance", flowrun.CheckUpdate("FR-1", 1, flowrun.State{Instance: flow.Instance{State: "X"}}, t0), false},
		{"delete", flowrun.CheckDelete("FR-1", 1), true},
		{"delete bad id", flowrun.CheckDelete("", 1), false},
		{"delete rev 0", flowrun.CheckDelete("FR-1", 0), false},
		{"due", flowrun.CheckDue(t0, flowrun.DueCursor{}, 1), true},
		{"due cursor", flowrun.CheckDue(t0, flowrun.DueCursor{At: t0, ID: "FR-1"}, flowrun.MaxPage), true},
		{"due zero until", flowrun.CheckDue(time.Time{}, flowrun.DueCursor{}, 1), false},
		{"due bad cursor", flowrun.CheckDue(t0, flowrun.DueCursor{ID: "bad id"}, 1), false},
		{"due limit 0", flowrun.CheckDue(t0, flowrun.DueCursor{}, 0), false},
		{"due limit high", flowrun.CheckDue(t0, flowrun.DueCursor{}, flowrun.MaxPage+1), false},
		{"query", flowrun.CheckQuery(flowrun.Query{Flow: "onboard", SubjectID: "EMP-1", After: "FR-1", Limit: 1}), true},
		{"query bad flow", flowrun.CheckQuery(flowrun.Query{Flow: "X", Limit: 1}), false},
		{"query bad subject", flowrun.CheckQuery(flowrun.Query{SubjectID: "bad id", Limit: 1}), false},
		{"query bad after", flowrun.CheckQuery(flowrun.Query{After: "bad id", Limit: 1}), false},
		{"query no limit", flowrun.CheckQuery(flowrun.Query{}), false},
		{"rekey", flowrun.CheckRekey("EMP-1", "EMP-2"), true},
		{"rekey bad old", flowrun.CheckRekey("", "EMP-2"), false},
		{"rekey bad new", flowrun.CheckRekey("EMP-1", ""), false},
		{"definition key", flowrun.CheckDefinitionKey("onboard", "0123456789abcdef"), true},
		{"definition bad flow", flowrun.CheckDefinitionKey("X", "0123456789abcdef"), false},
		{"definition bad version", flowrun.CheckDefinitionKey("onboard", "x"), false},
		{"prune", flowrun.CheckPrune(t0), true},
		{"prune zero", flowrun.CheckPrune(time.Time{}), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.ok {
				require.NoError(t, tc.err)
				return
			}
			require.ErrorIs(t, tc.err, flowrun.ErrInvalid)
		})
	}
}

// TestStateRoundTrip: the stored envelope reads back normalized, with
// empty maps as nil, and a damaged one is an error.
func TestStateRoundTrip(t *testing.T) {
	inst := flow.Instance{
		State: "open", Generation: 2, EnteredAt: t0, NextDueAt: t0.Add(time.Hour), Cause: 3, Previous: "new",
		Restarts:  map[string]time.Time{"nudge": t0},
		Reminders: map[string]flow.ReminderRecord{"ping": {Since: t0, Last: t0, Count: 1}},
	}
	data, err := flowrun.EncodeState(flowrun.State{Instance: inst})
	require.NoError(t, err)
	require.Contains(t, string(data), `"instance"`)
	got, err := flowrun.DecodeState(data)
	require.NoError(t, err)
	require.Equal(t, flowrun.NormInstance(inst), got.Instance)
	require.Nil(t, got.Pending)
	require.NotContains(t, string(data), `"pending"`)
	require.Equal(t, flow.NormTime(t0), got.Instance.EnteredAt)

	mark := &flowrun.Pending{State: "open", Generation: 2, Done: 1}
	data, err = flowrun.EncodeState(flowrun.State{Instance: inst, Pending: mark})
	require.NoError(t, err)
	got, err = flowrun.DecodeState(data)
	require.NoError(t, err)
	require.Equal(t, *mark, *got.Pending)
	require.NotSame(t, mark, got.Pending)

	empty := flowrun.NormInstance(flow.Instance{Restarts: map[string]time.Time{},
		Reminders: map[string]flow.ReminderRecord{}})
	require.Nil(t, empty.Restarts)
	require.Nil(t, empty.Reminders)

	_, err = flowrun.DecodeState([]byte("oops"))
	require.Error(t, err)
}

func TestFromNew(t *testing.T) {
	r := flowrun.FromNew(validNew())
	require.Equal(t, int64(1), r.Rev)
	require.Equal(t, flow.NormTime(t0), r.CreatedAt)
	require.Equal(t, r.CreatedAt, r.UpdatedAt)
	require.Equal(t, flowrun.RunID("FR-1"), r.ID)
}

func TestDigest(t *testing.T) {
	require.Len(t, flowrun.Digest([]byte("d")), 64)
	require.NotEqual(t, flowrun.Digest([]byte("a")), flowrun.Digest([]byte("b")))
}

func TestDueOrder(t *testing.T) {
	at := flow.NormTime(t0)
	runs := []flowrun.Run{
		{ID: "B", Instance: flow.Instance{NextDueAt: at.Add(time.Second)}},
		{ID: "C", Instance: flow.Instance{NextDueAt: at}},
		{ID: "A", Instance: flow.Instance{NextDueAt: at}},
	}
	flowrun.SortDue(runs)
	require.Equal(t, []flowrun.RunID{"A", "C", "B"}, []flowrun.RunID{runs[0].ID, runs[1].ID, runs[2].ID})

	require.True(t, flowrun.DueCursor{}.Before(at, "A"))
	c := flowrun.DueCursor{At: t0, ID: "B"}
	require.True(t, c.Before(at, "C"))
	require.False(t, c.Before(at, "A"))
	require.False(t, c.Before(at, "B"))
	require.True(t, c.Before(at.Add(time.Microsecond), "A"))
	require.False(t, c.Before(at.Add(-time.Microsecond), "Z"))
}
