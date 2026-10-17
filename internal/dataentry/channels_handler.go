package dataentry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

const (
	channelsSegment = "_channels"
	messagesSegment = "messages"
	// channelBodyLimit caps a channel request body: the largest message
	// realtime.yaml may allow, plus room for the JSON envelope. The service
	// applies the configured, possibly smaller, limit to the properties.
	channelBodyLimit = realtimelimits.MaxMessageBytes + 1<<10
)

// ChannelService is the channel capability the channel routes use. The
// wiring site passes the project's *realtime.Service, which authorizes
// every operation itself.
type ChannelService interface {
	ListAnchor(ctx context.Context, entityType, anchor string, state channels.State) (realtime.AnchorChannels, error)
	Head(ctx context.Context, addr realtime.RawAddress) (realtime.HeadView, error)
	Messages(ctx context.Context, addr realtime.RawAddress, page channels.Page) (realtime.MessagePage, error)
	Post(ctx context.Context, addr realtime.RawAddress, in realtime.PostInput) (realtime.PostResult, error)
	Create(ctx context.Context, entityType, anchor, typ string, in realtime.CreateInput) (realtime.ChannelView, error)
	Edit(ctx context.Context, addr realtime.RawAddress, seq int64, props map[string]any) (realtime.MessageView, error)
	Delete(ctx context.Context, addr realtime.RawAddress, seq int64, reason string) (realtime.MessageView, error)
}

// realtimeSwitch reports whether realtime is on now. *realtime.Service
// satisfies it.
type realtimeSwitch interface {
	Enabled() bool
}

// channelsHandler owns the channel routes (TKT-ZU7P13). {ref} is ID or
// ID@face; {type} is a channel type of realtime.yaml.
//
//	GET    /api/v1/{plural}/{ref}/_channels                    channel list
//	POST   /api/v1/{plural}/{ref}/_channels/{type}             create (cardinality many)
//	GET    /api/v1/{plural}/{ref}/_channels/{type}[/{name}]     channel head
//	GET    …/_channels/{type}[/{name}]/messages                one page of messages
//	POST   …/_channels/{type}[/{name}]/messages                post (201, or 202 ephemeral)
//	PATCH  …/_channels/{type}[/{name}]/messages/{seq}          edit own message
//	DELETE …/_channels/{type}[/{name}]/messages/{seq}          delete or remove
//
// Parsing is structural: after the type, `messages` is the subresource
// wherever it stands, and can never be a channel name, because names are minted as 13 base36
// digits. The service decides by cardinality whether a name belongs.
//
// A nil *channelsHandler is "channels not wired": every route answers the
// entity GET's uniform 404, as with realtime off. [SetChannels] installs it.
type channelsHandler struct {
	svc ChannelService
	// on answers whether realtime is on; see [channelsHandler.serve].
	on realtimeSwitch
	// changes subscribes the live worker to channel changes.
	changes func() channelChanges
}

// SetChannels installs the channel service and describes its routes in the
// OpenAPI document. Call it before [App.StartWatching] and [App.NewRouter],
// like [SetLeases]. A package function because App is at its plimsoll
// method cap.
//
// Nil: rejected, as for [SetLeases].
func SetChannels(a *App, svc *realtime.Service) error {
	if svc == nil {
		return errors.New("dataentry: SetChannels requires a channel service")
	}
	a.channels = &channelsHandler{
		svc:     svc,
		on:      svc,
		changes: func() channelChanges { return svc.Subscribe() },
	}
	if st := a.State(); st != nil && st.OpenAPIGen != nil {
		st.OpenAPIGen.SetRealtime(svc.Config)
	}
	return nil
}

// wakeSource returns the channel change subscription the live worker
// listens to, or nil when channels are not wired.
func (h *channelsHandler) wakeSource() func() channelChanges {
	if h == nil {
		return nil
	}
	return h.changes
}

// channelRoute is a parsed channel route below /_channels.
type channelRoute struct {
	addr realtime.RawAddress
	// msgs is set for …/messages; seq additionally for …/messages/{seq}.
	msgs bool
	seq  int64
}

// serve answers every route under /api/v1/{plural}/{ref}/_channels. rest
// holds the segments after _channels. With realtime off it answers the
// entity GET's 404 before it reads the route, query or body, so no request
// shape tells "off" apart from a hidden or missing entity.
func (h *channelsHandler) serve(w http.ResponseWriter, r *http.Request, typeName, ref string, rest []string) {
	if h == nil || !h.on.Enabled() {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}
	if len(rest) == 0 {
		if allowMethod(w, r, http.MethodGet) {
			h.list(w, r, typeName, ref)
		}
		return
	}
	rt, ok := parseChannelRoute(typeName, ref, rest)
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
		return
	}
	switch {
	case rt.seq != 0:
		switch r.Method {
		case http.MethodPatch:
			h.edit(w, r, rt)
		case http.MethodDelete:
			h.remove(w, r, rt)
		default:
			methodNotAllowed(w, r, "PATCH, DELETE")
		}
	case rt.msgs:
		switch r.Method {
		case http.MethodGet:
			h.messages(w, r, rt)
		case http.MethodPost:
			h.post(w, r, rt)
		default:
			methodNotAllowed(w, r, "GET, POST")
		}
	case rt.addr.Name == "":
		switch r.Method {
		case http.MethodGet:
			h.head(w, r, rt)
		case http.MethodPost:
			h.create(w, r, rt)
		default:
			methodNotAllowed(w, r, "GET, POST")
		}
	default:
		if allowMethod(w, r, http.MethodGet) {
			h.head(w, r, rt)
		}
	}
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request, allow string) {
	w.Header().Set("Allow", allow)
	writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
}

// parseChannelRoute parses {type}[/{name}][/messages[/{seq}]]. It reports
// false for any other shape.
func parseChannelRoute(typeName, ref string, rest []string) (channelRoute, bool) {
	if slices.Contains(rest, "") {
		return channelRoute{}, false
	}
	rt := channelRoute{addr: realtime.RawAddress{EntityType: typeName, Anchor: ref, Type: rest[0]}}
	tail := rest[1:]
	if len(tail) > 0 && tail[0] != messagesSegment {
		rt.addr.Name, tail = tail[0], tail[1:]
	}
	switch {
	case len(tail) == 0:
	case len(tail) == 1 && tail[0] == messagesSegment:
		rt.msgs = true
	case len(tail) == 2 && tail[0] == messagesSegment:
		seq, err := strconv.ParseInt(tail[1], 10, 64)
		if err != nil || seq <= 0 {
			return channelRoute{}, false
		}
		rt.msgs, rt.seq = true, seq
	default:
		return channelRoute{}, false
	}
	return rt, true
}

func (h *channelsHandler) list(w http.ResponseWriter, r *http.Request, typeName, ref string) {
	state := channels.State(r.URL.Query().Get("state"))
	if state != "" && !state.Valid() {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid state",
			"state must be open, closed or archived")
		return
	}
	got, err := h.svc.ListAnchor(r.Context(), typeName, ref, state)
	if err != nil {
		writeChannelError(w, r, err)
		return
	}
	if addr, perr := entity.ParseAddress(got.Anchor); perr == nil {
		recordTopic(r.Context(), anchorTopic(addr.ID()))
	}
	writeV1JSON(w, http.StatusOK, wireChannelList(got))
}

func (h *channelsHandler) head(w http.ResponseWriter, r *http.Request, rt channelRoute) {
	got, err := h.svc.Head(r.Context(), rt.addr)
	if err != nil {
		writeChannelError(w, r, err)
		return
	}
	recordTopic(r.Context(), channelTopic(got.Channel.Anchor, got.Channel.Type, got.Channel.Name))
	writeV1JSON(w, http.StatusOK, wireHead(got))
}

func (h *channelsHandler) messages(w http.ResponseWriter, r *http.Request, rt channelRoute) {
	page, ok := parseMessagePage(w, r)
	if !ok {
		return
	}
	got, err := h.svc.Messages(r.Context(), rt.addr, page)
	if err != nil {
		writeChannelError(w, r, err)
		return
	}
	writeV1JSON(w, http.StatusOK, wirePage(got, page))
}

// parseMessagePage reads after, before, after_rev and limit. A value that
// is not a whole number is a 400; the service checks the combination.
func parseMessagePage(w http.ResponseWriter, r *http.Request) (channels.Page, bool) {
	q := r.URL.Query()
	var page channels.Page
	for _, f := range []struct {
		name string
		dst  *int64
	}{{"after", &page.AfterSeq}, {"before", &page.BeforeSeq}, {"after_rev", &page.AfterRev}} {
		if v, ok, bad := queryInt(q, f.name); bad {
			writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid query", f.name+" must be a whole number")
			return page, false
		} else if ok {
			*f.dst = v
		}
	}
	limit, ok, bad := queryInt(q, "limit")
	if bad || (ok && limit <= 0) {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid query", "limit must be a positive number")
		return page, false
	}
	page.Limit = int(min(limit, int64(realtimelimits.MaxPageLimit)))
	return page, true
}

// queryInt reads the whole number q names. ok is false when it is absent;
// bad is set when it is present and not a whole number.
func queryInt(q url.Values, name string) (v int64, ok, bad bool) {
	s := q.Get(name)
	if s == "" {
		return 0, false, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false, true
	}
	return v, true, false
}

func (h *channelsHandler) post(w http.ResponseWriter, r *http.Request, rt channelRoute) {
	var req v1.PostMessageRequest
	if !decodeChannelBody(w, r, &req, false) {
		return
	}
	in := realtime.PostInput{Type: req.Type, Props: req.Props, ReplyTo: req.ReplyTo}
	got, err := h.svc.Post(r.Context(), rt.addr, in)
	if err != nil {
		writeChannelError(w, r, err)
		return
	}
	if got.Ephemeral {
		writeV1JSON(w, http.StatusAccepted, v1.EphemeralAccepted{ID: got.ID, ExpiresAt: channelTime(got.ExpiresAt)})
		return
	}
	writeV1JSON(w, http.StatusCreated, wireMessage(*got.Message))
}

func (h *channelsHandler) create(w http.ResponseWriter, r *http.Request, rt channelRoute) {
	var req v1.CreateChannelRequest
	if !decodeChannelBody(w, r, &req, true) {
		return
	}
	got, err := h.svc.Create(r.Context(), rt.addr.EntityType, rt.addr.Anchor, rt.addr.Type,
		realtime.CreateInput{Title: req.Title})
	if err != nil {
		writeChannelError(w, r, err)
		return
	}
	writeV1JSON(w, http.StatusCreated, wireChannel(got))
}

func (h *channelsHandler) edit(w http.ResponseWriter, r *http.Request, rt channelRoute) {
	var req v1.EditMessageRequest
	if !decodeChannelBody(w, r, &req, false) {
		return
	}
	got, err := h.svc.Edit(r.Context(), rt.addr, rt.seq, req.Props)
	if err != nil {
		writeChannelError(w, r, err)
		return
	}
	writeV1JSON(w, http.StatusOK, wireMessage(got))
}

// remove serves DELETE. The reason comes from the JSON body only, never
// from the URL, which ends up in access logs.
func (h *channelsHandler) remove(w http.ResponseWriter, r *http.Request, rt channelRoute) {
	var req v1.RemoveMessageRequest
	if !decodeChannelBody(w, r, &req, true) {
		return
	}
	got, err := h.svc.Delete(r.Context(), rt.addr, rt.seq, req.Reason)
	if err != nil {
		writeChannelError(w, r, err)
		return
	}
	writeV1JSON(w, http.StatusOK, wireMessage(got))
}

// decodeChannelBody reads a JSON body of at most [channelBodyLimit] bytes
// into dst. An empty body is accepted only when emptyOK. The decode is
// strict: an unknown field, or data after the JSON value, is a 400.
func decodeChannelBody(w http.ResponseWriter, r *http.Request, dst any, emptyOK bool) bool {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, channelBodyLimit))
	if err != nil {
		if _, tooBig := errors.AsType[*http.MaxBytesError](err); tooBig {
			writeV1Error(w, r, http.StatusRequestEntityTooLarge, realtime.TypeTooLarge, "Request body too large", "")
			return false
		}
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Could not read the request body", "")
		return false
	}
	if len(bytes.TrimSpace(data)) == 0 {
		if emptyOK {
			return true
		}
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid JSON", "the request needs a JSON body")
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	if err := dec.Decode(dst); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid JSON", err.Error())
		return false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid JSON",
			"unexpected data after the JSON object")
		return false
	}
	return true
}

// splitChannelRoute reports whether parts, the path segments after
// /api/v1/, address the channel routes, and returns the segments after
// _channels.
func splitChannelRoute(parts []string) (rest []string, ok bool) {
	if len(parts) < 3 || parts[2] != channelsSegment {
		return nil, false
	}
	return parts[3:], true
}
