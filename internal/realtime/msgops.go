package realtime

import (
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// The message operations a message's _actions names next to its actions.
// realtimecfg reserves these names, so no action takes one.
const (
	// OpEdit is offered when the reader may edit the message now: a PATCH.
	OpEdit = "edit"
	// OpDelete is offered when the reader may delete their own message
	// now (delete_own): a DELETE without a reason.
	OpDelete = "delete"
	// OpRemove is offered when the reader passes the remove gate: a DELETE
	// with a reason, which the channel type may require.
	OpRemove = "remove"
	// OpRelease is offered when the reader may release the message's
	// pending action now.
	OpRelease = "release"
)

// opGates holds the answers of the message operation gates for one call.
// They depend on the channel and the caller, not on the message, so a page
// asks each gate once rather than once per message.
type opGates struct {
	caller    bool
	edit      bool
	deleteOwn bool
	moderator bool
	post      map[string]bool
}

// opGates computes the gates on first use and keeps them on c.
func (c *call) opGates() *opGates {
	if c.ops != nil {
		return c.ops
	}
	ops := c.ct.MessageOps
	g := &opGates{caller: c.requireCaller() == nil, post: map[string]bool{}}
	if g.caller {
		g.edit = ops.EditOwn != nil && c.gate(ops.EditOwn.Gate)
		g.deleteOwn = ops.DeleteOwn != nil && c.gate(ops.DeleteOwn.Gate)
		g.moderator = c.moderator()
	}
	c.ops = g
	return g
}

// mayPostCached is [call.mayPost] asked once per message type and call.
func (g *opGates) mayPostCached(c *call, mt *realtimecfg.MessageType) bool {
	ok, seen := g.post[mt.Name]
	if !seen {
		ok = c.mayPost(mt)
		g.post[mt.Name] = ok
	}
	return ok
}

// messageActions lists what the reader may do with m now: the message
// operations and the actions of [call.offeredActions], sorted. Each answer
// follows the checks of the operation itself (Edit, Delete,
// ReleaseAction), so the affordance and the operation agree; the rate
// limit and the validation of new properties are left to the operation.
// restrictedRef reports that a ref property of m renders as restricted for
// the reader: they could not send the message back unchanged, so edit is
// not offered.
func (c *call) messageActions(m channels.Message, restrictedRef bool) []string {
	if m.Removed != nil {
		return nil
	}
	out := c.offeredActions(m)
	g := c.opGates()
	if !g.caller {
		return out
	}
	pending := m.ActionStatus() == channels.ActionPending
	if pending {
		if c.releaseCheck(m) == nil {
			out = append(out, OpRelease)
		}
	} else {
		if !restrictedRef && c.mayEdit(g, m) {
			out = append(out, OpEdit)
		}
		if c.mayDeleteOwn(g, m) {
			out = append(out, OpDelete)
		}
		if g.moderator {
			out = append(out, OpRemove)
		}
	}
	slices.Sort(out)
	return out
}

// mayEdit follows the checks of [Service.Edit] on m, which is not removed
// and not pending.
func (c *call) mayEdit(g *opGates, m channels.Message) bool {
	if !g.edit || !sameAuthor(m.Author, c.p) || c.state() != channels.StateOpen {
		return false
	}
	within := c.ct.MessageOps.EditOwn.Within
	if !within.IsZero() && !within.AddTo(m.At).After(c.now) {
		return false
	}
	mt := c.ct.MessageTypes[m.Type]
	return mt != nil && g.mayPostCached(c, mt)
}

// mayDeleteOwn follows [call.deleteOwn] with the gate answer cached.
func (c *call) mayDeleteOwn(g *opGates, m channels.Message) bool {
	if !g.deleteOwn || !sameAuthor(m.Author, c.p) {
		return false
	}
	from := c.ct.MessageOps.DeleteOwn.From
	return from == "" || string(c.state()) == string(from)
}
