package realtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// Promoter creates the entity a promoting action asks for. The wiring site
// implements it over the entity manager, as the caller, so the create ACL,
// the field gate, validation, automations and the graph audit all apply.
//
// A Promoter that can commit the entity, its relation and the action mark
// in one transaction does so and reports Marked. One that cannot creates
// the entity and leaves the mark to the service.
//
// An error means no entity was promoted, as far as the promoter knows.
// When it also wraps [ErrNothingWritten], the promoter is sure nothing was
// written, and the service releases the pending action. Otherwise the
// action stays pending until a moderator releases it. A refusal the caller
// should see is a *[DenialError].
type Promoter interface {
	Promote(ctx context.Context, r PromoteRequest) (PromoteResult, error)
	// CheckCreate reports whether the caller on ctx may create an entity
	// of typ at face, from the grants alone: it reads no store. A refusal
	// is a *[DenialError]. The service asks it once per promote target per
	// operation, so it offers a promoting action only when its create will
	// be allowed. The field grants and validation are not part of it; the
	// promote itself still checks them.
	CheckCreate(ctx context.Context, typ string, face entity.Face) error
}

// ErrNothingWritten marks a [Promoter] error after which nothing was
// written: the service may release the pending action.
var ErrNothingWritten = errors.New("realtime: promote wrote nothing")

// PromoteRequest is one promote.
type PromoteRequest struct {
	// Type and Face are the entity to create. Face is set exactly when the
	// type declares faces.
	Type string
	Face entity.Face
	// Props are the entity properties, copied from the message.
	Props map[string]any
	// Relate, when set, is the relation type from the new entity to the
	// anchor.
	Relate string
	// Anchor is the entity the channel hangs on.
	Anchor channels.Anchor
	// Origin becomes the triggered_by of the graph audit records. Its format
	// is `channel:<anchor>[@face]/<type>[/<name>]#<seq>/<action>`: the
	// channel key as [channels.Key.String] renders it, the message seq and
	// the action name.
	Origin string
	// Mark is the action write that completes the promote.
	Mark ActionMark
}

// ActionMark is the write that marks a promoting action done. A Promoter
// that commits it with the entity calls [ActionMark.Apply] on a channel
// store that joins its transaction.
type ActionMark struct {
	Key  channels.Key
	Seq  int64
	Name string
	// Done is the done action without its promoted record.
	Done channels.Action
}

// Apply moves the action from pending to done on st, recording the
// promoted entity id and type.
func (m ActionMark) Apply(ctx context.Context, st channels.Store, id, typ string) (channels.Message, error) {
	done := m.Done
	done.Promoted = &channels.Promoted{ID: id, Type: typ}
	return st.SetAction(ctx, m.Key, m.Seq, m.Name, channels.ActionPending, done)
}

// PromoteResult is a promoted entity.
type PromoteResult struct {
	ID   string
	Type string
	// Warnings are soft findings on the created entity, and steps after the
	// create that failed without undoing it.
	Warnings []entity.Warning
	// Marked is set when the promoter applied the mark in the transaction
	// that created the entity; Message is then the marked message.
	Marked  bool
	Message channels.Message
}

// NoPromoter is the [Promoter] of a process that cannot write the graph.
// Promoting actions are not offered while it is wired, because its
// [NoPromoter.CheckCreate] refuses every create.
type NoPromoter struct{}

// Promote refuses every promote.
func (NoPromoter) Promote(context.Context, PromoteRequest) (PromoteResult, error) {
	return PromoteResult{}, fmt.Errorf("%w: %w", ErrNothingWritten, notPromoting())
}

// CheckCreate refuses every create.
func (NoPromoter) CheckCreate(context.Context, string, entity.Face) error {
	return notPromoting()
}

func notPromoting() *DenialError {
	return deny(http.StatusForbidden, TypeActionNotOffered, "promoting is not available in this process")
}

// Warning codes of an action answer. A [Promoter] reports the last two.
const (
	// WarnPromoteUnconfirmed: the entity was created, but marking the
	// action done failed. The message shows the action pending until a
	// moderator releases it.
	WarnPromoteUnconfirmed = "promote_unconfirmed"
	// WarnPromoteRelateFailed: the entity was created without its relation
	// to the anchor. Only a promoter that cannot commit both together
	// reports it; elsewhere the promote fails instead.
	WarnPromoteRelateFailed = "promote_relate_failed"
	// WarnPromoteFollowUpFailed: a step after the create, such as an
	// automation or the cascade, failed. The entity stays.
	WarnPromoteFollowUpFailed = "promote_followup_failed"
)

// Timeouts of the promote steps. They run on a context that the client
// cannot cancel, so a dropped connection does not leave a half-done
// promote behind.
const (
	promoteTimeout = 30 * time.Second
	markTimeout    = 5 * time.Second
	markTries      = 3
	markBackoff    = 100 * time.Millisecond
)

// promoteProps copies the message properties p maps into entity
// properties. A message property that is absent or null is left out.
// Ref values are id strings, which checkPromoteRefs has
// checked the caller may read.
func promoteProps(p *realtimecfg.Promote, msg channels.Message) map[string]any {
	out := make(map[string]any, len(p.Map))
	for ep, mp := range p.Map {
		if v, ok := msg.Props[mp]; ok && v != nil {
			out[ep] = v
		}
	}
	return out
}

// promoteOrigin is the triggered_by of a promote; see
// [PromoteRequest.Origin].
func promoteOrigin(k channels.Key, seq int64, action string) string {
	return "channel:" + k.String() + "#" + strconv.FormatInt(seq, 10) + "/" + action
}

// promote runs the promote of action a on the pending message msg, and
// returns the message as it ends up and the warnings for the caller.
func (s *Service) promote(
	c *call, a *realtimecfg.Action, msg channels.Message, done channels.Action,
) (channels.Message, []entity.Warning, error) {
	p := a.PromoteTo
	req := PromoteRequest{
		Type: p.Type, Face: entity.Face(p.Face), Props: promoteProps(p, msg), Relate: p.Relate,
		Anchor: c.key.Anchor, Origin: promoteOrigin(c.key, msg.Seq, a.Name),
		Mark: ActionMark{Key: c.key, Seq: msg.Seq, Name: a.Name, Done: done},
	}
	// The caller's identity rides on c.ctx; only its cancellation is cut.
	pctx, cancel := context.WithTimeout(context.WithoutCancel(c.ctx), promoteTimeout)
	defer cancel()
	res, err := s.promoter.Promote(pctx, req)
	if err != nil {
		return channels.Message{}, nil, s.promoteFailed(c, msg, a.Name, err)
	}
	if res.Marked {
		return res.Message, res.Warnings, nil
	}
	marked, err := s.markDone(c, req.Mark, res.ID, res.Type)
	if err != nil {
		slog.ErrorContext(c.ctx, "realtime: promoted entity, but marking the action done failed; "+
			"the action stays pending until a moderator releases it",
			"channel", c.key.String(), "seq", msg.Seq, "action", a.Name, "entity", res.ID, "error", err)
		w := entity.Warning{
			Code:   WarnPromoteUnconfirmed,
			Detail: "the entity was created, but the message still shows the action pending",
		}
		return msg, append(res.Warnings, w), nil
	}
	return marked, res.Warnings, nil
}

// promoteFailed handles a promote error: it releases the pending action
// when the promoter wrote nothing, and returns the error to answer.
func (s *Service) promoteFailed(c *call, msg channels.Message, name string, err error) error {
	if errors.Is(err, ErrNothingWritten) {
		rctx, cancel := context.WithTimeout(context.WithoutCancel(c.ctx), markTimeout)
		defer cancel()
		_, rerr := s.store.SetAction(rctx, c.key, msg.Seq, name, channels.ActionPending,
			channels.Action{By: authorOf(c.p), At: c.now, Revert: true})
		if rerr != nil {
			slog.ErrorContext(c.ctx, "realtime: promote failed and releasing the action failed; "+
				"it stays pending until a moderator releases it",
				"channel", c.key.String(), "seq", msg.Seq, "action", name, "error", rerr)
		}
	} else {
		slog.ErrorContext(c.ctx, "realtime: promote failed after it may have written; "+
			"the action stays pending until a moderator releases it",
			"channel", c.key.String(), "seq", msg.Seq, "action", name, "error", err)
	}
	s.local.mark(c.key)
	if d, ok := errors.AsType[*DenialError](err); ok {
		return d
	}
	return fmt.Errorf("realtime: promote: %w", err)
}

// checkPromoteRefs refuses a promote of msg that would copy a ref into the
// new entity that the caller cannot read. The poster could read it, but
// the caller may not, and the entity would show them its id. The mapped
// ref properties resolve as the caller in one batch. An unreadable and a
// missing entity get the same 422, which names only the message property.
//
// A property the store recorded as a ref counts as one whatever the
// configuration now says, as it does when the message renders.
func (s *Service) checkPromoteRefs(c *call, p *realtimecfg.Promote, msg channels.Message) error {
	mt := c.ct.MessageTypes[msg.Type]
	if mt == nil || len(p.Map) == 0 {
		return nil
	}
	stored, err := s.local.storedRefs(c.ctx, s.store, c.snap.Config, c.ct.Name)
	if err != nil {
		return err
	}
	byProp := map[string]string{}
	for _, mp := range p.Map {
		id, ok := msg.Props[mp].(string)
		if !ok || id == "" {
			continue
		}
		prop, declared := mt.Properties[mp]
		if (declared && prop.Type == realtimecfg.PropertyRef) || slices.Contains(stored, msg.Type+"."+mp) {
			byProp[mp] = id
		}
	}
	if len(byProp) == 0 {
		return nil
	}
	ids := make([]string, 0, len(byProp))
	for _, id := range byProp {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	got, err := s.refs.Resolve(c.ctx, slices.Compact(ids))
	if err != nil {
		return fmt.Errorf("realtime: resolve refs: %w", err)
	}
	for _, mp := range sortedKeys(byProp) {
		if _, ok := got[byProp[mp]]; !ok {
			return deny(http.StatusUnprocessableEntity, TypePromoteInvalid,
				"the message property "+mp+" names an entity that cannot be promoted")
		}
	}
	return nil
}

// markDone applies m with a few tries, on a context the client cannot
// cancel: the entity exists, so the mark should land if it can.
func (s *Service) markDone(c *call, m ActionMark, id, typ string) (channels.Message, error) {
	var err error
	for try := range markTries {
		if try > 0 {
			time.Sleep(markBackoff << (try - 1))
		}
		mctx, cancel := context.WithTimeout(context.WithoutCancel(c.ctx), markTimeout)
		var got channels.Message
		got, err = m.Apply(mctx, s.store, id, typ)
		cancel()
		if err == nil || errors.Is(err, channels.ErrConflict) || errors.Is(err, channels.ErrNotFound) {
			// A conflict or a missing message will not heal by retrying.
			return got, err
		}
	}
	return channels.Message{}, err
}
