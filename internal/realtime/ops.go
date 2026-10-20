package realtime

import (
	"context"
	"errors"
	"fmt"
	"maps"
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

// Edit replaces the properties of the caller's own message (edit_own). The
// checks before the new properties are [call.editCheck]: the edit_own gate,
// the edit window, a current post role of the message type and an open
// channel. The new properties then pass every check a post does: the rate
// limit, validation, `when:` and the refs.
//
// props replaces every property the caller can read. A ref property whose
// stored entity the caller cannot read, and that props does not name, is
// kept as stored: the caller never saw its id, so they could not send it
// back, and dropping it would erase what they cannot see (the rule
// entitymanager.Manager.PatchEntity keeps for entities). Naming the
// property replaces it like any other.
func (s *Service) Edit(ctx context.Context, addr RawAddress, seq int64, props map[string]any) (MessageView, error) {
	c, openErr := s.open(ctx, addr)
	if openErr != nil {
		return MessageView{}, openErr
	}
	if err := c.editAvailable(); err != nil {
		return MessageView{}, err
	}
	m, err := s.message(c, seq)
	if err != nil {
		return MessageView{}, err
	}
	if checkErr := c.editCheck(m); checkErr != nil {
		return MessageView{}, checkErr
	}
	mt := c.ct.MessageTypes[m.Type]
	err = s.local.limiter.take(c.now, bucketIdentity(c.p), c.key, opEdit, realtimelimits.OpRate)
	if err != nil {
		return MessageView{}, err
	}
	props, kept, err := s.keepUnreadableRefs(c, mt, m, props)
	if err != nil {
		return MessageView{}, err
	}
	vp, refs, err := s.checkMessageKeeping(c, mt, props, c.snap.Config.Limits().MessageBytes, kept)
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

// keepUnreadableRefs returns props with every ref property of m added back
// that props does not name and whose stored entity the caller cannot read,
// and the names of those properties with their ids. props itself is not
// changed.
func (s *Service) keepUnreadableRefs(
	c *call, mt *realtimecfg.MessageType, m channels.Message, props map[string]any,
) (merged map[string]any, kept map[string]string, err error) {
	stored := map[string]string{}
	var ids []string
	for name, p := range mt.Properties {
		if p.Type != realtimecfg.PropertyRef {
			continue
		}
		if _, named := props[name]; named {
			continue
		}
		if id, ok := m.Props[name].(string); ok && id != "" {
			stored[name] = id
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return props, nil, nil
	}
	readable, err := s.refs.Resolve(c.ctx, ids)
	if err != nil {
		return nil, nil, fmt.Errorf("realtime: resolve refs: %w", err)
	}
	kept = map[string]string{}
	merged = maps.Clone(props)
	if merged == nil {
		merged = map[string]any{}
	}
	for name, id := range stored {
		if _, ok := readable[id]; ok {
			continue
		}
		merged[name] = id
		kept[name] = id
	}
	return merged, kept, nil
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
	if pendErr := requireNotPending(m); pendErr != nil {
		return MessageView{}, pendErr
	}
	removal := channels.Removal{By: authorOf(c.p), At: c.now}
	ops := c.ct.MessageOps
	moderator := c.moderator()
	ownRefusal := c.deleteOwn(m)
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
// when they may. [Service.Delete] and [call.messageActions] both ask it. A
// refusal with a Rule is specific to an author who passed the gate (the
// channel is not in the `from:` state); one without a Rule means
// delete_own does not apply.
func (c *call) deleteOwn(m channels.Message) *DenialError {
	op := c.ct.MessageOps.DeleteOwn
	if op == nil || !sameAuthor(m.Author, c.p) || !c.deleteOwnGate() {
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
