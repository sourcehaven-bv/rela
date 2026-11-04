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
	// Key addresses a declared pile instead of Pile (a `notify` action).
	Key string
	// Reason is a key of the declared pile's reasons, or empty.
	Reason string
	// SkipSelf drops a push whose owner is the acting user.
	SkipSelf bool
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

// RecipientFinder finds the users a `notify` action reaches: it follows hops
// from the trigger entity and reads property on every entity it lands on.
// The wiring site answers it from the raw store, since an automation is
// system policy and who it notifies must not depend on who made the write.
type RecipientFinder interface {
	Recipients(ctx context.Context, from entity.Ref, hops []automation.NotifyHop, property string) ([]string, error)
}

// maxNotifyRecipients bounds the users one notify action reaches for one
// write. Each push is a few store round trips inside the cascade, and the
// writer controls the recipient property, so a long list is cut and logged.
const maxNotifyRecipients = 200

// notify runs the `notify` actions for trigger: it finds each action's
// recipients and pushes trigger onto their declared pile, skipping the
// acting user. Like pushPiles it is best-effort and logs what fails, naming
// the automation and the entity but never a recipient.
func (r *Runner) notify(ctx context.Context, trigger *entity.Entity, notes []automation.NotificationToSend) {
	if len(notes) == 0 {
		return
	}
	if r.piles == nil {
		r.noPiles.Do(func() {
			slog.Warn("automation notify skipped: piles are not available in this context",
				"automation", notes[0].AutomationName)
		})
		return
	}
	for _, n := range notes {
		owners := n.Owners
		if len(n.Hops) > 0 {
			if r.recipients == nil {
				slog.Warn("automation notify skipped: no recipient finder in this context",
					"automation", n.AutomationName)
				continue
			}
			var err error
			if owners, err = r.recipients.Recipients(ctx, trigger.Ref(), n.Hops, n.Property); err != nil {
				slog.Warn("automation notify: finding recipients failed",
					"automation", n.AutomationName, "entity", trigger.Ref().String(), "error", err)
				continue
			}
		}
		if len(owners) > maxNotifyRecipients {
			slog.Warn("automation notify: too many recipients, notifying the first ones",
				"automation", n.AutomationName, "entity", trigger.Ref().String(),
				"recipients", len(owners), "limit", maxNotifyRecipients)
			owners = owners[:maxNotifyRecipients]
		}
		for _, owner := range owners {
			err := r.piles.PushPile(ctx, PileRequest{
				Owner: owner, Key: n.Pile, Reason: n.Reason, SkipSelf: true, Ref: trigger.Ref(),
			})
			if err != nil {
				slog.Warn("automation notify failed",
					"automation", n.AutomationName, "entity", trigger.Ref().String(), "error", err)
			}
		}
	}
}
