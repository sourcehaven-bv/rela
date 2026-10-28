package realtime

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

// ErrAnchorNotFound reports an anchor entity that does not exist, that the
// caller may not read, or that is not of a type the channel type attaches
// to. The HTTP layer answers it with the same body as the entity GET's
// not-found, so a channel route tells a hidden entity from a missing one no
// better than the entity route does.
var ErrAnchorNotFound = errors.New("realtime: anchor entity not found")

// ErrNoPrincipal is what an [AccessSource] wraps when the request carries
// no identity the ACL can evaluate, such as an unstamped principal under a
// declarative policy. The service answers it with a 403 no_principal; any
// other error from the source is a server error.
var ErrNoPrincipal = errors.New("realtime: the request carries no identity the ACL can evaluate")

// DenialError types: the stable `type` of an error answer.
const (
	TypeUnknownChannelType = "unknown_channel_type"
	// TypeNoSession is a 404 for a session read of a channel type that
	// declares no session.
	TypeNoSession        = "no_session"
	TypeChannelNotFound  = "channel_not_found"
	TypeMessageNotFound  = "message_not_found"
	TypeFaceRequired     = "face_required"
	TypeNoPrincipal      = "no_principal"
	TypeReadDenied       = "channel_read_denied"
	TypePostDenied       = "channel_post_denied"
	TypeCreateDenied     = "channel_create_denied"
	TypeEditDenied       = "message_edit_denied"
	TypeEditWindowClosed = "edit_window_closed"
	TypeDeleteDenied     = "message_delete_denied"
	TypeChannelNotOpen   = "channel_not_open"
	TypeNotCreatable     = "channel_not_creatable"
	TypeRemoved          = "removed"
	TypeMessageTypeGone  = "message_type_gone"
	TypeTooManyChannels  = "too_many_channels"
	TypeConflict         = "conflict"
	TypeInvalid          = "invalid_message"
	TypeTooManyProps     = "too_many_props"
	TypeReasonRequired   = "reason_required"
	TypeTooLarge         = "too_large"
	TypeInvalidRef       = "invalid_ref"
	TypeRateLimited      = "rate_limited"
	// TypeLifecycleNotOffered is a 403 for a lifecycle operation the
	// channel type does not declare.
	TypeLifecycleNotOffered = "lifecycle_not_offered"
	// TypeTypingNotOffered is a 403 for typing in a channel whose type
	// does not set typing: true.
	TypeTypingNotOffered = "typing_not_offered"
	// TypeLifecycleDenied is a 403 for a declared lifecycle operation whose
	// gate or condition refuses the caller.
	TypeLifecycleDenied = "lifecycle_denied"
	// TypeLifecycleWrongState is a 409 for a lifecycle operation that does
	// not start from the channel's current state.
	TypeLifecycleWrongState = "lifecycle_wrong_state"
	// TypeRetentionActive is a 409 for a delete before the channel's
	// retention ends, or of a channel under retention that was never closed.
	TypeRetentionActive = "retention_active"
	// TypeActionNotOffered is a 403 for an action the message type does not
	// declare, or a promoting action in a process that cannot promote.
	TypeActionNotOffered = "action_not_offered"
	// TypeActionDenied is a 403 for an action, or a release, whose gate
	// refuses the caller.
	TypeActionDenied = "action_denied"
	// TypeActionTaken is a 409 for an action on a message that already
	// carries one, or that lost the race for it.
	TypeActionTaken = "action_taken"
	// TypeActionPending is a 409 for an action, edit or removal of a message
	// whose action is pending, and for a release before the action has been
	// pending long enough.
	TypeActionPending = "action_pending"
	// TypeActionNotPending is a 409 for a release of a message whose action
	// is not pending.
	TypeActionNotPending = "action_not_pending"
	// TypePromoteDenied is a 403 for a promote the entity create ACL or the
	// field grants refuse.
	TypePromoteDenied = "promote_denied"
	// TypePromoteInvalid is a 422 for a promote whose entity or relation
	// fails validation.
	TypePromoteInvalid = "promote_invalid"
	// TypePromoteConflict is a 409 for a promote whose entity or relation
	// already exists.
	TypePromoteConflict = "promote_conflict"
	// TypeChannelTypeConflict is a 409 for a write to a channel type whose
	// record in the channel store differs from realtime.yaml: another
	// anchor or cardinality, or a recorded ref declared as another type.
	// Writes to the type stay refused until realtime.yaml matches the
	// record again. The HTTP layer (T3b) answers it as 409
	// channel_type_conflict, with the title naming the type.
	TypeChannelTypeConflict = "channel_type_conflict"
	// TypeConfigChanged is a 409 for a write during which realtime.yaml was
	// reloaded. Nothing was written; a retry runs under the new config.
	TypeConfigChanged = "config_changed"
	// TypeSignalsNotKept is a 422 for a signal posted through a transport
	// in another process than the server (see [PostInput.LogOnly]).
	TypeSignalsNotKept = "signals_not_kept"
	// TypeSessionNotice is a 409 for an edit or removal of a session
	// notice: it records a transition, which stays recorded.
	TypeSessionNotice = "session_notice"
	// TypeSessionMessage is a 409 for an author's edit or delete of a
	// message that moved the session. A moderator may still remove it.
	TypeSessionMessage = "session_message"
	// TypeReminderMessage is a 409 for an edit or author delete of a
	// message a session reminder posted. A moderator may still remove it.
	TypeReminderMessage = "reminder_message"
	// TypeSessionOutputDenied is a 403 for a post that would fire a
	// transition whose outputs the poster may not write, or whose outputs
	// cannot run now. The title names the output's config key.
	TypeSessionOutputDenied = "session_output_denied"
	// TypeOutputsPending is a 409 for a release of a session notice's
	// outputs mark, or a removal of the message that caused a move whose
	// outputs are still pending.
	TypeOutputsPending = "outputs_pending"
	// TypeInvalidCursor: an inbox cursor no inbox answer produced.
	TypeInvalidCursor = "invalid_cursor"
	// TypeInboxTooLarge: the channels an inbox would have to judge exceed
	// [channels.MaxWaitingRows]. Nothing is answered, because a partial
	// inbox would read as a complete one.
	TypeInboxTooLarge = "inbox_too_large"
	// TypeTooManyMentions is a 422 for a post or edit that addresses more
	// than [channels.MaxMentions] principals.
	TypeTooManyMentions = "too_many_mentions"
)

// DenialError is a refused operation: an HTTP status, a stable type, a title for
// people and, for a 403, the rule that refused. RetryAfter is set for a
// rate limit (429) and says when to try again.
//
// Titles may name channel types, roles and permissions: realtime.yaml is
// configuration, which is not secret. They never name an entity or a
// message beyond what the caller addressed.
type DenialError struct {
	Status     int
	Type       string
	Title      string
	Rule       string
	RetryAfter time.Duration
}

func (d *DenialError) Error() string {
	if d.Rule != "" {
		return fmt.Sprintf("realtime: %s: %s (%s)", d.Type, d.Title, d.Rule)
	}
	return fmt.Sprintf("realtime: %s: %s", d.Type, d.Title)
}

func deny(status int, typ, title string) *DenialError {
	return &DenialError{Status: status, Type: typ, Title: title}
}

func forbidden(typ, title, rule string) *DenialError {
	return &DenialError{Status: http.StatusForbidden, Type: typ, Title: title, Rule: rule}
}

func rateLimited(after time.Duration) *DenialError {
	if after < time.Second {
		after = time.Second
	}
	return &DenialError{
		Status: http.StatusTooManyRequests, Type: TypeRateLimited,
		Title: "too many requests; try again later", RetryAfter: after,
	}
}

func unknownChannelType(name string) *DenialError {
	return deny(http.StatusNotFound, TypeUnknownChannelType, fmt.Sprintf("no channel type %q here", name))
}

func channelNotFound() *DenialError {
	return deny(http.StatusNotFound, TypeChannelNotFound, "channel not found")
}

func messageNotFound() *DenialError {
	return deny(http.StatusNotFound, TypeMessageNotFound, "message not found")
}

func invalid(format string, args ...any) *DenialError {
	return deny(http.StatusBadRequest, TypeInvalid, fmt.Sprintf(format, args...))
}

func invalidRef() *DenialError {
	// One body for an unreadable and a missing entity, so a post cannot
	// probe which ids exist.
	return deny(http.StatusUnprocessableEntity, TypeInvalidRef,
		"a referenced entity does not exist, cannot be read, or is not of an allowed type")
}

// typeConflict is the 409 for a write to channel type typ whose store
// record conflicts with the configuration.
func typeConflict(typ string) *DenialError {
	return &DenialError{
		Status: http.StatusConflict, Type: TypeChannelTypeConflict,
		Title: fmt.Sprintf("channel type %s is recorded with another anchor, cardinality or ref properties "+
			"than realtime.yaml declares; writes stay refused until they match", typ),
		Rule: "channel_types." + typ,
	}
}

// tooManyMentions is the 422 for a message that addresses more than
// [channels.MaxMentions] principals. It names no target, so it tells the
// poster nothing about who they may not see.
func tooManyMentions() *DenialError {
	return deny(http.StatusUnprocessableEntity, TypeTooManyMentions, fmt.Sprintf(
		"a message may mention at most %d people or agents", channels.MaxMentions))
}

func signalsNotKept(typ string) *DenialError {
	return deny(http.StatusUnprocessableEntity, TypeSignalsNotKept, fmt.Sprintf(
		"message type %s is a signal; signals live in the server process that receives them, "+
			"so post them through rela-server", typ))
}

func configChanged() *DenialError {
	return deny(http.StatusConflict, TypeConfigChanged, "realtime.yaml was reloaded during the request; try again")
}

// messageStoreError is [storeError] for an operation on one message: a
// missing channel or message is message_not_found.
func messageStoreError(err error) error {
	if errors.Is(err, channels.ErrNotFound) {
		return messageNotFound()
	}
	return storeError(err)
}

// actionStoreError is [messageStoreError] for an action write: a lost
// compare-and-set means another action got there first. A pending action
// that got there first is [channels.ErrActionPending], which
// [storeError] answers as action_pending.
func actionStoreError(err error) error {
	if errors.Is(err, channels.ErrConflict) {
		return deny(http.StatusConflict, TypeActionTaken, "another action on this message got there first")
	}
	return messageStoreError(err)
}

// storeError maps a channel store error on a write to a [DenialError], and
// returns any other error unchanged. A missing channel is
// channel_not_found: it was deleted after the operation read it.
func storeError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, channels.ErrNotFound):
		return channelNotFound()
	case errors.Is(err, channels.ErrRemoved):
		return deny(http.StatusConflict, TypeRemoved, "the message was removed")
	case errors.Is(err, channels.ErrTooManyChannels):
		return deny(http.StatusConflict, TypeTooManyChannels, "this entity holds the most channels of this type allowed")
	case errors.Is(err, channels.ErrNotOpen):
		return deny(http.StatusConflict, TypeChannelNotOpen, "the channel is not open")
	case errors.Is(err, channels.ErrActionPending):
		return deny(http.StatusConflict, TypeActionPending, "the message has a pending action")
	case errors.Is(err, channels.ErrConflict):
		return deny(http.StatusConflict, TypeConflict, "the channel changed concurrently; try again")
	case errors.Is(err, channels.ErrTooLarge):
		return deny(http.StatusRequestEntityTooLarge, TypeTooLarge, "a value is over its size limit")
	case errors.Is(err, channels.ErrInvalidReply):
		return invalid("reply_to must name a top-level message that was not removed")
	case errors.Is(err, channels.ErrInvalidPage):
		return invalid("name at most one of after, before and after_rev, none negative")
	case errors.Is(err, channels.ErrInvalidMessage):
		return invalid("the message is not valid")
	}
	return err
}
