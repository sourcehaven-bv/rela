package pgstore

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// catchUpOverlap is how far BELOW the highest seen seq the watermark is held,
// so a transaction that grabbed a lower seq but committed late is still picked
// up on a later catch-up. It trades a little redundant re-emission (harmless —
// consumers re-snapshot by id) for not missing a late-committing write.
//
// The assumption this overlap encodes: NO single write transaction holds a seq
// for longer than `catchUpOverlap` later writes take to commit. rela's writes
// are mostly short, but a few are NOT single-row: a cascade delete writes one
// tombstone per cascaded relation, and a rename re-keys + tombstones the entity
// and every incident relation — each consuming a fresh `nextval('rela_seq')`, so
// one such transaction burns a CONTIGUOUS BLOCK of seqs that all commit at once.
// That is still bounded by an entity's relation count (tens, not thousands), so
// 100 stays generous. The assumption breaks only if someone batches many writes
// in ONE long transaction (e.g. a bulk import) — then a low seq could commit
// after >100 higher seqs and be skipped. That is the precise trigger for the
// exact (xid8 + pg_snapshot_xmin) upgrade documented in the package; it is not
// built because rela has no such path today.
const catchUpOverlap = 100

// defaultCatchUpInterval is the production safety-net poll period.
const defaultCatchUpInterval = 30 * time.Second

// catchUpInterval is the safety-net poll: even if a NOTIFY is ever missed, the
// listener self-heals within this interval by re-running the catch-up query.
// An atomic so tests can shorten it (SetCatchUpIntervalForTest) without racing
// the listener goroutines that read it. Holds a time.Duration as an int64.
// A non-positive value disables the poll entirely (the listener blocks on the
// live notification alone) — used only by tests that isolate the live feed.
var catchUpInterval atomic.Int64

func init() { catchUpInterval.Store(int64(defaultCatchUpInterval)) }

// listener runs the cross-process change feed for one Store: it holds a
// dedicated PostgreSQL connection, LISTENs on the shared feed channel, turns
// notifications (and a seq-watermark catch-up) into store.Events, and emits
// them to the store's in-process subscribers.
//
// Because the channel is shared by every schema on the database, the listener
// filters on receipt: it drops a notification whose schema is not its own, and
// one whose origin is its own store (already emitted in-process). The
// seq-watermark catch-up is NOT shared — `rela_seq` is per-schema and the
// catch-up query is unqualified SQL — so it runs against this store's pool.
//
// It owns its own connection (separate from the store's query pool) so a slow
// LISTEN never starves query traffic. Lifecycle is owned by the Store:
// startListener spawns it, Store.Close stops it.
type listener struct {
	store    *Store
	dsn      string
	schema   string
	originID string

	cancel context.CancelFunc
	done   chan struct{}
}

// startListener builds and starts a listener for s against dsn. It resolves the
// store's schema (stamped into payloads and matched on receipt), then runs the
// loop in a goroutine. A failure to establish the initial connection is returned so the
// caller can degrade with a warning (the store stays usable; cross-process
// events are simply unavailable).
func startListener(ctx context.Context, s *Store, dsn string) (*listener, error) {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, err
	}
	// Resolve on the store's own pool, not this listener connection: the
	// producer writes through the pool, so the schema stamped into payloads
	// must be the pool's. They are the same DSN today, but tying it to the
	// writer is what stays correct if a pool ever repoints its search_path.
	schema, err := resolveSchema(ctx, s.db)
	if err != nil {
		_ = conn.Close(ctx)
		return nil, err
	}
	// Enabling the producer (see Store.notify, which no-ops on an empty schema).
	s.schema = schema

	lctx, cancel := context.WithCancel(context.Background())
	l := &listener{
		store:    s,
		dsn:      dsn,
		schema:   schema,
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
			// A non-positive interval disables the safety-net poll: wait on ctx
			// alone so only a live notification (or shutdown) wakes the loop. Used
			// by tests that must prove the live feed in isolation; production always
			// holds a positive interval.
			interval := time.Duration(catchUpInterval.Load())
			waitCtx, cancel := ctx, func() {}
			if interval > 0 {
				waitCtx, cancel = context.WithTimeout(ctx, interval)
			}
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
			// A live notification we couldn't use (unparseable / possibly
			// truncated past pg_notify's 8000-byte cap) triggers an IMMEDIATE
			// catch-up rather than waiting up to catchUpInterval for the ticker.
			if needCatchUp := l.handleNotification(n); needCatchUp {
				watermark = l.catchUp(ctx, watermark)
			}
		}
	}
}

// listen issues LISTEN on the shared feed channel. The name is a compile-time
// constant; it is quoted only because LISTEN does not accept bound parameters.
func (l *listener) listen(ctx context.Context, conn *pgx.Conn) error {
	_, err := conn.Exec(ctx, "LISTEN "+pgQuoteIdentifier(feedChannel))
	return err
}

// handleNotification turns one NOTIFY into a store.Event, skipping notifications
// that are not ours to deliver. It returns true when the caller should run an
// immediate catch-up: an unparseable payload (e.g. one truncated past
// pg_notify's 8000-byte limit) is never trusted, so we reconcile from real rows
// right away rather than waiting for the safety ticker.
//
// Two filters, in this order:
//
//   - **Schema.** Every schema on the database shares one channel, so a write
//     to another schema arrives here too. It describes rows this store cannot
//     see; emitting it would fabricate an event for an entity that does not
//     exist in this store's schema. Checked BEFORE origin because a foreign
//     schema's write is irrelevant regardless of who wrote it.
//   - **Origin.** Our own write, already emitted in-process by the writer.
//
// Neither triggers a catch-up: both are expected, correctly-formed traffic, not
// a signal that anything was missed.
func (l *listener) handleNotification(n *pgconn.Notification) (needCatchUp bool) {
	fe, ok := parseFeedPayload(n.Payload)
	if !ok {
		slog.Debug("pgstore listener: unparseable notification payload; catching up", "channel", n.Channel)
		return true
	}
	if fe.schema != l.schema {
		return false // another schema sharing this channel — not our rows
	}
	if fe.origin == l.originID {
		return false // our own write — already emitted in-process
	}
	l.store.emit(fe.ev)
	return false
}

// primeWatermark returns the current high-water seq (held an overlap below the
// max) WITHOUT emitting anything. Called once at listener start so the feed
// reports only changes that happen AFTER startup — a process learns about
// pre-existing data through its normal initial load, not a watcher replay.
// Returns 0 on error (so the first catch-up would replay from the start, which
// is safe if rare).
func (l *listener) primeWatermark(ctx context.Context) int64 {
	var maxSeq *int64
	// MUST cover exactly the same tables as catchUp (entities + relations +
	// deletions). If it included attachments — which consume rela_seq but emit
	// no events — the watermark could prime ABOVE the highest event-bearing seq
	// and silently eat the catchUpOverlap budget, skipping a late-committing
	// row. Keep these two queries' table sets identical.
	const q = `SELECT max(seq) FROM (
		SELECT seq FROM entities
		UNION ALL SELECT seq FROM relations
		UNION ALL SELECT seq FROM deletions) t`
	if err := l.store.db.QueryRow(ctx, q).Scan(&maxSeq); err != nil || maxSeq == nil {
		return 0
	}
	return max(*maxSeq-catchUpOverlap, 0)
}

// catchUp emits store.Events for every row with seq > watermark across the
// live tables AND the deletions tombstones, in seq order, and returns the new
// watermark held an overlap below the highest seq seen. Idempotent: re-emitting
// an already-seen change is harmless because consumers re-snapshot by id (and a
// re-emitted delete for an already-absent id is a no-op). Errors are logged and
// the watermark is left unchanged (the next catch-up retries).
//
// Including the deletions table is what makes the durable catch-up path
// delete-aware: before tombstones, a delete that missed its live NOTIFY was
// lost forever, because a scan of live rows can't see a removed row.
func (l *listener) catchUp(ctx context.Context, watermark int64) int64 {
	// Table set MUST match primeWatermark (entities + relations + deletions).
	// `typ` carries the entity type so events are structurally identical to the
	// NOTIFY path; `deleted` distinguishes a tombstone from a live row.
	// The a slot carries the state reference in its codec serialization
	// (TKT-DOFYR1): live rows join (id, pointer) / (from_id, from_pointer)
	// the same way tombstones' id_a was written, so catchUpEvent parses
	// one shape. The SQL concatenation mirrors entity.FormatStateRef
	// exactly (bare id when the pointer is ''); both components are
	// already canonical, so no other form can be produced.
	const q = `
		SELECT kind, a, b, c, typ, deleted, seq FROM (
			-- TRAP (TKT-WAV8XP PR-C, RULING 4): the pointer tests below have
			-- INVERTED POLARITY versus every other pointer literal in this
			-- package. They are NOT a read scope and NOT a default-world
			-- filter: this scan is deliberately UNFILTERED (note the absence
			-- of any WHERE pointer clause), and the CASE expressions are
			-- pointer-aware SERIALIZATION — they rebuild the state ref
			-- (bare id when the pointer is empty, id@pointer otherwise) so
			-- catch-up rows match the shape tombstones were written in.
			--
			-- Mislabeling these "default world" and adding a world arm
			-- BREAKS THE CHANGE FEED: the catch-up would stop reporting
			-- non-default states, so a cross-process writer's state edits
			-- would silently never reach other processes' subscribers, and
			-- the watermark would advance past them so the miss is
			-- unrecoverable rather than merely delayed.
			SELECT 'e' AS kind,
			       id || CASE WHEN pointer = '' THEN '' ELSE '@' || pointer END AS a,
			       '' AS b, '' AS c, type AS typ, false AS deleted, seq FROM entities
			UNION ALL
			SELECT 'r',
			       from_id || CASE WHEN from_pointer = '' THEN '' ELSE '@' || from_pointer END,
			       rel_type, to_id, '', false, seq FROM relations
			UNION ALL
			SELECT kind, id_a, id_b, id_c, typ, true, seq FROM deletions
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
		var kind, a, b, c, typ string
		var deleted bool
		var seq int64
		if err := rows.Scan(&kind, &a, &b, &c, &typ, &deleted, &seq); err != nil {
			slog.Debug("pgstore listener: catch-up scan failed", "error", err)
			return watermark
		}
		if seq > highest {
			highest = seq
		}
		l.store.emit(catchUpEvent(kind, a, b, c, typ, deleted))
	}
	if err := rows.Err(); err != nil {
		slog.Debug("pgstore listener: catch-up rows error", "error", err)
		return watermark
	}

	// Hold the watermark an overlap below the highest seen so a late-committing
	// lower seq is re-scanned next time (it'll be re-emitted, which is harmless).
	return max(highest-catchUpOverlap, watermark)
}

// catchUpEvent builds a store.Event from a catch-up row. For a live row,
// catch-up can't distinguish create vs update (the row just exists), so it
// reports an Updated "this exists, re-snapshot it" event; consumers treat any
// event as a re-snapshot trigger, so Updated is the faithful choice. A tombstone
// row (deleted) reports the corresponding Deleted event so consumers mirror the
// removal.
func catchUpEvent(kind, a, b, c, typ string, deleted bool) store.Event {
	// The a slot is a codec-serialized state reference; an unparseable
	// value degrades to a bare id with the zero pointer (a re-snapshot
	// by id is still the right consumer response).
	id, ptr, err := entity.ParseStateRef(a)
	if err != nil {
		id, ptr = a, ""
	}
	if kind == "r" {
		if deleted {
			return store.Event{Op: store.EventRelationDeleted, From: id, Pointer: ptr, RelationType: b, To: c}
		}
		return store.Event{Op: store.EventRelationUpdated, From: id, Pointer: ptr, RelationType: b, To: c}
	}
	if deleted {
		return store.Event{Op: store.EventEntityDeleted, EntityType: typ, EntityID: id, Pointer: ptr}
	}
	return store.Event{Op: store.EventEntityUpdated, EntityType: typ, EntityID: id, Pointer: ptr}
}

// dropConn closes a broken connection. Callers set their conn to nil afterward
// so the outer loop reconnects.
func dropConn(ctx context.Context, conn *pgx.Conn) {
	if conn != nil {
		_ = conn.Close(ctx)
	}
}

// reconnect dials a fresh connection, backing off between attempts until it
// succeeds or ctx is cancelled. A fixed 2s backoff (no jitter/cap) is fine for a
// single dedicated connection. Early failures log at Debug to avoid noise on a
// transient blip, but after warnAfterFailures consecutive failures it escalates
// to Warn so a *persistently* dead feed (wrong DSN after restart, DB down for
// minutes) is visible in default log configs rather than silent.
func (l *listener) reconnect(ctx context.Context) (*pgx.Conn, error) {
	const backoff = 2 * time.Second
	const warnAfterFailures = 5 // ~10s of failures before escalating to Warn
	failures := 0
	for {
		conn, err := pgx.Connect(ctx, l.dsn)
		if err == nil {
			if failures >= warnAfterFailures {
				slog.Warn("pgstore listener: reconnected; cross-process change feed restored",
					"after_failures", failures)
			}
			return conn, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		failures++
		if failures == warnAfterFailures {
			slog.Warn("pgstore listener: change feed connection down; retrying",
				"consecutive_failures", failures, "error", err)
		} else {
			slog.Debug("pgstore listener: reconnect failed, retrying", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}
}

// pgQuoteIdentifier quotes a PostgreSQL identifier (doubling embedded quotes)
// for safe interpolation into LISTEN, which does not accept bound parameters.
// The channel is a compile-time constant, so this is defense-in-depth rather
// than a live injection vector.
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
