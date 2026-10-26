package realtime

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// systemWriter makes the channel writes no request asks for. It is kept
// apart from [Service] so that no HTTP or MCP path can reach it.
type systemWriter struct {
	store   channels.Store
	local   *Local
	cfg     ConfigSource
	dueWake *DueWake
}

// post appends a message of mt with props to key as [systemAuthor], naming
// triggeredBy. marked reports that the post moved the session and the move
// wrote an outputs mark (see [triggerMark]). A post that can never succeed
// returns a permanentError.
// It charges no rate bucket: the trigger engine caps its own
// posts per round. With requireOpen the store refuses a channel that is
// not open with [channels.ErrNotOpen], in the same step as the append. A
// missing cardinality-one channel is created.
//
// The properties and the type's `when:` are checked again although
// realtimecfg checked both at load, because the message is the service's
// to keep valid whoever writes it.
func (w systemWriter) post(
	ctx context.Context, snap Snapshot, now time.Time, key channels.Key, ct realtimecfg.ChannelType,
	mt *realtimecfg.MessageType, props map[string]any, triggeredBy string, requireOpen bool,
) (marked bool, err error) {
	vp, err := validateProps(mt, props, snap.Config.Limits().MessageBytes)
	if err != nil {
		return false, permanentError{err}
	}
	if !whenHolds(ctx, mt, vp.props, now) {
		return false, permanentError{
			fmt.Errorf("realtime: the message does not meet the when: condition of %s", mt.Name),
		}
	}
	if recErr := recordType(ctx, w.store, w.local, w.cfg, snap, ct); recErr != nil {
		var denial *DenialError
		if errors.As(recErr, &denial) && denial.Type == TypeChannelTypeConflict {
			return false, permanentError{recErr}
		}
		return false, recErr
	}
	// A trigger's post fires a transition with by: trigger or with no
	// poster rule. With requireOpen off it may land in a closed channel,
	// which the decider never moves.
	sp := newSessionPost(ctx, ct, mt, vp.props, now, poster{trigger: true}, triggeredBy)
	if sp.step != nil {
		sp.step.Mark = triggerMark(ct, mt.Name, now, &marked)
	}
	_, err = w.store.Append(ctx, key, channels.NewMessage{
		Type: mt.Name, Author: systemAuthor(), At: now, Props: vp.props, TriggeredBy: triggeredBy,
		RequireOpen: requireOpen, Session: sp.step,
	})
	sp.logGuardFailure(ctx)
	if isPermanent(err) {
		return false, permanentError{err}
	}
	if err != nil {
		return false, err
	}
	noteDue(w.dueWake, sp.step)
	w.local.mark(key)
	moved := sp.step != nil && sp.step.Outcome != nil && sp.step.Outcome.Moved
	return marked && moved, nil
}

// moves is what one systemWriter.transition did.
type moves struct {
	// tried counts the channels it tried to move, moved those it moved.
	tried, moved int
	// more reports that channels are left that limit kept it from trying.
	more bool
}

// transition runs the automatic operation a on the channels of type ct
// at anchor whose state is one of a.from, as [systemAuthor], naming
// triggeredBy. It tries at most limit channels. Each move goes through
// applyTransition, the compare-and-set the sweep and requests use, so a
// channel a person or another process moved first is skipped rather than
// moved again. A channel that was never written does not exist and is not
// created. After a failure, or with more left, a second call moves the
// rest: the channels already moved have left a.from.
func (w systemWriter) transition(
	ctx context.Context, now time.Time, anchor channels.Anchor, ct realtimecfg.ChannelType, a *autoRule,
	triggeredBy string, limit int,
) (moves, error) {
	var res moves
	list, err := w.store.List(ctx, anchor, ct.Name, "")
	if isPermanent(err) {
		return res, permanentError{err}
	}
	if err != nil {
		return res, err
	}
	for _, c := range list {
		if !slices.Contains(a.from, c.State) {
			continue
		}
		if res.tried >= limit {
			res.more = true
			return res, nil
		}
		res.tried++
		moved, err := applyTransition(ctx, w.store, w.local, c.Key, c.State, channels.Transition{
			To: a.to, By: systemAuthor(), At: now, TriggeredBy: triggeredBy,
			SessionDue: transitionSchedule(ct, a.to, now),
		})
		switch {
		case err == nil:
			res.moved++
			if a.to == channels.StateOpen {
				w.dueWake.Note(moved.Session.NextDueAt)
			}
		case errors.Is(err, channels.ErrConflict), errors.Is(err, channels.ErrNotFound):
		case isPermanent(err):
			return res, permanentError{err}
		default:
			return res, err
		}
	}
	return res, nil
}

// isPermanent reports whether a channel write error is about this one
// write and fails the same way on every try, rather than about the store
// as a whole.
func isPermanent(err error) bool {
	for _, target := range []error{
		channels.ErrTooManyChannels, channels.ErrTooLarge, channels.ErrStaleAnchor,
		channels.ErrInvalidKey, channels.ErrInvalidMessage, channels.ErrTypeShapeConflict,
	} {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// permanentError is a post that fails the same way however often it is
// tried, so the engine gives it up at once rather than retrying.
type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }
