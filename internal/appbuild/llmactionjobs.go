package appbuild

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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

// LLMActionJobs queues background llm action runs. A nil value has no queue.
type LLMActionJobs struct{ q jobs.Enqueuer }

// LLMActionQueue returns the queue background llm actions run on, or nil
// when s runs no background jobs (a one-shot process would exit before the
// job ran).
func LLMActionQueue(s *Services) *LLMActionJobs {
	if s.llmActionQueue == nil {
		return nil
	}
	return &LLMActionJobs{q: s.llmActionQueue}
}

// llmActionPayload is the job's payload. Run identifies one queued run, so
// a redelivered job can tell that it already applied.
type llmActionPayload struct {
	Action, Entity, World, User, PrincipalType, Scopes, Run string
}

// key is the job's idempotency key: everything but Run, JSON-encoded so no
// value can run into the next.
func (p llmActionPayload) key() string {
	b, _ := json.Marshal([]string{p.Action, p.Entity, p.World, p.User, p.PrincipalType, p.Scopes}) // strings never fail
	return "llm:" + string(b)
}

// handledKey is the state key recording the last run applied for p's key.
func (p llmActionPayload) handledKey() string {
	sum := sha256.Sum256([]byte(p.key()))
	return "llm-action-jobs/" + hex.EncodeToString(sum[:])
}

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
	if j == nil || j.q == nil {
		return errors.New("llm actions: no job queue")
	}
	scopes := who.Scopes()
	slices.Sort(scopes)
	p := llmActionPayload{Action: action, Entity: address, World: world, User: who.User,
		PrincipalType: who.PrincipalType(), Scopes: strings.Join(scopes, " "), Run: newRunToken()}
	err := j.q.Enqueue(ctx, jobs.Job{
		Kind: llmActionJobKind,
		Payload: map[string]any{
			"action": p.Action, "entity": p.Entity, "world": p.World, "user": p.User,
			"principal_type": p.PrincipalType, "scopes": p.Scopes, "run": p.Run,
		},
		// A few attempts ride out a rate limit or a provider blip; a refusal
		// is not retried (see handleLLMActionJob).
		Retry:          jobs.RetryBounded,
		IdempotencyKey: p.key(),
	})
	if errors.Is(err, jobs.ErrDuplicateJob) {
		return nil
	}
	return err
}

// registerLLMActionJobs registers the job handler on q. It registers even
// when no action applies itself, because data-entry.yaml is reloaded while
// the process runs.
func registerLLMActionJobs(
	q jobs.Registrar, h *llmActionHolder, bind func(context.Context) (context.Context, error), kv state.KV,
) error {
	if kv == nil {
		return errors.New("appbuild: register llm action jobs: state is required")
	}
	err := q.Register(llmActionJobKind, func(ctx context.Context, job jobs.Job) error {
		return handleLLMActionJob(ctx, job, h, bind, kv)
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
// runs again. A run that applied records its token in kv first, so the
// redelivery ends without asking the model again or creating its records a
// second time. A failed apply is never retried (see
// [dataentry.LLMActionRunner.RunLLMAction]), so only a run that wrote nothing is.
func handleLLMActionJob(
	ctx context.Context, job jobs.Job, h *llmActionHolder,
	bind func(context.Context) (context.Context, error), kv state.KV,
) error {
	str := func(k string) string { v, _ := job.Payload[k].(string); return v }
	p := llmActionPayload{Action: str("action"), Entity: str("entity"), World: str("world"), User: str("user"),
		PrincipalType: str("principal_type"), Scopes: str("scopes"), Run: str("run")}
	if p.Action == "" || p.Entity == "" || p.User == "" || principal.IsRelaIdentity(p.User) {
		slog.Error("llm action job: unusable payload", "action", p.Action, "entity", p.Entity)
		return nil
	}
	if done, err := kv.Get(ctx, p.handledKey()); err == nil && p.Run != "" && string(done) == p.Run {
		return nil
	} else if err != nil && !os.IsNotExist(err) && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("llm action %q: read run state: %w", p.Action, err)
	}
	ctx = principal.With(ctx, principal.VerifiedFrom(p.User, principal.ToolLLMAction,
		principal.Claims{PrincipalType: p.PrincipalType, Scopes: strings.Fields(p.Scopes)}))
	ctx, err := bind(ctx)
	if err != nil {
		return fmt.Errorf("llm action %q: identity %q: %w", p.Action, p.User, err)
	}
	action, address, user := p.Action, p.Entity, p.User
	retry, err := h.run(ctx, action, address, p.World)
	if err == nil {
		if p.Run != "" {
			if putErr := kv.Put(ctx, p.handledKey(), []byte(p.Run)); putErr != nil {
				slog.Warn("llm action job: record applied run", "action", action, "entity", address, "err", putErr)
			}
		}
		return nil
	}
	if retry && !job.FinalAttempt() {
		return err
	}
	slog.Warn("llm action job failed", "action", action, "entity", address, "user", user, "err", err)
	return nil
}
