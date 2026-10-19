package realtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// AnchorExistence tells the sweep which entities exist. It reads the store
// directly: no ACL applies, because the sweep acts for the system and never
// shows what it read. A soft-deleted entity counts as existing, since it
// can still be restored.
type AnchorExistence interface {
	// Faces returns, for each id in ids that exists, its stored faces. A
	// faceless entity has the one face "". An id that does not exist is
	// absent from the map.
	Faces(ctx context.Context, ids []string) (map[string][]entity.Face, error)
}

// ChangeMarker is told about the channels a sweep changed, so live
// subscribers in this process see the change. [*Local] implements it.
type ChangeMarker interface {
	Mark(keys ...channels.Key)
}

// SweepDeps are the collaborators of a [Sweeper].
type SweepDeps struct {
	Store   channels.Store
	Config  ConfigSource
	Anchors AnchorExistence
	Marker  ChangeMarker
	Clock   Clock
}

// Sweeper is the background work on channels that no request drives: the
// automatic close and archive, the report of stale pending actions, and
// retention, which erases detached channels once they are due.
//
// It holds the ids found missing by earlier runs (see
// [Sweeper.RunRetention]), so one Sweeper serves one process for its whole
// life. A restart forgets them, which only delays an orphan's detach.
type Sweeper struct {
	store   channels.Store
	cfg     ConfigSource
	anchors AnchorExistence
	marker  ChangeMarker
	clock   Clock

	mu      sync.Mutex
	missing map[string]time.Time
}

// NewSweeper returns the sweeper over d.
//
// Nil: every field of d is rejected.
func NewSweeper(d SweepDeps) (*Sweeper, error) {
	switch {
	case d.Store == nil:
		return nil, errNil("Store")
	case d.Config == nil:
		return nil, errNil("Config")
	case d.Anchors == nil:
		return nil, errNil("Anchors")
	case d.Marker == nil:
		return nil, errNil("Marker")
	case d.Clock == nil:
		return nil, errNil("Clock")
	}
	return &Sweeper{
		store: d.Store, cfg: d.Config, anchors: d.Anchors, marker: d.Marker, clock: d.Clock,
		missing: map[string]time.Time{},
	}, nil
}

// systemAuthor is the author of every write the realtime worker makes
// without a request.
func systemAuthor() channels.Author {
	return channels.Author{User: principal.UserRealtime, Tool: principal.ToolRealtime, PrincipalType: "system"}
}

// applyTransition moves key from expect to t through the store's
// compare-and-set and tells m the channel changed. A person's request, the
// sweep's `auto.after` and an `auto.when` rule all move channels through
// it, each having checked expect against opFrom. t.TriggeredBy names the
// rule behind a change no person made; the store keeps it in the audit
// record.
func applyTransition(
	ctx context.Context, st channels.Store, m ChangeMarker, key channels.Key, expect channels.State,
	t channels.Transition,
) (channels.Channel, error) {
	c, err := st.SetLifecycle(ctx, key, expect, t)
	if err != nil {
		return c, err
	}
	m.Mark(key)
	return c, nil
}

// autoLabel is the triggered-by label of the automatic operation op of
// channel type typ.
func autoLabel(typ, op string) string {
	return "auto:" + typ + "/" + op
}

// LifecycleReport is what one [Sweeper.RunLifecycle] did.
type LifecycleReport struct {
	Closed   int
	Archived int
	// StalePending lists the actions pending for longer than
	// [realtimelimits.PendingRelease], in channel types that declare
	// actions. The sweep only reports them; a moderator releases one.
	StalePending []channels.PendingAction
}

// autoStep is one automatic lifecycle operation of one channel type.
type autoStep struct {
	op     string
	target channels.State
	after  realtimecfg.Period
	from   []channels.State
}

// autoSteps returns the automatic steps ct declares with `after:`, each
// starting from the states opFrom gives.
func autoSteps(ct realtimecfg.ChannelType) []autoStep {
	var out []autoStep
	for _, name := range []string{realtimecfg.LifecycleClose, realtimecfg.LifecycleArchive} {
		op := ct.Lifecycle.Op(name)
		if op == nil || op.Auto == nil || op.Auto.After.IsZero() {
			continue
		}
		out = append(out, autoStep{
			op: name, target: lifecycleTarget(name), after: op.Auto.After, from: opFrom(name, op),
		})
	}
	return out
}

// RunLifecycle closes and archives the channels whose `auto.after` has
// passed, and reports stale pending actions. It does nothing when realtime
// is off. Under a degraded config it skips the automatic steps, because
// the config it would follow is no longer the operator's.
//
// A channel another process or a person moved first is skipped: the
// compare-and-set fails with [channels.ErrConflict]. Other errors are
// joined into the returned error after every step has run.
func (s *Sweeper) RunLifecycle(ctx context.Context) (LifecycleReport, error) {
	var rep LifecycleReport
	snap := s.cfg.Load()
	if snap.Config == nil {
		return rep, nil
	}
	now := s.clock()
	var errs []error
	types := snap.Config.ChannelTypes()
	if !snap.Degraded {
		for _, ct := range types {
			for _, step := range autoSteps(ct) {
				n, err := s.runAutoStep(ctx, ct.Name, step, now)
				errs = append(errs, err)
				if step.target == channels.StateClosed {
					rep.Closed += n
				} else {
					rep.Archived += n
				}
			}
		}
	}
	stale, err := s.stalePending(ctx, types, now)
	rep.StalePending = stale
	errs = append(errs, err)
	return rep, errors.Join(errs...)
}

func (s *Sweeper) runAutoStep(ctx context.Context, typ string, step autoStep, now time.Time) (int, error) {
	moved := 0
	var errs []error
	for _, from := range step.from {
		list, err := s.store.ListByLifecycle(ctx, typ, from, step.after.Cutoff(now))
		if err != nil {
			errs = append(errs, fmt.Errorf("realtime: list %s %s channels: %w", from, typ, err))
			continue
		}
		for _, c := range list {
			if step.after.AddTo(c.StateSince()).After(now) {
				continue
			}
			_, err := applyTransition(ctx, s.store, s.marker, c.Key, from, channels.Transition{
				To: step.target, By: systemAuthor(), At: now, TriggeredBy: autoLabel(typ, step.op),
			})
			switch {
			case err == nil:
				moved++
			case errors.Is(err, channels.ErrConflict), errors.Is(err, channels.ErrNotFound):
			default:
				errs = append(errs, fmt.Errorf("realtime: auto %s %s: %w", step.op, c.Key, err))
			}
		}
	}
	return moved, errors.Join(errs...)
}

// stalePending returns the pending actions older than PendingRelease in
// the types that declare actions. It reads nothing when no type does.
func (s *Sweeper) stalePending(
	ctx context.Context, types []realtimecfg.ChannelType, now time.Time,
) ([]channels.PendingAction, error) {
	withActions := map[string]bool{}
	for _, ct := range types {
		for _, mt := range ct.MessageTypes {
			if len(mt.Actions) > 0 {
				withActions[ct.Name] = true
			}
		}
	}
	if len(withActions) == 0 {
		return nil, nil
	}
	all, err := s.store.StalePending(ctx, now.Add(-realtimelimits.PendingRelease))
	if err != nil {
		return nil, fmt.Errorf("realtime: list stale pending actions: %w", err)
	}
	var out []channels.PendingAction
	for _, p := range all {
		if withActions[p.Key.Type] {
			out = append(out, p)
		}
	}
	return out, nil
}
