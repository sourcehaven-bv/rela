package realtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// OutputsDeps are the collaborators of an [Outputs] runner.
type OutputsDeps struct {
	Store  channels.Store
	Config ConfigSource
	Writer OutputWriter
	// Local is the process-local state the channel services share; its
	// lock keeps the runner and an inline run of one channel apart.
	Local *Local
	Clock Clock
}

// Outputs retries the outputs marks no live holder runs: marks whose
// inline run stopped, failed for a reason a retry may not repeat, or was
// never started (a timer's or a trigger's move, a job mark). The leader runs it on its minute
// tick, in a round of its own, so a slow output never holds up the
// timers, the triggers or the leader lock check.
type Outputs struct {
	e outputEngine
	// warned holds the dangling refs already logged, so each is logged
	// once per process.
	warned sync.Map
}

// NewOutputs returns the runner over d.
//
// Nil: every field of d is rejected.
func NewOutputs(d OutputsDeps) (*Outputs, error) {
	switch {
	case d.Store == nil:
		return nil, errNil("Store")
	case d.Config == nil:
		return nil, errNil("Config")
	case d.Writer == nil:
		return nil, errNil("Writer")
	case d.Local == nil:
		return nil, errNil("Local")
	case d.Clock == nil:
		return nil, errNil("Clock")
	}
	e := outputEngine{store: d.Store, cfg: d.Config, writer: d.Writer, local: d.Local, clock: d.Clock}
	return &Outputs{e: e}, nil
}

// OutputsReport is what one [Outputs.Run] did.
type OutputsReport struct {
	// Ran counts the marks this round claimed and ran.
	Ran int
	// More is set when eligible marks were left for the next round.
	More bool
}

// Run claims and runs at most [realtimelimits.MaxOutputRunsPerRound]
// eligible marks, oldest first. It does nothing while realtime is off or
// the configuration is degraded: the acl.yaml that broke it may restrict
// what the outputs write.
func (o *Outputs) Run(ctx context.Context) (OutputsReport, error) {
	var rep OutputsReport
	snap := o.e.cfg.Load()
	if snap.Config == nil || snap.Degraded {
		return rep, nil
	}
	now := o.e.clock()
	all, err := o.e.store.StalePending(ctx, now)
	if err != nil {
		return rep, fmt.Errorf("realtime: list eligible outputs marks: %w", err)
	}
	var marks []channels.PendingAction
	for _, p := range all {
		if p.Name == channels.OutputsAction {
			marks = append(marks, p)
		}
	}
	if len(marks) > realtimelimits.MaxOutputRunsPerRound {
		marks, rep.More = marks[:realtimelimits.MaxOutputRunsPerRound], true
	}
	var errs []error
	for _, p := range marks {
		if ctx.Err() != nil {
			// The round's deadline passed: the rest wait for the next.
			rep.More = true
			break
		}
		ran, err := o.runOne(ctx, p)
		if ran {
			rep.Ran++
		}
		errs = append(errs, err)
	}
	return rep, errors.Join(errs...)
}

// runOne claims and runs the mark p names. A mark another runner claimed
// first is skipped.
func (o *Outputs) runOne(ctx context.Context, p channels.PendingAction) (bool, error) {
	unlock := o.e.local.lockOutputs(p.Key)
	defer unlock()
	r, ok, err := o.e.load(ctx, p.Key, p.Seq)
	if err != nil {
		return false, fmt.Errorf("realtime: read outputs mark %s#%d: %w", p.Key, p.Seq, err)
	}
	if !ok || r.notice.Action.Status != channels.ActionPending {
		ref := fmt.Sprintf("%s#%d", p.Key, p.Seq)
		if _, seen := o.warned.LoadOrStore(ref, true); !seen {
			slog.WarnContext(ctx, "realtime: a pending outputs ref names no pending outputs mark; skipped",
				"channel", p.Key.String(), "seq", p.Seq)
		}
		return false, nil
	}
	now := o.e.clock()
	claim := *r.notice.Action
	claim.Outputs = claim.Outputs.Clone()
	claim.Outputs.Attempt++
	claim.Outputs.Claim, claim.Outputs.ClaimedAt = channels.NewName(), now
	claim.At, claim.PendingAt = now, now.Add(realtimelimits.OutputClaimTTL)
	got, err := o.e.store.SetAction(ctx, p.Key, p.Seq, channels.OutputsAction, channels.ActionPending, claim)
	if errors.Is(err, channels.ErrConflict) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("realtime: claim outputs mark %s#%d: %w", p.Key, p.Seq, err)
	}
	r.notice, r.retry = got, true
	r.plan = resolveMark(o.e.cfg.Load(), p.Key, r.cause, got.Action.Outputs)
	if readsTitle(r.plan.outs) {
		if h, herr := o.e.store.Head(ctx, p.Key); herr == nil {
			r.title = h.Title
		}
	}
	o.e.run(ctx, &r)
	return true, nil
}
