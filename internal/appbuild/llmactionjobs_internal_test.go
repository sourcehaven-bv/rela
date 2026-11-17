package appbuild

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
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

// fakeIdent stands in for [managerOutputs]: rebuild stamps the principal
// and fails with err.
type fakeIdent struct {
	mark realtime.OutputPoster
	got  realtime.OutputPoster
	err  error
}

func (f *fakeIdent) Poster(context.Context) (realtime.OutputPoster, error) { return f.mark, nil }

func (f *fakeIdent) rebuild(
	ctx context.Context, p principal.Principal, mark realtime.OutputPoster,
) (context.Context, error) {
	f.got = mark
	return principal.With(ctx, p), f.err
}

// queued enqueues one run and returns its job at attempt 1, with the
// state its marker went to.
func queued(t *testing.T, ident llmIdentity, who principal.Principal) (jobs.Job, *mapKV) {
	t.Helper()
	q := &dupQueue{}
	kv := &mapKV{m: map[string][]byte{}}
	require.NoError(t, (&LLMActionJobs{q: q, ident: ident, kv: kv}).EnqueueLLMAction(
		context.Background(), "triage", "TKT-1", "published", who))
	require.Len(t, q.jobs, 1)
	j := q.jobs[0]
	j.Retry, j.Attempt = jobs.RetryBounded, 1
	return j, kv
}

func TestHandleLLMActionJob(t *testing.T) {
	ident := &fakeIdent{}
	alice := principal.Principal{User: "alice"}
	run := func(r *fakeLLMRunner, j jobs.Job, kv *mapKV) error {
		h := &llmActionHolder{}
		h.r.Store(&llmActionRunnerBox{r})
		return handleLLMActionJob(context.Background(), j, h, ident, kv)
	}

	j, kv := queued(t, ident, alice)
	ok := &fakeLLMRunner{}
	require.NoError(t, run(ok, j, kv))
	require.Equal(t, "alice", ok.user)
	require.Empty(t, kv.m, "a finished run leaves no marker")

	boom := errors.New("provider down")
	j, kv = queued(t, ident, alice)
	require.ErrorIs(t, run(&fakeLLMRunner{retry: true, err: boom}, j, kv), boom)
	require.Len(t, kv.m, 1, "a run to retry keeps its marker")
	j.Attempt = 1000
	require.NoError(t, run(&fakeLLMRunner{retry: true, err: boom}, j, kv), "the final attempt ends the job")
	require.Empty(t, kv.m)

	j, kv = queued(t, ident, alice)
	require.NoError(t, run(&fakeLLMRunner{err: boom}, j, kv), "a refusal is not retried")
	require.Empty(t, kv.m)

	for _, k := range []string{"user", "run"} {
		bad, badKV := queued(t, ident, alice)
		bad.Payload[k] = ""
		r := &fakeLLMRunner{}
		require.NoError(t, run(r, bad, badKV))
		require.Zero(t, r.runs, "a payload without %s does not run", k)
	}

	// No runner installed yet: retryable.
	j, kv = queued(t, ident, alice)
	require.Error(t, handleLLMActionJob(context.Background(), j, &llmActionHolder{}, ident, kv))

	// An identity that no longer resolves ends the job without a run.
	gone := &fakeIdent{mark: realtime.OutputPoster{UserEntity: true}, err: errOutputsNoPoster}
	j, kv = queued(t, gone, alice)
	r := &fakeLLMRunner{}
	h := &llmActionHolder{}
	h.r.Store(&llmActionRunnerBox{r})
	require.NoError(t, handleLLMActionJob(context.Background(), j, h, gone, kv))
	require.Zero(t, r.runs)
	require.Equal(t, realtime.OutputPoster{UserEntity: true}, gone.got, "the mark rides the payload")
	require.Empty(t, kv.m)

	// Any other identity failure is returned for a retry.
	failing := &fakeIdent{err: boom}
	j, kv = queued(t, failing, alice)
	require.ErrorIs(t, handleLLMActionJob(context.Background(), j, h, failing, kv), boom)
}

func TestEnqueueLLMAction_NoQueue(t *testing.T) {
	var j *LLMActionJobs
	require.Error(t, j.EnqueueLLMAction(context.Background(), "a", "TKT-1", "", principal.Principal{User: "alice"}))
}

// A duplicate enqueue rides on the pending run and leaves no marker of its
// own.
func TestEnqueueLLMAction_DuplicateDropsMarker(t *testing.T) {
	q := &dupQueue{dups: 1}
	kv := &mapKV{m: map[string][]byte{}}
	require.NoError(t, (&LLMActionJobs{q: q, ident: &fakeIdent{}, kv: kv}).EnqueueLLMAction(
		context.Background(), "triage", "TKT-1", "", principal.Principal{User: "alice"}))
	require.Empty(t, q.jobs)
	require.Empty(t, kv.m)
}

// The job carries the client's ceiling, so the background run is no wider
// than the request that queued it, and a redelivered run that already
// finished does not run again.
func TestLLMActionJob_CeilingAndRedelivery(t *testing.T) {
	q := &dupQueue{}
	kv := &mapKV{m: map[string][]byte{}}
	ident := &fakeIdent{}
	who := principal.VerifiedFrom("alice", principal.ToolDataEntry,
		principal.Claims{PrincipalType: "client", Scopes: []string{"write", "read"}})
	j := &LLMActionJobs{q: q, ident: ident, kv: kv}
	require.NoError(t, j.EnqueueLLMAction(context.Background(), "triage", "TKT-1", "published", who))
	require.NoError(t, j.EnqueueLLMAction(context.Background(), "triage", "TKT-1", "published", who))
	require.Len(t, q.jobs, 2)
	require.Equal(t, q.jobs[0].IdempotencyKey, q.jobs[1].IdempotencyKey, "one pending run per action, entity and caller")
	require.NotEqual(t, q.jobs[0].Payload["run"], q.jobs[1].Payload["run"], "each enqueue is a run of its own")

	r := &fakeLLMRunner{}
	h := &llmActionHolder{}
	h.r.Store(&llmActionRunnerBox{r})
	job := q.jobs[0]
	job.Retry, job.Attempt = jobs.RetryBounded, 1
	require.NoError(t, handleLLMActionJob(context.Background(), job, h, ident, kv))
	require.Equal(t, "client", r.who.PrincipalType())
	require.ElementsMatch(t, []string{"read", "write"}, r.who.Scopes())
	require.Equal(t, principal.ToolLLMAction, r.who.Tool)
	require.Equal(t, "published", r.world, "the run reads in the world it was queued from")

	require.NoError(t, handleLLMActionJob(context.Background(), job, h, ident, kv))
	require.Equal(t, 1, r.runs, "a redelivered run that finished does not run again")

	next := q.jobs[1]
	next.Retry, next.Attempt = jobs.RetryBounded, 1
	require.NoError(t, handleLLMActionJob(context.Background(), next, h, ident, kv))
	require.Equal(t, 2, r.runs, "a later run of the same key runs")
	require.Empty(t, kv.m)

	require.Error(t, registerLLMActionJobs(q, h, ident, nil), "state is required")
	require.Error(t, registerLLMActionJobs(q, h, nil, kv), "identity is required")
}

func TestLLMActionPayloadKey_NoSeparatorCollision(t *testing.T) {
	a := llmActionPayload{Action: "a:b", Entity: "c"}
	b := llmActionPayload{Action: "a", Entity: "b:c"}
	require.NotEqual(t, a.key(), b.key())
}
