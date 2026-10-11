package autocascade

import (
	"context"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// PileRequest is one add_to_pile push of a cascade's trigger entity.
type PileRequest struct {
	// Owner is the target user. Empty means the acting user (the principal
	// on ctx).
	Owner string
	// Pile is the pile name.
	Pile string
	// Ref is the trigger entity, its face included.
	Ref entity.Ref
	// Create makes the pile when it does not exist.
	Create bool
}

// PilePusher is what the Runner needs to run add_to_pile actions. The wiring
// site adapts the piles service to it; [PilePushFunc] does so from a closure.
type PilePusher interface {
	PushPile(ctx context.Context, req PileRequest) error
}

// PilePushFunc adapts a function to [PilePusher].
type PilePushFunc func(ctx context.Context, req PileRequest) error

// PushPile satisfies [PilePusher] by calling the function itself.
func (f PilePushFunc) PushPile(ctx context.Context, req PileRequest) error { return f(ctx, req) }

// pushPiles runs the add_to_pile actions for trigger. A pile is a
// notification surface, not a system of record, so every failure is logged
// and the write it rides on still succeeds. Without a pusher the actions are
// skipped with one warning per Runner.
//
// The log line names the automation and the entity, never the pile or the
// owner: both are interpolated, so they can carry entity field values or a
// login.
func (r *Runner) pushPiles(ctx context.Context, trigger *entity.Entity, pushes []automation.PileToPush) {
	if len(pushes) == 0 {
		return
	}
	if r.piles == nil {
		r.noPiles.Do(func() {
			slog.Warn("automation add_to_pile skipped: piles are not available in this context",
				"automation", pushes[0].AutomationName)
		})
		return
	}
	for _, p := range pushes {
		err := r.piles.PushPile(ctx, PileRequest{Owner: p.Owner, Pile: p.Pile, Ref: trigger.Ref(), Create: p.Create})
		if err != nil {
			slog.Warn("automation add_to_pile failed",
				"automation", p.AutomationName,
				"entity", trigger.Ref().String(),
				"error", err)
		}
	}
}
