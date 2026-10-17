package realtime

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// Lifecycle runs the lifecycle operation op (close, reopen, archive or
// delete) on the channel at addr, and returns the channel as it is after
// the change. A delete returns the zero ChannelView.
//
// The checks run in this order after the read check: the channel exists
// (a cardinality-one channel that holds no message has nothing to act
// on), the operation is declared, the caller has an identity, the gate,
// the state the operation starts from, the condition, the delete
// retention and the rate limit. [Service.Head] offers an operation in its
// affordances exactly when every check but the rate limit passes.
//
// The write is a compare-and-set on the state the checks judged, and a
// delete also on the rev, so a concurrent change answers 409 conflict
// rather than bypassing `from:` or the retention.
//
// Deleting a cardinality-one channel erases its history; the next post
// starts a new channel.
func (s *Service) Lifecycle(ctx context.Context, addr RawAddress, op string) (ChannelView, error) {
	c, err := s.open(ctx, addr)
	if err != nil {
		return ChannelView{}, err
	}
	if c.head == nil {
		return ChannelView{}, channelNotFound()
	}
	target, err := c.lifecycleCheck(op)
	if err != nil {
		return ChannelView{}, err
	}
	if err := s.local.limiter.take(c.now, bucketIdentity(c.p), c.key, opLifecycle, realtimelimits.OpRate); err != nil {
		return ChannelView{}, err
	}
	if op == realtimecfg.LifecycleDelete {
		err := s.store.Delete(c.ctx, c.key, channels.Deletion{
			Expect: c.head.State, Rev: c.head.Rev, By: authorOf(c.p), At: c.now,
		})
		if err != nil {
			return ChannelView{}, storeError(err)
		}
		s.local.mark(c.key)
		return ChannelView{}, nil
	}
	ch, err := s.store.SetLifecycle(c.ctx, c.key, c.head.State,
		channels.Transition{To: target, By: authorOf(c.p), At: c.now})
	if err != nil {
		return ChannelView{}, storeError(err)
	}
	s.local.mark(c.key)
	return channelView(ch.Key, &ch), nil
}

// lifecycleOps lists the lifecycle operations the caller may run on c's
// channel now, in [realtimecfg.LifecycleOpNames] order. It asks
// [call.lifecycleCheck], the function the POST asks, so the affordance and
// the answer agree.
func (c *call) lifecycleOps() []string {
	out := []string{}
	if c.head == nil {
		return out
	}
	for _, name := range realtimecfg.LifecycleOpNames() {
		if _, err := c.lifecycleCheck(name); err == nil {
			out = append(out, name)
		}
	}
	return out
}

// lifecycleCheck decides whether the caller may run the lifecycle
// operation name on c's stored channel, and returns the state it moves to
// (empty for delete). c.head must be set.
func (c *call) lifecycleCheck(name string) (channels.State, error) {
	rule := "channel_types." + c.ct.Name + ".lifecycle." + name
	op := c.ct.Lifecycle.Op(name)
	if op == nil {
		return "", forbidden(TypeLifecycleNotOffered,
			fmt.Sprintf("%s is not declared for %s channels", name, c.ct.Name), rule)
	}
	if err := c.requireCaller(); err != nil {
		return "", err
	}
	if !c.gate(op.Gate) {
		return "", gateDenied(TypeLifecycleDenied, name, rule, op.Gate, c.ct.Roles)
	}
	from := naturalFrom(name)
	if op.From != "" {
		from = []channels.State{channels.State(op.From)}
	}
	if !slices.Contains(from, c.head.State) {
		names := make([]string, len(from))
		for i, st := range from {
			names[i] = string(st)
		}
		return "", deny(http.StatusConflict, TypeLifecycleWrongState,
			fmt.Sprintf("the channel is %s; %s needs %s", c.head.State, name, strings.Join(names, " or ")))
	}
	if len(op.Conditions) > 0 {
		if err := c.lifecycleCondition(op, name, rule); err != nil {
			return "", err
		}
	}
	if name == realtimecfg.LifecycleDelete && !op.Retention.IsZero() {
		if err := retentionCheck(op.Retention, c.head.ClosedAt, c.now); err != nil {
			return "", err
		}
	}
	return lifecycleTarget(name), nil
}

// lifecycleCondition refuses an operation whose `condition:` does not hold
// for the anchor. It is judged on the redacted record the gated anchor
// read returned, so a property the caller may not see reads as absent.
// While realtime.yaml is degraded every condition refuses: the acl.yaml
// that broke it may restrict a property the condition reads.
func (c *call) lifecycleCondition(op *realtimecfg.LifecycleOp, name, rule string) error {
	rule += ".condition"
	if c.snap.Degraded {
		return forbidden(TypeLifecycleDenied,
			"conditions are suspended while realtime.yaml is degraded", rule)
	}
	if !c.conditionHolds(op.Conditions[c.anchor.Type]) {
		title := op.Denied
		if title == "" {
			title = fmt.Sprintf("the entity does not meet the condition of %s", name)
		}
		return forbidden(TypeLifecycleDenied, title, rule)
	}
	return nil
}

// retentionCheck refuses a delete while retention keeps the channel: until
// retention has passed since it closed. A channel that never closed has
// not started its retention, so it is refused too.
func retentionCheck(retention realtimecfg.Period, closedAt, now time.Time) error {
	if closedAt.IsZero() {
		return deny(http.StatusConflict, TypeRetentionActive,
			fmt.Sprintf("retention keeps this channel for %s after it closes; close it first", retention))
	}
	if until := retention.AddTo(closedAt); now.Before(until) {
		return deny(http.StatusConflict, TypeRetentionActive,
			"retention keeps this channel until "+until.UTC().Format(time.RFC3339))
	}
	return nil
}

// naturalFrom is the states an operation starts from when its `from:` is
// not set.
func naturalFrom(name string) []channels.State {
	switch name {
	case realtimecfg.LifecycleClose:
		return []channels.State{channels.StateOpen}
	case realtimecfg.LifecycleReopen:
		return []channels.State{channels.StateClosed, channels.StateArchived}
	case realtimecfg.LifecycleArchive:
		return []channels.State{channels.StateOpen, channels.StateClosed}
	default:
		return []channels.State{channels.StateOpen, channels.StateClosed, channels.StateArchived}
	}
}

// lifecycleTarget is the state an operation moves to; empty for delete.
func lifecycleTarget(name string) channels.State {
	switch name {
	case realtimecfg.LifecycleClose:
		return channels.StateClosed
	case realtimecfg.LifecycleReopen:
		return channels.StateOpen
	case realtimecfg.LifecycleArchive:
		return channels.StateArchived
	default:
		return ""
	}
}

// anchorProps lists the anchor properties the roles and the lifecycle
// conditions of ct read: the properties the anchor read must return.
func anchorProps(ct realtimecfg.ChannelType) []string {
	out := roleProps(ct)
	for _, name := range realtimecfg.LifecycleOpNames() {
		op := ct.Lifecycle.Op(name)
		if op == nil {
			continue
		}
		for _, prog := range op.Conditions {
			out = append(out, prog.Attributes("entity")...)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}
