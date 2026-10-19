package realtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// ActionResult is the message after an action, and the warnings of a
// promote that succeeded with findings.
type ActionResult struct {
	Message  MessageView
	Warnings []entity.Warning
}

// RunAction runs the action name on message seq of the channel at addr.
//
// The checks run in this order after the read check: the message exists
// and was not removed, its type declares the action, the caller has an
// identity, the action's gate, the channel is open, the message carries no
// action yet, and the rate limit. [MessageView.Actions] offers an action
// exactly when every check but the rate limit passes; both ask
// [call.actionCheck].
//
// A message carries one action. A non-promoting action is one
// compare-and-set from none to done. A promoting action first moves the
// message to pending, which refuses every concurrent action, edit and
// removal, then asks the [Promoter] to create the entity from the message
// as the pending write returned it, then marks the action done with the
// promoted entity. See [Promoter] for what happens when a step fails.
func (s *Service) RunAction(ctx context.Context, addr RawAddress, seq int64, name string) (ActionResult, error) {
	c, err := s.open(ctx, addr)
	if err != nil {
		return ActionResult{}, err
	}
	m, err := s.message(c, seq)
	if err != nil {
		return ActionResult{}, err
	}
	a, err := c.actionCheck(m, name)
	if err != nil {
		return ActionResult{}, err
	}
	err = s.local.limiter.take(c.now, bucketIdentity(c.p), c.key, opAction, realtimelimits.OpRate)
	if err != nil {
		return ActionResult{}, err
	}
	by := authorOf(c.p)
	if a.PromoteTo == nil {
		//nolint:contextcheck // c.ctx is ctx with the acl.Request attached; see .golangci.yml on realtime.
		got, serr := s.store.SetAction(c.ctx, c.key, seq, name, channels.ActionNone,
			channels.Action{Status: channels.ActionDone, By: by, At: c.now})
		if serr != nil {
			return ActionResult{}, actionStoreError(serr)
		}
		s.local.mark(c.key)
		return s.actionResult(c, got, nil)
	}
	//nolint:contextcheck // c.ctx is ctx with the acl.Request attached; see .golangci.yml on realtime.
	pending, err := s.store.SetAction(c.ctx, c.key, seq, name, channels.ActionNone,
		channels.Action{Status: channels.ActionPending, By: by, At: c.now, PendingAt: c.now})
	if err != nil {
		return ActionResult{}, actionStoreError(err)
	}
	s.local.mark(c.key)
	done := channels.Action{Status: channels.ActionDone, By: by, At: c.now, PendingAt: c.now}
	//nolint:contextcheck // c.ctx is ctx with the acl.Request attached; see .golangci.yml on realtime.
	got, warnings, err := s.promote(c, a, pending, done)
	if err != nil {
		return ActionResult{}, err
	}
	s.local.mark(c.key)
	return s.actionResult(c, got, warnings)
}

// actionResult renders m after an action. The renderer resolves the
// promoted entity as the caller sees it.
func (s *Service) actionResult(c *call, m channels.Message, warnings []entity.Warning) (ActionResult, error) {
	r, err := s.renderer(c, []channels.Message{m}, nil)
	if err != nil {
		return ActionResult{}, err
	}
	return ActionResult{Message: r.message(m), Warnings: warnings}, nil
}

// ReleaseAction clears the pending action of message seq, so the message
// can carry an action again. It is for a promote that failed in a way the
// service could not undo: its entity may exist, so a person decides.
//
// The gate is message_ops.remove when the channel type declares it, and
// otherwise the gate of the pending action. The action must have been
// pending for [realtimelimits.PendingRelease], so a promote still running
// is not released under it. The release writes an audit record with status
// released. It works in every channel state.
func (s *Service) ReleaseAction(ctx context.Context, addr RawAddress, seq int64) (MessageView, error) {
	c, err := s.open(ctx, addr)
	if err != nil {
		return MessageView{}, err
	}
	if callerErr := c.requireCaller(); callerErr != nil {
		return MessageView{}, callerErr
	}
	m, err := s.message(c, seq)
	if err != nil {
		return MessageView{}, err
	}
	if checkErr := c.releaseCheck(m); checkErr != nil {
		return MessageView{}, checkErr
	}
	err = s.local.limiter.take(c.now, bucketIdentity(c.p), c.key, opAction, realtimelimits.OpRate)
	if err != nil {
		return MessageView{}, err
	}
	//nolint:contextcheck // c.ctx is ctx with the acl.Request attached; see .golangci.yml on realtime.
	got, err := s.store.SetAction(c.ctx, c.key, seq, m.Action.Name, channels.ActionPending,
		channels.Action{By: authorOf(c.p), At: c.now})
	if errors.Is(err, channels.ErrConflict) {
		return MessageView{}, deny(http.StatusConflict, TypeActionNotPending,
			"the action stopped being pending; read the message again")
	}
	if err != nil {
		return MessageView{}, messageStoreError(err)
	}
	s.local.mark(c.key)
	return s.rendererWith(c, nil).message(got), nil
}

// releaseCheck decides whether the caller may release the pending action
// of m now.
func (c *call) releaseCheck(m channels.Message) error {
	ops := c.ct.MessageOps
	if ops.Remove != nil && !c.gate(ops.Remove.Gate) {
		return gateDenied(TypeActionDenied, "releasing an action",
			"channel_types."+c.ct.Name+".message_ops.remove", ops.Remove.Gate, c.ct.Roles)
	}
	if m.ActionStatus() != channels.ActionPending {
		return deny(http.StatusConflict, TypeActionNotPending, "the message has no pending action")
	}
	if ops.Remove == nil {
		rule := actionRule(c.ct, m.Type, m.Action.Name)
		a := c.declaredAction(m.Type, m.Action.Name)
		if a == nil {
			return forbidden(TypeActionDenied,
				"the pending action is no longer declared; only a moderator can release it", rule)
		}
		if !c.gate(a.Gate) {
			return gateDenied(TypeActionDenied, "releasing "+a.Name, rule, a.Gate, c.ct.Roles)
		}
	}
	if after := m.Action.PendingAt.Add(realtimelimits.PendingRelease); c.now.Before(after) {
		return deny(http.StatusConflict, TypeActionPending, fmt.Sprintf(
			"the action is pending since %s; it can be released after %s",
			m.Action.PendingAt.UTC().Format(time.RFC3339), after.UTC().Format(time.RFC3339)))
	}
	return nil
}

// declaredAction is the action name of message type typ, or nil.
func (c *call) declaredAction(typ, name string) *realtimecfg.Action {
	mt := c.ct.MessageTypes[typ]
	if mt == nil {
		return nil
	}
	return mt.Actions[name]
}

func actionRule(ct realtimecfg.ChannelType, typ, name string) string {
	return "channel_types." + ct.Name + ".message_types." + typ + ".actions." + name
}

// actionCheck decides whether the caller may run the action name on m now,
// and returns the action. m is a stored message that was not removed. The
// gate answers are cached on c, so a page asks each gate once.
func (c *call) actionCheck(m channels.Message, name string) (*realtimecfg.Action, error) {
	rule := actionRule(c.ct, m.Type, name)
	a := c.declaredAction(m.Type, name)
	if a == nil {
		return nil, forbidden(TypeActionNotOffered,
			fmt.Sprintf("message type %s does not declare the action %s", m.Type, name), rule)
	}
	if a.PromoteTo != nil && !c.promotes {
		return nil, forbidden(TypeActionNotOffered, "promoting is not available in this process", rule)
	}
	if err := c.requireCaller(); err != nil {
		return nil, err
	}
	if !c.actionGate(m.Type, a) {
		return nil, gateDenied(TypeActionDenied, "running "+name, rule, a.Gate, c.ct.Roles)
	}
	if err := c.requireOpen(); err != nil {
		return nil, err
	}
	switch m.ActionStatus() {
	case channels.ActionNone:
	case channels.ActionDone:
		return nil, deny(http.StatusConflict, TypeActionTaken, "the message already carries the action "+m.Action.Name)
	case channels.ActionPending:
		return nil, deny(http.StatusConflict, TypeActionPending,
			"the action "+m.Action.Name+" is pending on this message")
	}
	return a, nil
}

// actionGate is c.gate(a.Gate), asked once per action per call.
func (c *call) actionGate(typ string, a *realtimecfg.Action) bool {
	key := typ + "\x00" + a.Name
	if ok, seen := c.actionGates[key]; seen {
		return ok
	}
	if c.actionGates == nil {
		c.actionGates = map[string]bool{}
	}
	ok := c.gate(a.Gate)
	c.actionGates[key] = ok
	return ok
}

// offeredActions lists the actions the caller may run on m now, sorted. It
// asks [call.actionCheck], the function the POST asks, so the affordance
// and the answer agree. A message that cannot carry an action gets nil
// without a gate being asked.
func (c *call) offeredActions(m channels.Message) []string {
	if m.Removed != nil || m.Action != nil || c.state() != channels.StateOpen {
		return nil
	}
	mt := c.ct.MessageTypes[m.Type]
	if mt == nil || len(mt.Actions) == 0 {
		return nil
	}
	var out []string
	for _, name := range sortedKeys(mt.Actions) {
		if _, err := c.actionCheck(m, name); err == nil {
			out = append(out, name)
		}
	}
	return out
}
