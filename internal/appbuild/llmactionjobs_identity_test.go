package appbuild_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// patchRunner stands in for the data-entry runner: it writes status on the
// incident through the real entity manager, as the identity the job bound.
type patchRunner struct {
	svc  *appbuild.Services
	runs int
	err  error
}

func (r *patchRunner) RunLLMAction(ctx context.Context, _, _, _ string) (bool, error) {
	r.runs++
	_, r.err = r.svc.EntityManager().PatchEntity(ctx, "INC-001",
		entity.Patch{Properties: map[string]any{"status": "triaged"}})
	return false, r.err
}

// TestLLMActionJob_Identity pins that a background llm action binds the
// user who queued it as the data-entry edge bound them: a verified user
// who matched no user entity stays unmatched, so `unmatched_principal:
// reject` refuses the write, and a user whose entity was removed after the
// run was queued is not run at all.
func TestLLMActionJob_Identity(t *testing.T) {
	t.Run("unmatched user is refused", func(t *testing.T) {
		svc := identityServices(t)
		r := &patchRunner{svc: svc}
		run, err := appbuild.LLMActionJobRun(verifiedAs(t, svc, "alice"), svc, r, "triage", "INC-001")
		require.NoError(t, err)
		require.NoError(t, run(context.Background()), "a refusal ends the job")
		require.Equal(t, 1, r.runs)
		require.Error(t, r.err)
		require.Empty(t, incident(t, svc).Properties["status"], "nothing was written")
	})

	t.Run("matched user writes", func(t *testing.T) {
		svc := identityServices(t)
		r := &patchRunner{svc: svc}
		run, err := appbuild.LLMActionJobRun(verifiedAs(t, svc, "carol"), svc, r, "triage", "INC-001")
		require.NoError(t, err)
		require.NoError(t, run(context.Background()))
		require.NoError(t, r.err)
		require.Equal(t, "triaged", incident(t, svc).Properties["status"])

		require.NoError(t, run(context.Background()))
		require.Equal(t, 1, r.runs, "a redelivered run that finished does not run again")
	})

	t.Run("removed user is not run", func(t *testing.T) {
		svc := identityServices(t)
		r := &patchRunner{svc: svc}
		run, err := appbuild.LLMActionJobRun(verifiedAs(t, svc, "carol"), svc, r, "triage", "INC-001")
		require.NoError(t, err)
		_, err = svc.Store().DeleteFamily(context.Background(), "CAROL", true)
		require.NoError(t, err)
		require.NoError(t, run(context.Background()))
		require.Zero(t, r.runs)
		require.Empty(t, incident(t, svc).Properties["status"])
	})
}
