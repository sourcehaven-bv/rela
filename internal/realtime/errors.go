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

// DenialError types: the stable `type` of an error answer.
const (
	TypeUnknownChannelType = "unknown_channel_type"
	TypeChannelNotFound    = "channel_not_found"
	TypeMessageNotFound    = "message_not_found"
	TypeFaceRequired       = "face_required"
	TypeNoPrincipal        = "no_principal"
	TypeReadDenied         = "channel_read_denied"
	TypePostDenied         = "channel_post_denied"
	TypeCreateDenied       = "channel_create_denied"
	TypeEditDenied         = "message_edit_denied"
	TypeEditWindowClosed   = "edit_window_closed"
	TypeDeleteDenied       = "message_delete_denied"
	TypeChannelNotOpen     = "channel_not_open"
	TypeNotCreatable       = "channel_not_creatable"
	TypeRemoved            = "removed"
	TypeMessageTypeGone    = "message_type_gone"
	TypeTooManyChannels    = "too_many_channels"
	TypeConflict           = "conflict"
	TypeInvalid            = "invalid_message"
	TypeTooManyProps       = "too_many_props"
	TypeReasonRequired     = "reason_required"
	TypeTooLarge           = "too_large"
	TypeInvalidRef         = "invalid_ref"
	TypeRateLimited        = "rate_limited"
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

// storeError maps a channel store error on a write to a [DenialError], and
// returns any other error unchanged.
func storeError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, channels.ErrRemoved):
		return deny(http.StatusConflict, TypeRemoved, "the message was removed")
	case errors.Is(err, channels.ErrTooManyChannels):
		return deny(http.StatusConflict, TypeTooManyChannels, "this entity holds the most channels of this type allowed")
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
