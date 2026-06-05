package pgstore

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// catchUpOverlap is how far BELOW the highest seen seq the watermark is held,
// so a transaction that grabbed a lower seq but committed late is still picked
// up on a later catch-up. It trades a little redundant re-emission (harmless —
// consumers re-snapshot by id) for not missing a late-committing write. Sized
// generously relative to rela's short write transactions. The exact (xid8 +
// pg_snapshot_xmin) fix is the documented upgrade if strict ordering is ever
// needed.
const catchUpOverlap = 100

// catchUpInterval is the safety-net poll: even if a NOTIFY is ever missed, the
// listener self-heals within this interval by re-running the catch-up query.
// A var (not const) so tests can shorten it via SetCatchUpIntervalForTest.
var catchUpInterval = 30 * time.Second

// listener runs the cross-process change feed for one Store: it holds a
// dedicated PostgreSQL connection, LISTENs on the store's schema-scoped channel,
// turns notifications (and a seq-watermark catch-up) into store.Events, and
// emits them to the store's in-process subscribers.
//
// It owns its own connection (separate from the store's query pool) so a slow
// LISTEN never starves query traffic. Lifecycle is owned by the Store:
// startListener spawns it, Store.Close stops it.
type listener struct {
	store    *Store
	dsn      string
	channel  string
	originID string

	cancel context.CancelFunc
	done   chan struct{}
}

// startListener builds and starts a listener for s against dsn. It resolves the
// schema-scoped channel from a throwaway connection, then runs the loop in a
// goroutine. A failure to establish the initial connection is returned so the
// caller can degrade with a warning (the store stays usable; cross-process
// events are simply unavailable).
func startListener(ctx context.Context, s *Store, dsn string) (*listener, error) {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, err
	}
	channel, err := resolveChannel(ctx, conn)
	if err != nil {
		_ = conn.Close(ctx)
		return nil, err
	}
	// Keep the store's producer channel in sync with the listener's (both must
	// match for self/remote notifications to land on the same channel).
	s.channel = channel

	lctx, cancel := context.WithCancel(context.Background())
	l := &listener{
		store:    s,
		dsn:      dsn,
		channel:  channel,
		originID: s.originID,
		cancel:   cancel,
		done:     make(chan struct{}),
	}
	// lctx is a detached lifetime context (cancelled by stop()), deliberately
	// independent of any request ctx — the listener outlives single requests.
	go l.run(lctx, conn) //nolint:contextcheck,gosec // listener uses its own lifetime ctx by design
	return l, nil
}

// stop signals the run loop to exit and waits for it to finish (closing its
// connection). Idempotent.
func (l *listener) stop() {
	if l == nil {
		return
	}
	l.cancel()
	<-l.done
}

// run is the listener loop. It owns conn and is responsible for closing it.
// It runs an initial catch-up, then alternates between waiting for a
// notification (with a periodic timeout that triggers a safety-net catch-up)
// and, on any connection error, reconnecting + re-LISTENing + catching up.
func (l *listener) run(ctx context.Context, conn *pgx.Conn) {
	defer close(l.done)

	// Prime the watermark to the current max(seq) WITHOUT emitting: everything
	// already committed when this process started is "already known" (consumers
	// re-snapshot on their own first load), not news. From here, catch-up emits
	// only writes that land after startup — i.e. genuinely missed notifications.
	watermark := l.primeWatermark(ctx)

	for {
		if conn == nil {
			c, err := l.reconnect(ctx)
			if err != nil {
				return // ctx cancelled (shutdown)
			}
			conn = c
		}

		// (Re)LISTEN, then recover the gap since the last watermark (covers
		// anything missed while we had no live subscription). This runs ONCE per
		// connection, not per notification.
		if err := l.listen(ctx, conn); err != nil {
			dropConn(ctx, conn)
			conn = nil
			continue
		}
		watermark = l.catchUp(ctx, watermark)

		// Inner loop: handle live notifications, with a periodic timeout that
		// triggers the safety-net catch-up. Stays here until the connection
		// breaks (then the outer loop reconnects) or ctx is cancelled.
		for {
			waitCtx, cancel := context.WithTimeout(ctx, catchUpInterval)
			n, err := conn.WaitForNotification(waitCtx)
			cancel()

			if ctx.Err() != nil {
				//nolint:contextcheck // ctx is cancelled (shutdown); close needs a live ctx
				_ = conn.Close(context.Background())
				return
			}
			if errors.Is(err, context.DeadlineExceeded) {
				watermark = l.catchUp(ctx, watermark) // safety-net poll
				continue
			}
			if err != nil {
				dropConn(ctx, conn) // connection problem — outer loop reconnects
				conn = nil
				break
			}
			l.handleNotification(n) // live event; does NOT trigger a catch-up
		}
	}
}

// listen issues LISTEN on the channel. The channel is a server-generated,
// identifier-shaped string (prefix + current_schema()), quoted to be safe.
func (l *listener) listen(ctx context.Context, conn *pgx.Conn) error {
	_, err := conn.Exec(ctx, "LISTEN "+pgQuoteIdentifier(l.channel))
	return err
}

// handleNotification turns one NOTIFY into a store.Event, skipping our own
// writes (already emitted in-process) and falling back to a catch-up for any
// unparseable payload (never trust the wire form).
func (l *listener) handleNotification(n *pgconn.Notification) {
	fe, ok := parseFeedPayload(n.Payload)
	if !ok {
		// Malformed/garbage payload — don't trust it; the next catch-up
		// (or the safety ticker) reconciles from real rows.
		slog.Debug("pgstore listener: unparseable notification payload", "channel", n.Channel)
		return
	}
	if fe.origin == l.originID {
		return // our own write — already emitted in-process
	}
	l.store.emit(fe.ev)
}

// primeWatermark returns the current high-water seq (held an overlap below the
// max) WITHOUT emitting anything. Called once at listener start so the feed
// reports only changes that happen AFTER startup — a process learns about
// pre-existing data through its normal initial load, not a watcher replay.
// Returns 0 on error (so the first catch-up would replay from the start, which
// is safe if rare).
func (l *listener) primeWatermark(ctx context.Context) int64 {
	var maxSeq *int64
	const q = `SELECT max(seq) FROM (
		SELECT seq FROM entities UNION ALL SELECT seq FROM relations
		UNION ALL SELECT seq FROM attachments) t`
	if err := l.store.db.QueryRow(ctx, q).Scan(&maxSeq); err != nil || maxSeq == nil {
		return 0
	}
	return max(*maxSeq-catchUpOverlap, 0)
}

// catchUp emits store.Events for every row with seq > watermark across the three
// tables, in seq order, and returns the new watermark held an overlap below the
// highest seq seen. Idempotent: re-emitting an already-seen change is harmless
// because consumers re-snapshot by id. Errors are logged and the watermark is
// left unchanged (the next catch-up retries).
func (l *listener) catchUp(ctx context.Context, watermark int64) int64 {
	const q = `
		SELECT kind, a, b, c, seq FROM (
			SELECT 'e' AS kind, id      AS a, ''       AS b, ''    AS c, seq FROM entities
			UNION ALL
			SELECT 'r',         from_id,      rel_type,       to_id,      seq FROM relations
		) t
		WHERE seq > $1
		ORDER BY seq`
	rows, err := l.store.db.Query(ctx, q, watermark)
	if err != nil {
		if ctx.Err() == nil {
			slog.Debug("pgstore listener: catch-up query failed", "error", err)
		}
		return watermark
	}
	defer rows.Close()

	highest := watermark
	for rows.Next() {
		var kind, a, b, c string
		var seq int64
		if err := rows.Scan(&kind, &a, &b, &c, &seq); err != nil {
			slog.Debug("pgstore listener: catch-up scan failed", "error", err)
			return watermark
		}
		if seq > highest {
			highest = seq
		}
		l.store.emit(catchUpEvent(kind, a, b, c))
	}
	if err := rows.Err(); err != nil {
		slog.Debug("pgstore listener: catch-up rows error", "error", err)
		return watermark
	}

	// Hold the watermark an overlap below the highest seen so a late-committing
	// lower seq is re-scanned next time (it'll be re-emitted, which is harmless).
	return max(highest-catchUpOverlap, watermark)
}

// catchUpEvent builds a store.Event from a catch-up row. Catch-up can't
// distinguish create vs update (the row just exists), so it reports an
// Updated/Created-equivalent "this exists, re-snapshot it" event; consumers
// treat any event as a re-snapshot trigger, so Updated is the faithful choice.
func catchUpEvent(kind, a, b, c string) store.Event {
	if kind == "r" {
		return store.Event{Op: store.EventRelationUpdated, From: a, RelationType: b, To: c}
	}
	return store.Event{Op: store.EventEntityUpdated, EntityID: a}
}

// dropConn closes a broken connection. Callers set their conn to nil afterward
// so the outer loop reconnects.
func dropConn(ctx context.Context, conn *pgx.Conn) {
	if conn != nil {
		_ = conn.Close(ctx)
	}
}

// reconnect dials a fresh connection, backing off between attempts until it
// succeeds or ctx is cancelled.
func (l *listener) reconnect(ctx context.Context) (*pgx.Conn, error) {
	const backoff = 2 * time.Second
	for {
		conn, err := pgx.Connect(ctx, l.dsn)
		if err == nil {
			return conn, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		slog.Debug("pgstore listener: reconnect failed, retrying", "error", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}
}

// pgQuoteIdentifier quotes a PostgreSQL identifier (doubling embedded quotes)
// for safe interpolation into LISTEN, which does not accept bound parameters.
// The channel is server-derived (prefix + current_schema()), so this is
// defense-in-depth rather than a live injection vector.
func pgQuoteIdentifier(ident string) string {
	out := make([]byte, 0, len(ident)+2)
	out = append(out, '"')
	for i := range len(ident) {
		if ident[i] == '"' {
			out = append(out, '"')
		}
		out = append(out, ident[i])
	}
	out = append(out, '"')
	return string(out)
}
