package flowengine

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
)

// TestFailedJobs_AgreesWithExplain pins BUG-S5VV0N0: the run list asks
// FailedJobs which runs read as failed, and it must answer as Explain
// does, for a run and for a forked run, before and after the job fails.
func TestFailedJobs_AgreesWithExplain(t *testing.T) {
	tests := []struct {
		name string
		src  string
		fail bool
	}{
		{name: "job queued", src: jobFlow},
		{name: "job failed", src: jobFlow, fail: true},
		{name: "fork, job queued", src: jobForkFlow},
		{name: "fork, job failed on a branch", src: jobForkFlow, fail: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeRunner{results: []error{Permanent(errors.New("410"))}}
			h, id := startJob(t, tc.src, r)
			ctx := context.Background()
			if tc.fail {
				require.NoError(t, jobSteps{h.eng}.HandleAttempt(ctx, h.q.last(t)))
			}
			run, err := h.eng.Run(ctx, id)
			require.NoError(t, err)
			xs, err := h.eng.Explain(ctx, run)
			require.NoError(t, err)
			require.Equal(t, tc.fail, xs[0].Status == flow.StatusFailed)
			require.Equal(t, tc.fail, h.eng.FlowCache().FailedJobs(ctx, []flowrun.Run{run})[id])
		})
	}
}
