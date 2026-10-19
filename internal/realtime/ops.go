package realtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// message reads the message at seq that an operation acts on. A missing
// message is 404 and a removed one 409.
func (s *Service) message(c *call, seq int64) (channels.Message, error) {
	if c.head == nil || seq <= 0 {
		return channels.Message{}, messageNotFound()
	}
	m, err := s.store.Get(c.ctx, c.key, seq)
	if errors.Is(err, channels.ErrNotFound) {
		return channels.Message{}, messageNotFound()
	}
	if err != nil {
		return channels.Message{}, fmt.Errorf("realtime: read message: %w", err)
	}
	if m.Removed != nil {
		return channels.Message{}, storeError(channels.ErrRemoved)
	}
	return m, nil
}

// requireNotPending refuses a change to m while its action is pending: the
// promote builds the entity from the message as it was.
func requireNotPending(m channels.Message) error {
	if m.ActionStatus() == channels.ActionPending {
		return storeError(channels.ErrActionPending)
	}
	return nil
}

// Edit replaces the properties of the caller's own message (edit_own). It
// needs the edit_own gate and, when `within:` is set, an edit inside the
// window after posting. The new properties pass every check a post does: a
// current post role of the message type, an open channel, the rate limit,
// validation, `when:` and the refs.
func (s *Service) Edit(ctx context.Context, addr RawAddress, seq int64, props map[string]any) (MessageView, error) {
	c, openErr := s.open(ctx, addr)
	if openErr != nil {
		return MessageView{}, openErr
	}
	if err := c.requireCaller(); err != nil {
		return MessageView{}, err
	}
	ops := c.ct.MessageOps
	if ops.EditOwn == nil {
		return MessageView{}, forbidden(TypeEditDenied,
			fmt.Sprintf("channel type %s does not offer edit_own", c.ct.Name),
			"channel_types."+c.ct.Name+".message_ops.edit_own")
	}
	m, err := s.message(c, seq)
	if err != nil {
		return MessageView{}, err
	}
	rule := "channel_types." + c.ct.Name + ".message_ops.edit_own"
	if err := requireNotPending(m); err != nil {
		return MessageView{}, err
	}
	if !sameAuthor(m.Author, c.p) {
		return MessageView{}, forbidden(TypeEditDenied, "only the author may edit a message", rule)
	}
	if !c.gate(ops.EditOwn.Gate) {
		return MessageView{}, gateDenied(TypeEditDenied, "editing a message", rule, ops.EditOwn.Gate, c.ct.Roles)
	}
	if !ops.EditOwn.Within.IsZero() && !ops.EditOwn.Within.AddTo(m.At).After(c.now) {
		return MessageView{}, forbidden(TypeEditWindowClosed,
			fmt.Sprintf("a message can be edited within %s of posting", ops.EditOwn.Within), rule+".within")
	}
	mt := c.ct.MessageTypes[m.Type]
	if mt == nil {
		return MessageView{}, deny(http.StatusConflict, TypeMessageTypeGone,
			fmt.Sprintf("channel type %s no longer declares message type %q", c.ct.Name, m.Type))
	}
	if !c.mayPost(mt) {
		return MessageView{}, postDenied(c.ct, mt)
	}
	err = c.requireOpen()
	if err != nil {
		return MessageView{}, err
	}
	err = s.local.limiter.take(c.now, bucketIdentity(c.p), c.key, opEdit, realtimelimits.OpRate)
	if err != nil {
		return MessageView{}, err
	}
	vp, refs, err := s.checkMessage(c, mt, props, c.snap.Config.Limits().MessageBytes)
	if err != nil {
		return MessageView{}, err
	}
	// The new props may hold a ref the type's record lacks.
	if recErr := s.record(c); recErr != nil {
		return MessageView{}, recErr
	}
	got, err := s.store.Edit(c.ctx, c.key, seq, vp.props, c.now)
	if err != nil {
		return MessageView{}, messageStoreError(err)
	}
	s.local.mark(c.key)
	return s.rendererWith(c, refs).message(got), nil
}

// Delete removes a message. A caller who gives a reason and passes the
// remove gate removes it as a moderator, so the reason is kept even on
// their own message. Otherwise the author's own delete (delete_own) is
// tried first: the caller wrote the message, passes its gate and, when
// `from:` is set, the channel is in that state. Then the moderator's
// remove: the remove gate, and a reason when one is required. A caller who
// passes neither is refused naming both rules.
//
// The reason is kept for moderators and the audit trail; an author's own
// delete records none.
func (s *Service) Delete(ctx context.Context, addr RawAddress, seq int64, reason string) (MessageView, error) {
	c, openErr := s.open(ctx, addr)
	if openErr != nil {
		return MessageView{}, openErr
	}
	if err := c.requireCaller(); err != nil {
		return MessageView{}, err
	}
	m, err := s.message(c, seq)
	if err != nil {
		return MessageView{}, err
	}
	if err := requireNotPending(m); err != nil {
		return MessageView{}, err
	}
	removal := channels.Removal{By: authorOf(c.p), At: c.now}
	ops := c.ct.MessageOps
	moderator := c.moderator()
	ownRefusal := c.deleteOwn(ops.DeleteOwn, m)
	switch {
	case moderator && reason != "":
		removal.Kind, removal.Reason = channels.RemovalModerator, reason
	case ownRefusal == nil:
		removal.Kind = channels.RemovalAuthor
	case moderator:
		if ops.Remove.ReasonRequired {
			return MessageView{}, deny(http.StatusBadRequest, TypeReasonRequired,
				"removing a message needs a reason")
		}
		removal.Kind, removal.Reason = channels.RemovalModerator, reason
	case ownRefusal.Type == TypeDeleteDenied && ownRefusal.Rule != "":
		return MessageView{}, ownRefusal
	default:
		return MessageView{}, forbidden(TypeDeleteDenied, c.deleteTitle(),
			"channel_types."+c.ct.Name+".message_ops")
	}
	err = s.local.limiter.take(c.now, bucketIdentity(c.p), c.key, opRemove, realtimelimits.OpRate)
	if err != nil {
		return MessageView{}, err
	}
	got, err := s.store.Remove(c.ctx, c.key, seq, removal)
	if err != nil {
		return MessageView{}, messageStoreError(err)
	}
	s.local.mark(c.key)
	return s.rendererWith(c, nil).message(got), nil
}

// deleteOwn reports why the caller may not delete m as its author, or nil
// when they may. A refusal with a Rule is specific to an author who passed
// the gate (the channel is not in the `from:` state); one without a Rule
// means delete_own does not apply.
func (c *call) deleteOwn(op *realtimecfg.DeleteOwn, m channels.Message) *DenialError {
	if op == nil || !sameAuthor(m.Author, c.p) || !c.gate(op.Gate) {
		return &DenialError{Type: TypeDeleteDenied}
	}
	if op.From != "" && string(c.state()) != string(op.From) {
		return forbidden(TypeDeleteDenied,
			fmt.Sprintf("an author may delete a message only while the channel is %s", op.From),
			"channel_types."+c.ct.Name+".message_ops.delete_own.from")
	}
	return nil
}

// deleteTitle names who may delete a message, for a refusal.
func (c *call) deleteTitle() string {
	ops := c.ct.MessageOps
	var title string
	if ops.DeleteOwn != nil {
		title = "the author with " + describeGate(ops.DeleteOwn.Gate, c.ct.Roles)
	}
	if ops.Remove != nil {
		if title != "" {
			title += ", or "
		}
		title += describeGate(ops.Remove.Gate, c.ct.Roles)
	}
	if title == "" {
		return fmt.Sprintf("channel type %s does not offer deleting messages", c.ct.Name)
	}
	return "deleting this message needs " + title
}
