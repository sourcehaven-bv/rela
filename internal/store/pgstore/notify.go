package pgstore

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Notify and Listen carry opaque payloads between the processes of one
// schema on a NOTIFY channel of their own, apart from the entity change feed
// on feedChannel. They ride the listener's dedicated connection, so a
// second channel costs no connection. What the payloads mean is the
// caller's business: pgstore only adds the schema prefix and filters on
// it, as it does for the entity feed.
//
// Payloads are visible to every session that LISTENs on the channel, in
// every schema of the database: callers must not put content in them that
// another tenant on the database must not see.

// ChannelHints is the NOTIFY channel realtime channel hints use.
const ChannelHints = "rela_channels"

// notifyChannels are the channels Notify and Listen accept. The listener
// LISTENs on each, next to feedChannel.
var notifyChannels = []string{ChannelHints}

// maxNotifyBytes is PostgreSQL's limit on a NOTIFY payload, less one.
const maxNotifyBytes = 7999

// ErrListenerUnavailable is returned by [Notify] and [Listen] when the store
// has no running listener: it was built with [New], or its listener could
// not connect at [Open].
var ErrListenerUnavailable = errors.New("pgstore: change listener unavailable")

// listenerOf returns the running listener of st.
func listenerOf(st store.Store) (*listener, error) {
	s, ok := store.Innermost(st).(*Store)
	if !ok {
		return nil, errors.New("pgstore: not a pgstore")
	}
	s.changeFeedMu.Lock()
	l := s.listener
	s.changeFeedMu.Unlock()
	if l == nil {
		return nil, ErrListenerUnavailable
	}
	return l, nil
}

func checkNotifyChannel(channel string) error {
	if slices.Contains(notifyChannels, channel) {
		return nil
	}
	return fmt.Errorf("pgstore: %q is not a notify channel", channel)
}

// Notify sends body to the listeners of channel on st's schema, in other
// processes and in this one.
func Notify(ctx context.Context, st store.Store, channel string, body []byte) error {
	if err := checkNotifyChannel(channel); err != nil {
		return err
	}
	l, err := listenerOf(st)
	if err != nil {
		return err
	}
	payload := l.schema + payloadSep + string(body)
	if len(payload) > maxNotifyBytes {
		return fmt.Errorf("pgstore: notify payload of %d bytes is over the limit", len(payload))
	}
	if _, err := l.store.db.Exec(ctx, `SELECT pg_notify($1, $2)`, channel, payload); err != nil {
		return fmt.Errorf("pgstore: notify: %w", err)
	}
	return nil
}

// Listen calls deliver with the body of each notification on channel from
// st's schema. It calls gap once the registration is live, so that every
// notification sent from then on is delivered: at once when LISTEN already
// runs, else when it starts. It calls gap again after the listener
// reconnected, once LISTEN runs again: notifications sent while it was away
// are lost. deliver and gap must not block; they run on the listener's
// goroutine, except the first gap, which may run on the caller's. cancel
// stops the delivery.
func Listen(st store.Store, channel string, deliver func([]byte), gap func()) (cancel func(), err error) {
	if chErr := checkNotifyChannel(channel); chErr != nil {
		return nil, chErr
	}
	if deliver == nil || gap == nil {
		return nil, errors.New("pgstore: listen needs deliver and gap")
	}
	l, err := listenerOf(st)
	if err != nil {
		return nil, err
	}
	cancel, live := l.handlers.add(channel, deliver, gap)
	if live {
		gap()
	}
	return cancel, nil
}

// notifyHandlers are the Listen registrations of one listener.
type notifyHandlers struct {
	mu   sync.Mutex
	next int
	byID map[int]notifyHandler
	// listening is whether LISTEN runs on the listener's connection.
	listening bool
}

// setListening records whether LISTEN runs, and on its start tells every
// handler, since payloads sent while it did not run are lost.
func (h *notifyHandlers) setListening(on bool) {
	h.mu.Lock()
	h.listening = on
	h.mu.Unlock()
	if on {
		h.gapAll()
	}
}

type notifyHandler struct {
	channel string
	deliver func([]byte)
	gap     func()
}

// add registers a handler and reports whether LISTEN already runs, so the
// caller owes it its first gap.
func (h *notifyHandlers) add(channel string, deliver func([]byte), gap func()) (cancel func(), live bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.byID == nil {
		h.byID = map[int]notifyHandler{}
	}
	id := h.next
	h.next++
	h.byID[id] = notifyHandler{channel: channel, deliver: deliver, gap: gap}
	var once sync.Once
	return func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.byID, id)
			h.mu.Unlock()
		})
	}, h.listening
}

// matching returns the handlers of channel, or of every channel when
// channel is empty.
func (h *notifyHandlers) matching(channel string) []notifyHandler {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]notifyHandler, 0, len(h.byID))
	for _, x := range h.byID {
		if channel == "" || x.channel == channel {
			out = append(out, x)
		}
	}
	return out
}

// gapAll tells every handler that notifications may have been lost.
func (h *notifyHandlers) gapAll() {
	for _, x := range h.matching("") {
		x.gap()
	}
}

// isNotifyChannel reports whether n arrived on a Notify channel rather than
// the entity feed.
func isNotifyChannel(name string) bool { return checkNotifyChannel(name) == nil }

// handleNotify routes a notification on a Notify channel. One from another
// schema is dropped. So is one without a schema prefix: [Notify] never
// sends that, and any session on the database can, so treating it as a
// gap would let it make every schema resync. A real loss is reported by
// gapAll when LISTEN runs again.
func (l *listener) handleNotify(n *pgconn.Notification) {
	schema, body, ok := strings.Cut(n.Payload, payloadSep)
	if !ok {
		slog.Debug("pgstore listener: dropped a notification without a schema prefix", "channel", n.Channel)
		return
	}
	if schema != l.schema {
		return
	}
	for _, h := range l.handlers.matching(n.Channel) {
		h.deliver([]byte(body))
	}
}
