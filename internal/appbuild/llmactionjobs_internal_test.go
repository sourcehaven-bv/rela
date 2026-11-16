package appbuild

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

func TestFlowActionUsable_LLM(t *testing.T) {
	llm := func(apply string) dataentryconfig.Action {
		return dataentryconfig.Action{
			LLM:         &dataentryconfig.LLMAction{Prompt: "p", Apply: apply},
			AvailableOn: &dataentryconfig.ActionScope{EntityTypes: []string{"ticket"}},
		}
	}
	require.NoError(t, flowActionUsable("a", llm(dataentryconfig.LLMApplyAuto), ""))
	require.NoError(t, flowActionUsable("a", llm(dataentryconfig.LLMApplyAuto), "ticket"))
	require.Error(t, flowActionUsable("a", llm(dataentryconfig.LLMApplyAuto), "person"))
	require.Error(t, flowActionUsable("a", llm(""), ""))
}

type fakeLLMRunner struct {
	user  string
	world string
	who   principal.Principal
	runs  int
	retry bool
	err   error
}

func (f *fakeLLMRunner) RunLLMAction(ctx context.Context, _, _, world string) (bool, error) {
	f.world = world
	f.who = principal.From(ctx)
	f.user = f.who.User
	f.runs++
	return f.retry, f.err
}

func TestHandleLLMActionJob(t *testing.T) {
	bind := func(ctx context.Context) (context.Context, error) { return ctx, nil }
	job := func(attempt int) jobs.Job {
		return jobs.Job{Kind: llmActionJobKind, Retry: jobs.RetryBounded, Attempt: attempt,
			Payload: map[string]any{"action": "triage", "entity": "TKT-1", "user": "alice"}}
	}
	run := func(r *fakeLLMRunner, j jobs.Job) error {
		h := &llmActionHolder{}
		h.r.Store(&llmActionRunnerBox{r})
		return handleLLMActionJob(context.Background(), j, h, bind, &mapKV{m: map[string][]byte{}})
	}

	ok := &fakeLLMRunner{}
	require.NoError(t, run(ok, job(1)))
	require.Equal(t, "alice", ok.user)

	boom := errors.New("provider down")
	require.ErrorIs(t, run(&fakeLLMRunner{retry: true, err: boom}, job(1)), boom)
	require.NoError(t, run(&fakeLLMRunner{retry: true, err: boom}, job(1000)), "the final attempt ends the job")
	require.NoError(t, run(&fakeLLMRunner{err: boom}, job(1)), "a refusal is not retried")

	bad := job(1)
	bad.Payload["user"] = ""
	r := &fakeLLMRunner{}
	require.NoError(t, run(r, bad))
	require.Empty(t, r.user, "an unusable payload does not run")

	// No runner installed yet: retryable.
	require.Error(t, handleLLMActionJob(context.Background(), job(1), &llmActionHolder{}, bind, &mapKV{m: map[string][]byte{}}))

	// A bind failure is returned for a retry.
	failBind := func(context.Context) (context.Context, error) { return nil, boom }
	require.Error(t, handleLLMActionJob(context.Background(), job(1), &llmActionHolder{}, failBind, &mapKV{m: map[string][]byte{}}))
}

func TestEnqueueLLMAction_NoQueue(t *testing.T) {
	var j *LLMActionJobs
	require.Error(t, j.EnqueueLLMAction(context.Background(), "a", "TKT-1", "", principal.Principal{User: "alice"}))
}

// The job carries the client's ceiling, so the background run is no wider
// than the request that queued it, and a redelivered run that already
// applied does not run again.
func TestLLMActionJob_CeilingAndRedelivery(t *testing.T) {
	q := &dupQueue{}
	who := principal.VerifiedFrom("alice", principal.ToolDataEntry,
		principal.Claims{PrincipalType: "client", Scopes: []string{"write", "read"}})
	require.NoError(t, (&LLMActionJobs{q: q}).EnqueueLLMAction(context.Background(), "triage", "TKT-1", "published", who))
	require.NoError(t, (&LLMActionJobs{q: q}).EnqueueLLMAction(context.Background(), "triage", "TKT-1", "published", who))
	require.Len(t, q.jobs, 2)
	require.Equal(t, q.jobs[0].IdempotencyKey, q.jobs[1].IdempotencyKey, "one pending run per action, entity and caller")
	require.NotEqual(t, q.jobs[0].Payload["run"], q.jobs[1].Payload["run"], "each enqueue is a run of its own")

	r := &fakeLLMRunner{}
	h := &llmActionHolder{}
	h.r.Store(&llmActionRunnerBox{r})
	kv := &mapKV{m: map[string][]byte{}}
	bind := func(ctx context.Context) (context.Context, error) { return ctx, nil }
	job := q.jobs[0]
	job.Retry, job.Attempt = jobs.RetryBounded, 1
	require.NoError(t, handleLLMActionJob(context.Background(), job, h, bind, kv))
	require.Equal(t, "client", r.who.PrincipalType())
	require.ElementsMatch(t, []string{"read", "write"}, r.who.Scopes())
	require.Equal(t, principal.ToolLLMAction, r.who.Tool)
	require.Equal(t, "published", r.world, "the run reads in the world it was queued from")

	require.NoError(t, handleLLMActionJob(context.Background(), job, h, bind, kv))
	require.Equal(t, 1, r.runs, "a redelivered run that applied does not run again")

	next := q.jobs[1]
	next.Retry, next.Attempt = jobs.RetryBounded, 1
	require.NoError(t, handleLLMActionJob(context.Background(), next, h, bind, kv))
	require.Equal(t, 2, r.runs, "a later run of the same key runs")

	require.Error(t, registerLLMActionJobs(q, h, bind, nil), "state is required")
}

func TestLLMActionPayloadKey_NoSeparatorCollision(t *testing.T) {
	a := llmActionPayload{Action: "a:b", Entity: "c"}
	b := llmActionPayload{Action: "a", Entity: "b:c"}
	require.NotEqual(t, a.key(), b.key())
	require.NotEqual(t, a.handledKey(), b.handledKey())
}
