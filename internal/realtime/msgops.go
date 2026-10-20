package realtime

import (
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// The message operations a message's _actions names next to its actions.
// The names are [realtimecfg.MessageOpNames], which no action may take.
const (
	// OpEdit is offered when the reader may edit the message now: a PATCH.
	OpEdit = realtimecfg.MessageOpEdit
	// OpDelete is offered when the reader may delete their own message
	// now (delete_own): a DELETE without a reason.
	OpDelete = realtimecfg.MessageOpDelete
	// OpRemove is offered when the reader passes the remove gate: a DELETE
	// with a reason, which the channel type may require.
	OpRemove = realtimecfg.MessageOpRemove
	// OpRelease is offered when the reader may release the message's
	// pending action now.
	OpRelease = realtimecfg.MessageOpRelease
)

// MessageOpNames returns every message operation name, sorted: the names
// of [realtimecfg.MessageOpNames], for a client mirror to be checked
// against without importing realtimecfg.
func MessageOpNames() []string { return realtimecfg.MessageOpNames() }

// opGates holds the answers of the message operation gates for one call.
// They depend on the channel and the caller, not on the message, so a page
// asks each gate once rather than once per message. The operations and the
// affordances read them through the same methods: [call.editGate],
// [call.deleteOwnGate], [call.moderator] and [call.mayPost].
type opGates struct {
	edit, deleteOwn, moderator *bool
	post                       map[string]bool
}

// memo returns *slot, computing it with f on first use.
func memo(slot **bool, f func() bool) bool {
	if *slot == nil {
		ok := f()
		*slot = &ok
	}
	return **slot
}

// editGate reports whether the caller passes the edit_own gate; false when
// the channel type does not offer edit_own.
func (c *call) editGate() bool {
	return memo(&c.ops.edit, func() bool {
		op := c.ct.MessageOps.EditOwn
		return op != nil && c.requireCaller() == nil && c.gate(op.Gate)
	})
}

// deleteOwnGate reports whether the caller passes the delete_own gate;
// false when the channel type does not offer delete_own.
func (c *call) deleteOwnGate() bool {
	return memo(&c.ops.deleteOwn, func() bool {
		op := c.ct.MessageOps.DeleteOwn
		return op != nil && c.requireCaller() == nil && c.gate(op.Gate)
	})
}

// messageActions lists what the reader may do with m now: the message
// operations and the actions of [call.offeredActions], sorted. Each
// operation is offered when the check its operation runs passes
// ([call.editCheck], [call.deleteOwn], [call.moderator],
// [call.releaseCheck]); the rate limit and the validation of new properties
// are left to the operation.
func (c *call) messageActions(m channels.Message) []string {
	if m.Removed != nil {
		return nil
	}
	out := c.offeredActions(m)
	if c.requireCaller() != nil {
		return out
	}
	if m.ActionStatus() == channels.ActionPending {
		if c.releaseCheck(m) == nil {
			out = append(out, OpRelease)
		}
	} else {
		if c.editCheck(m) == nil {
			out = append(out, OpEdit)
		}
		if c.deleteOwn(m) == nil {
			out = append(out, OpDelete)
		}
		if c.moderator() {
			out = append(out, OpRemove)
		}
	}
	slices.Sort(out)
	return out
}

// messageDeadlines are the times at which m's operations change for the
// reader without m changing: EditUntil while edit is offered and edit_own
// has a window, ReleaseAfter while release is refused only because the
// action has not been pending for [realtimelimits.PendingRelease] yet. A
// client re-reads the message at the soonest one.
func (c *call) messageDeadlines(m channels.Message, actions []string) (editUntil, releaseAfter time.Time) {
	if slices.Contains(actions, OpEdit) {
		if within := c.ct.MessageOps.EditOwn.Within; !within.IsZero() {
			editUntil = within.AddTo(m.At)
		}
	}
	if m.Removed == nil && m.ActionStatus() == channels.ActionPending && !slices.Contains(actions, OpRelease) &&
		c.requireCaller() == nil && c.releaseGateCheck(m) == nil {

		releaseAfter = m.Action.PendingAt.Add(realtimelimits.PendingRelease)
	}
	return editUntil, releaseAfter
}

// editAvailable refuses an edit before the message is read: a caller
// without identity, or a channel type without edit_own.
func (c *call) editAvailable() error {
	if err := c.requireCaller(); err != nil {
		return err
	}
	if c.ct.MessageOps.EditOwn == nil {
		return forbidden(TypeEditDenied,
			fmt.Sprintf("channel type %s does not offer edit_own", c.ct.Name),
			"channel_types."+c.ct.Name+".message_ops.edit_own")
	}
	return nil
}

// editCheck decides whether the caller may edit m now. m is a stored
// message that was not removed. [Service.Edit] and [call.messageActions]
// both ask it, so the affordance and the operation agree; the rate limit
// and the validation of the new properties are left to Edit.
func (c *call) editCheck(m channels.Message) error {
	if err := c.editAvailable(); err != nil {
		return err
	}
	op := c.ct.MessageOps.EditOwn
	rule := "channel_types." + c.ct.Name + ".message_ops.edit_own"
	if err := requireNotPending(m); err != nil {
		return err
	}
	if !sameAuthor(m.Author, c.p) {
		return forbidden(TypeEditDenied, "only the author may edit a message", rule)
	}
	if !c.editGate() {
		return gateDenied(TypeEditDenied, "editing a message", rule, op.Gate, c.ct.Roles)
	}
	if !op.Within.IsZero() && !op.Within.AddTo(m.At).After(c.now) {
		return forbidden(TypeEditWindowClosed,
			fmt.Sprintf("a message can be edited within %s of posting", op.Within), rule+".within")
	}
	mt := c.ct.MessageTypes[m.Type]
	if mt == nil {
		return deny(http.StatusConflict, TypeMessageTypeGone,
			fmt.Sprintf("channel type %s no longer declares message type %q", c.ct.Name, m.Type))
	}
	if !c.mayPost(mt) {
		return postDenied(c.ct, mt)
	}
	return c.requireOpen()
}
