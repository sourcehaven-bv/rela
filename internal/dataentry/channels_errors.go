package dataentry

import (
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// channelErrorStatus is the HTTP status of every refusal type the channel
// service returns. It is the one place the channel routes' error contract
// is written down; TestChannelErrorStatus_CoversEveryType fails when the
// service gains a type this table lacks.
//
// The 404s split in two. A miss on the anchor entity (hidden, missing,
// another type) is not here: it is the entity GET's uniform 404 (see
// writeChannelError). The 404s below are about configuration (no such
// channel type for this entity type, which is not a secret) or about a
// channel or message the caller has already passed the read role for.
var channelErrorStatus = map[string]int{
	realtime.TypeUnknownChannelType:  http.StatusNotFound,
	realtime.TypeChannelNotFound:     http.StatusNotFound,
	realtime.TypeMessageNotFound:     http.StatusNotFound,
	realtime.TypeFaceRequired:        http.StatusUnprocessableEntity,
	realtime.TypeNoPrincipal:         http.StatusForbidden,
	realtime.TypeReadDenied:          http.StatusForbidden,
	realtime.TypePostDenied:          http.StatusForbidden,
	realtime.TypeCreateDenied:        http.StatusForbidden,
	realtime.TypeEditDenied:          http.StatusForbidden,
	realtime.TypeEditWindowClosed:    http.StatusForbidden,
	realtime.TypeDeleteDenied:        http.StatusForbidden,
	realtime.TypeChannelNotOpen:      http.StatusConflict,
	realtime.TypeNotCreatable:        http.StatusConflict,
	realtime.TypeRemoved:             http.StatusConflict,
	realtime.TypeMessageTypeGone:     http.StatusConflict,
	realtime.TypeTooManyChannels:     http.StatusConflict,
	realtime.TypeConflict:            http.StatusConflict,
	realtime.TypeChannelTypeConflict: http.StatusConflict,
	realtime.TypeConfigChanged:       http.StatusConflict,
	realtime.TypeLifecycleNotOffered: http.StatusForbidden,
	realtime.TypeTypingNotOffered:    http.StatusForbidden,
	realtime.TypeLifecycleDenied:     http.StatusForbidden,
	realtime.TypeLifecycleWrongState: http.StatusConflict,
	realtime.TypeRetentionActive:     http.StatusConflict,
	realtime.TypeSignalsNotKept:      http.StatusUnprocessableEntity,
	realtime.TypeSessionNotice:       http.StatusConflict,
	realtime.TypeSessionMessage:      http.StatusConflict,
	realtime.TypeActionNotOffered:    http.StatusForbidden,
	realtime.TypeActionDenied:        http.StatusForbidden,
	realtime.TypeActionTaken:         http.StatusConflict,
	realtime.TypeActionPending:       http.StatusConflict,
	realtime.TypeActionNotPending:    http.StatusConflict,
	realtime.TypePromoteDenied:       http.StatusForbidden,
	realtime.TypePromoteInvalid:      http.StatusUnprocessableEntity,
	realtime.TypePromoteConflict:     http.StatusConflict,
	realtime.TypeInvalid:             http.StatusBadRequest,
	realtime.TypeTooManyProps:        http.StatusBadRequest,
	realtime.TypeReasonRequired:      http.StatusBadRequest,
	realtime.TypeTooLarge:            http.StatusRequestEntityTooLarge,
	// One body for a ref to a hidden entity and to a missing one, so a post
	// cannot probe which ids exist.
	realtime.TypeInvalidRef:  http.StatusUnprocessableEntity,
	realtime.TypeRateLimited: http.StatusTooManyRequests,
}

// writeChannelError answers a failed channel operation.
//
//   - Realtime off, and an anchor the caller may not read or that does not
//     exist, answer the entity GET's 404, byte for byte apart from
//     `instance`. A channel route therefore tells a hidden entity from a
//     missing one no better than the entity route does.
//   - A refusal answers its status from [channelErrorStatus], its type and
//     title. A 403 names the rule in `detail`: realtime.yaml is
//     configuration, which is not secret. A 429 carries Retry-After in
//     whole seconds, rounded up.
//   - Anything else is a 500 with the cause in the server log only.
func writeChannelError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, realtime.ErrRealtimeOff) || errors.Is(err, realtime.ErrAnchorNotFound) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}
	if errors.Is(err, realtime.ErrNoPrincipal) {
		writeV1Error(w, r, http.StatusForbidden, realtime.TypeNoPrincipal,
			"the request carries no identity the ACL can evaluate", "")
		return
	}
	d, ok := errors.AsType[*realtime.DenialError](err)
	if !ok {
		writeInternalError(w, r, "channel_failed", "Channel request failed", err)
		return
	}
	status, known := channelErrorStatus[d.Type]
	if !known {
		slog.WarnContext(r.Context(), "dataentry: channel refusal type missing from channelErrorStatus",
			"type", d.Type, "status", d.Status)
		status = d.Status
	}
	if status == http.StatusTooManyRequests {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(d.RetryAfter.Seconds()))))
	}
	writeV1Error(w, r, status, d.Type, d.Title, d.Rule)
}
