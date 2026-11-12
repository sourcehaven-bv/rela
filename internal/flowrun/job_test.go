package flowrun_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/flowrun"
)

func TestCheckState_JobAttempt(t *testing.T) {
	at := func(j flowrun.JobAttempt) flowrun.State {
		return flowrun.State{Pending: &flowrun.Pending{State: "send", Generation: 1, Job: &j}}
	}
	tests := []struct {
		name string
		j    flowrun.JobAttempt
		ok   bool
	}{
		{"waiting", flowrun.JobAttempt{Phase: flowrun.JobWaiting, NextAt: t0}, true},
		{"queued", flowrun.JobAttempt{Seq: 1, Phase: flowrun.JobQueued, QueuedAt: t0, Deadline: t0}, true},
		{"queued without deadline", flowrun.JobAttempt{Seq: 1, Phase: flowrun.JobQueued}, false},
		{"queued without seq", flowrun.JobAttempt{Phase: flowrun.JobQueued, Deadline: t0}, false},
		{"started", flowrun.JobAttempt{Seq: 1, Phase: flowrun.JobStarted, Deadline: t0, StartedAt: t0}, true},
		{"started without start", flowrun.JobAttempt{Seq: 1, Phase: flowrun.JobStarted, Deadline: t0}, false},
		{"finished", flowrun.JobAttempt{Seq: 1, Phase: flowrun.JobFinished, Outcome: flowrun.JobTimeout,
			StartedAt: t0}, true},
		{"finished without start", flowrun.JobAttempt{Seq: 1, Phase: flowrun.JobFinished,
			Outcome: flowrun.JobTimeout}, false},
		{"finished without outcome", flowrun.JobAttempt{Seq: 1, Phase: flowrun.JobFinished, StartedAt: t0}, false},
		{"finished bad outcome", flowrun.JobAttempt{Seq: 1, Phase: flowrun.JobFinished, Outcome: "maybe",
			StartedAt: t0}, false},
		{"failed", flowrun.JobAttempt{Seq: 2, Failed: 2, Phase: flowrun.JobFailed, FailKind: flowrun.JobLost}, true},
		{"failed without kind", flowrun.JobAttempt{Seq: 2, Phase: flowrun.JobFailed}, false},
		{"waiting with outcome", flowrun.JobAttempt{Phase: flowrun.JobWaiting, Outcome: flowrun.JobOK}, false},
		{"unknown phase", flowrun.JobAttempt{Phase: "sleeping"}, false},
		{"negative failed", flowrun.JobAttempt{Phase: flowrun.JobWaiting, Failed: -1}, false},
		{"seq too large", flowrun.JobAttempt{Phase: flowrun.JobWaiting, Seq: flowrun.MaxJobSeq + 1}, false},
		{"too many unsent", flowrun.JobAttempt{Phase: flowrun.JobWaiting,
			Unsent: flowrun.MaxPendingFailures + 1}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := flowrun.CheckState(at(tc.j))
			require.Equal(t, tc.ok, err == nil, "err = %v", err)
		})
	}
}
