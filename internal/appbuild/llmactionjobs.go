package appbuild

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// Background runs of `llm:` actions with `apply: auto` (TKT-WQ925LL). The
// data-entry route queues a run; the job runs the action as the user who
// queued it and applies its proposal.

// llmActionJobKind runs one `apply: auto` llm action on one entity.
var llmActionJobKind = jobs.NewKind("llm-action", "apply")

// LLMActionRunner runs an `apply: auto` llm action as the identity bound on
// ctx and applies its proposal. retry reports whether another attempt may
// succeed where this one failed.
type LLMActionRunner interface {
	RunLLMAction(ctx context.Context, action, address, world string) (retry bool, err error)
}

// llmActionHolder carries the runner, which lives in the data-entry app and
// is built after the services, as [attachRenderHolder] carries the renderer.
//
// Nil: accepted by the methods, which answer like an empty holder.
type llmActionHolder struct {
	r atomic.Pointer[llmActionRunnerBox]
}

type llmActionRunnerBox struct{ LLMActionRunner }

// run forwards to the installed runner. Before one is installed the run is
// retryable: the host installs it as it starts.
func (h *llmActionHolder) run(ctx context.Context, action, address, world string) (bool, error) {
	var box *llmActionRunnerBox
	if h != nil {
		box = h.r.Load()
	}
	if box == nil {
		return true, errors.New("llm actions: no data-entry app runs llm actions in this process yet")
	}
	return box.RunLLMAction(ctx, action, address, world)
}

// SetLLMActionRunner installs the runner behind background llm actions and
// flow steps that run one. The data-entry host calls it once its app is
// built. A package function because Services is at its plimsoll cap.
//
// Nil: rejected, because it would make every run fail at first use rather
// than at wiring.
func SetLLMActionRunner(s *Services, r LLMActionRunner) error {
	if r == nil {
		return errors.New("appbuild: SetLLMActionRunner: runner is required")
	}
	if s.llmActions == nil {
		return errors.New("appbuild: SetLLMActionRunner: services were not fully assembled")
	}
	s.llmActions.r.Store(&llmActionRunnerBox{r})
	return nil
}

// llmIdentity records who queues a run and rebuilds them when it runs.
// [managerOutputs] implements it, as it does the same for a session output.
type llmIdentity interface {
	// Poster reports whether the caller on ctx names a user entity and is
	// a verified principal that matched none.
	Poster(ctx context.Context) (realtime.OutputPoster, error)
	// rebuild binds p as [managerOutputs.rebuild] does.
	rebuild(ctx context.Context, p principal.Principal, mark realtime.OutputPoster) (context.Context, error)
}

// LLMActionJobs queues background llm action runs. A nil value has no queue.
type LLMActionJobs struct {
	q     jobs.Enqueuer
	ident llmIdentity
	kv    state.KV
}

// LLMActionQueue returns the queue background llm actions run on, or nil
// when s runs no background jobs (a one-shot process would exit before the
// job ran).
func LLMActionQueue(s *Services) *LLMActionJobs {
	if s.llmActionQueue == nil {
		return nil
	}
	return &LLMActionJobs{q: s.llmActionQueue, ident: s.llmIdentity, kv: s.stateKV}
}

// llmActionPayload is the job's payload. Run identifies one queued run, so
// a redelivered job can tell that it already finished (see [runKey]).
type llmActionPayload struct {
	Action, Entity, World, User, PrincipalType, Scopes, Run string
	// Unmatched and UserEntity are the queuing request's identity mark
	// (see [llmIdentity]).
	Unmatched, UserEntity bool
}

// key is the job's idempotency key: everything but Run, JSON-encoded so no
// value can run into the next.
func (p llmActionPayload) key() string {
	b, _ := json.Marshal([]string{p.Action, p.Entity, p.World, p.User, p.PrincipalType, p.Scopes}) // strings never fail
	return "llm:" + string(b)
}

// runKey is the state key that marks run as pending. It is written when
// the run is queued and deleted when the run finishes, so a redelivery of a
// finished run finds none and ends. The key is the random token alone and
// the value is empty: the marker names no user or entity, and it lives only
// as long as its job. A job the fs tier drops at exit leaves its marker
// behind; the marker is an empty file under .rela/state that links to
// nothing.
func runKey(run string) string { return "llm-action-jobs/runs/" + run }

func newRunToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // never fails (crypto/rand)
	return hex.EncodeToString(b)
}

// EnqueueLLMAction queues a run of action on address as who. A second run
// of the same action on the same entity for the same principal while one is
// pending is collapsed into it.
//
// The payload selects a configured action and names who runs it; it grants
// nothing. The action, its gates and its outputs come from the live
// configuration when the job runs. Besides the user it carries who's
// principal type and scopes, the client attenuation the ACL ceiling is built
// from: they only narrow the user's grants, so the job runs no wider than
// the request that queued it. Roles from an assertion are not carried; the
// job reads the user's roles from the graph.
func (j *LLMActionJobs) EnqueueLLMAction(
	ctx context.Context, action, address, world string, who principal.Principal,
) error {
	if j == nil || j.q == nil || j.ident == nil || j.kv == nil {
		return errors.New("llm actions: no job queue")
	}
	mark, err := j.ident.Poster(ctx)
	if err != nil {
		return fmt.Errorf("llm actions: identity: %w", err)
	}
	scopes := who.Scopes()
	slices.Sort(scopes)
	p := llmActionPayload{Action: action, Entity: address, World: world, User: who.User,
		PrincipalType: who.PrincipalType(), Scopes: strings.Join(scopes, " "), Run: newRunToken(),
		Unmatched: mark.Unmatched, UserEntity: mark.UserEntity}
	// Before the job: a worker may pick it up before Enqueue returns.
	if err = j.kv.Put(ctx, runKey(p.Run), nil); err != nil {
		return fmt.Errorf("llm actions: record run: %w", err)
	}
	err = j.q.Enqueue(ctx, jobs.Job{
		Kind: llmActionJobKind,
		Payload: map[string]any{
			"action": p.Action, "entity": p.Entity, "world": p.World, "user": p.User,
			"principal_type": p.PrincipalType, "scopes": p.Scopes, "run": p.Run,
			"unmatched": p.Unmatched, "user_entity": p.UserEntity,
		},
		// A few attempts ride out a rate limit or a provider blip; a refusal
		// is not retried (see handleLLMActionJob).
		Retry:          jobs.RetryBounded,
		IdempotencyKey: p.key(),
	})
	if err != nil {
		// Nothing was queued under this run: a duplicate rides on the
		// pending run's own marker.
		if delErr := j.kv.Delete(ctx, runKey(p.Run)); delErr != nil {
			slog.Warn("llm actions: drop unqueued run", "err", delErr)
		}
	}
	if errors.Is(err, jobs.ErrDuplicateJob) {
		return nil
	}
	return err
}

// registerLLMActionJobs registers the job handler on q. It registers even
// when no action applies itself, because data-entry.yaml is reloaded while
// the process runs.
func registerLLMActionJobs(
	q jobs.Registrar, h *llmActionHolder, ident llmIdentity, kv state.KV,
) error {
	if kv == nil || ident == nil {
		return errors.New("appbuild: register llm action jobs: state and identity are required")
	}
	err := q.Register(llmActionJobKind, func(ctx context.Context, job jobs.Job) error {
		return handleLLMActionJob(ctx, job, h, ident, kv)
	})
	if err != nil {
		return fmt.Errorf("appbuild: register llm action jobs: %w", err)
	}
	return nil
}

// handleLLMActionJob runs one job as the user who queued it. A failure that
// another attempt would meet the same way is logged and ends the job, so a
// refused run does not ask the model again.
//
// The queue delivers at least once: a job whose completion was not recorded
// runs again. A run deletes its marker (see [runKey]) when it finishes, so
// the redelivery finds none and ends without asking the model again or
// creating its records a second time. A failed apply is never retried (see
// [dataentry.LLMActionRunner.RunLLMAction]), so only a run that wrote
// nothing keeps its marker for the next attempt.
func handleLLMActionJob(
	ctx context.Context, job jobs.Job, h *llmActionHolder, ident llmIdentity, kv state.KV,
) error {
	str := func(k string) string { v, _ := job.Payload[k].(string); return v }
	flag := func(k string) bool { v, _ := job.Payload[k].(bool); return v }
	p := llmActionPayload{Action: str("action"), Entity: str("entity"), World: str("world"), User: str("user"),
		PrincipalType: str("principal_type"), Scopes: str("scopes"), Run: str("run"),
		Unmatched: flag("unmatched"), UserEntity: flag("user_entity")}
	if p.Action == "" || p.Entity == "" || p.User == "" || p.Run == "" || principal.IsRelaIdentity(p.User) {
		slog.Error("llm action job: unusable payload", "action", p.Action, "entity", p.Entity)
		return nil
	}
	if _, err := kv.Get(ctx, runKey(p.Run)); os.IsNotExist(err) || errors.Is(err, fs.ErrNotExist) {
		return nil // finished on an earlier delivery
	} else if err != nil {
		return fmt.Errorf("llm action %q: read run state: %w", p.Action, err)
	}
	finish := func() {
		if err := kv.Delete(ctx, runKey(p.Run)); err != nil {
			slog.Warn("llm action job: clear finished run", "action", p.Action, "entity", p.Entity, "err", err)
		}
	}
	who := principal.VerifiedFrom(p.User, principal.ToolLLMAction,
		principal.Claims{PrincipalType: p.PrincipalType, Scopes: strings.Fields(p.Scopes)})
	runCtx, err := ident.rebuild(ctx, who, realtime.OutputPoster{Unmatched: p.Unmatched, UserEntity: p.UserEntity})
	if errors.Is(err, errOutputsNoPoster) {
		slog.Warn("llm action job: the user no longer resolves", "action", p.Action, "entity", p.Entity)
		finish()
		return nil
	}
	if err != nil {
		return fmt.Errorf("llm action %q: identity %q: %w", p.Action, p.User, err)
	}
	retry, err := h.run(runCtx, p.Action, p.Entity, p.World)
	if err != nil && retry && !job.FinalAttempt() {
		return err
	}
	finish()
	if err != nil {
		slog.Warn("llm action job failed", "action", p.Action, "entity", p.Entity, "user", p.User, "err", err)
	}
	return nil
}
