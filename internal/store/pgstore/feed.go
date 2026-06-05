package pgstore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The cross-process change feed (TKT-WZYWM9). Writes emit a PostgreSQL
// NOTIFY carrying the changed identity; a listener in another process turns
// those notifications back into store.Events. See listener.go for the consumer
// and the seq-watermark catch-up that makes delivery recoverable.
//
// # Channel scoping
//
// LISTEN/NOTIFY channels are DATABASE-global, not schema-scoped. Since the
// conformance harness runs many isolated schemas on one database, the channel
// name is schema-qualified (rela_changed_<schema>) so two schemas never
// cross-talk. All processes of one deployment share a schema, so they share a
// channel and see each other's writes.
//
// # Self-echo
//
// A process that writes also receives its own NOTIFY back. The payload carries
// the writing store's random originID; the listener skips notifications whose
// origin matches its own store (those writes were already emitted in-process).

// feedChannelPrefix is the constant part of the NOTIFY channel; the active
// schema is appended (see resolveChannel).
const feedChannelPrefix = "rela_changed_"

// payloadSep separates payload fields. It is '\x1f' (ASCII Unit Separator),
// which storeutil.ValidateID rejects in entity IDs and relation types (control
// characters are forbidden), so it can never collide with an ID/type/property
// value in the payload.
const payloadSep = "\x1f"

// newOriginID returns a random per-store identifier used to recognize (and
// skip) a store's own NOTIFY echoes.
func newOriginID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure is fatal-grade; fall back to a process-unique-ish
		// constant so self-echo filtering still mostly works. Practically never hit.
		return "origin-fallback"
	}
	return hex.EncodeToString(b[:])
}

// resolveChannel returns the schema-scoped NOTIFY channel for this store's
// connection. The schema is the first entry of the active search_path
// (current_schema()), which is where the store's tables live.
func resolveChannel(ctx context.Context, db DBTX) (string, error) {
	var schema string
	if err := db.QueryRow(ctx, `SELECT current_schema()`).Scan(&schema); err != nil {
		return "", fmt.Errorf("pgstore: resolve schema for change channel: %w", err)
	}
	if schema == "" {
		schema = "public"
	}
	return feedChannelPrefix + schema, nil
}

// feedEvent is the wire form of a change: the writing store's origin plus the
// fields needed to reconstruct a store.Event. Encoded as 7 SEP-joined fields:
//
//	origin SEP kind SEP op SEP entityType SEP a SEP b SEP c
//
// where (a,b,c) are (id,"","") for an entity or (from,relType,to) for a
// relation, and entityType is the entity's type ("" for relations).
//
// The separator (\x1f, a control char) cannot appear in an id or relation type
// because storeutil.ValidateID rejects control characters. entityType is NOT
// validated for control chars on write; a type containing \x1f would inject an
// extra field, fail the 7-field parse, and degrade that one notification to a
// catch-up (handleNotification returns needCatchUp) — never a corruption.
type feedEvent struct {
	origin string
	ev     store.Event
}

// notifyPayload encodes a store.Event (with this store's origin) for pg_notify.
// Entities use the EntityType/EntityID fields; relations use RelationType/From/To.
func notifyPayload(origin string, ev store.Event) string {
	kind, op := feedKindOp(ev.Op)
	var a, b, c string
	switch ev.Op {
	case store.EventEntityCreated, store.EventEntityUpdated, store.EventEntityDeleted:
		a = ev.EntityID
	case store.EventRelationCreated, store.EventRelationUpdated, store.EventRelationDeleted:
		a, b, c = ev.From, ev.RelationType, ev.To
	}
	// EntityType is carried for entity events so the listener can rebuild a
	// faithful event; relations don't carry a type beyond RelationType.
	return strings.Join([]string{origin, kind, op, ev.EntityType, a, b, c}, payloadSep)
}

// parseFeedPayload reverses notifyPayload. Returns ok=false for any malformed
// payload (the listener then falls back to the catch-up query rather than
// trusting a bad NOTIFY).
// payloadFields is the number of SEP-joined fields in a notify payload:
// origin, kind, op, entityType, a, b, c.
const payloadFields = 7

func parseFeedPayload(payload string) (feedEvent, bool) {
	parts := strings.Split(payload, payloadSep)
	if len(parts) != payloadFields {
		return feedEvent{}, false
	}
	origin, kind, op, entType, a, b, c := parts[0], parts[1], parts[2], parts[3], parts[4], parts[5], parts[6]
	feOp, ok := feedOp(kind, op)
	if !ok {
		return feedEvent{}, false
	}
	ev := store.Event{Op: feOp}
	switch kind {
	case "e":
		ev.EntityType, ev.EntityID = entType, a
	case "r":
		ev.From, ev.RelationType, ev.To = a, b, c
	}
	return feedEvent{origin: origin, ev: ev}, true
}

// feedKindOp maps a store.EventOp to the compact (kind, op) payload codes.
func feedKindOp(op store.EventOp) (kind, code string) {
	switch op {
	case store.EventEntityCreated:
		return "e", "c"
	case store.EventEntityUpdated:
		return "e", "u"
	case store.EventEntityDeleted:
		return "e", "d"
	case store.EventRelationCreated:
		return "r", "c"
	case store.EventRelationUpdated:
		return "r", "u"
	case store.EventRelationDeleted:
		return "r", "d"
	default:
		return "?", "?"
	}
}

// feedOp maps the compact (kind, op) codes back to a store.EventOp.
func feedOp(kind, op string) (store.EventOp, bool) {
	switch kind + op {
	case "ec":
		return store.EventEntityCreated, true
	case "eu":
		return store.EventEntityUpdated, true
	case "ed":
		return store.EventEntityDeleted, true
	case "rc":
		return store.EventRelationCreated, true
	case "ru":
		return store.EventRelationUpdated, true
	case "rd":
		return store.EventRelationDeleted, true
	default:
		return 0, false
	}
}

// notify emits a NOTIFY for ev on this store's channel, carrying this store's
// origin. q is the write's transaction (or the pool) — calling it inside the
// write's transaction makes the notification atomic with the write: it fires
// only on commit, never on rollback.
//
// A store with no resolved channel (built via New without listener wiring, e.g.
// conformance tests) makes this a no-op. Errors are intentionally ignored: the
// notification is a best-effort hint and the seq catch-up recovers anything
// lost — a failed NOTIFY must never fail the write.
func (s *Store) notify(ctx context.Context, q DBTX, ev store.Event) {
	if s.channel == "" {
		return
	}
	// pg_notify takes (text, text), both bound parameters — no injection surface.
	_, _ = q.Exec(ctx, `SELECT pg_notify($1, $2)`, s.channel, notifyPayload(s.originID, ev))
}
