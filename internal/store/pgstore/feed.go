package pgstore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// notifyDisabled suppresses the producer's pg_notify when set, so a test can
// commit an ordinary write whose live notification never fires and assert that
// the seq-watermark catch-up still recovers it. Production never sets it. An
// atomic so it's safe vs. the writer goroutines that read it; the listener
// tests that flip it run sequentially (no t.Parallel), like catchUpInterval.
var notifyDisabled atomic.Bool

// The cross-process change feed (TKT-WZYWM9). Writes emit a PostgreSQL
// NOTIFY carrying the changed identity; a listener in another process turns
// those notifications back into store.Events. See listener.go for the consumer
// and the seq-watermark catch-up that makes delivery recoverable.
//
// # Channel scoping
//
// LISTEN/NOTIFY channels are DATABASE-global, not schema-scoped, and many
// schemas can share one database (the conformance harness, the postgres e2e,
// and a multi-tenant deployment all do). Isolation is therefore enforced by
// carrying the writing schema IN THE PAYLOAD and filtering on receipt, NOT by
// giving each schema its own channel name.
//
// The channel is a single constant (rela_changed). That is deliberate: LISTEN
// requires a dedicated session per connection, so a per-schema channel name
// would cost one permanently-held connection PER SCHEMA before any pooling.
// A constant channel lets one connection serve every schema a process touches.
// The listener already filters (it drops its own echoes by originID), so schema
// filtering is one more predicate rather than new machinery.
//
// What this does NOT share is the seq-watermark catch-up: `rela_seq` is
// per-schema and the catch-up query is unqualified SQL resolved through the
// connection's search_path, so priming and catching up stay bound to each
// store's own pool. See listener.go.
//
// # Self-echo
//
// A process that writes also receives its own NOTIFY back. The payload carries
// the writing store's random originID; the listener skips notifications whose
// origin matches its own store (those writes were already emitted in-process).

// feedChannel is the single NOTIFY channel every rela process LISTENs on,
// regardless of schema. The writing schema travels in the payload instead of
// the channel name — see the "Channel scoping" note above for why.
const feedChannel = "rela_changed"

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

// resolveSchema returns the schema this store's tables live in — the first
// entry of the active search_path (current_schema()). It is stamped into every
// NOTIFY payload and matched on receipt, so it is what keeps two schemas
// sharing one channel from cross-talking.
//
// A schema containing the payload separator would shift every field and break
// the parse, so it is rejected here rather than trusted. PostgreSQL permits
// almost anything inside a quoted identifier, and schema names are
// operator- (or, under multi-tenancy, tenant-) derived, so this is a trust
// boundary and not a formality. Failing loudly at Open beats emitting
// unparseable notifications for the life of the process.
func resolveSchema(ctx context.Context, db DBTX) (string, error) {
	var schema string
	if err := db.QueryRow(ctx, `SELECT current_schema()`).Scan(&schema); err != nil {
		return "", fmt.Errorf("pgstore: resolve schema for change feed: %w", err)
	}
	if schema == "" {
		schema = "public"
	}
	if strings.Contains(schema, payloadSep) {
		return "", fmt.Errorf("pgstore: schema name %q contains the change-feed payload separator", schema)
	}
	return schema, nil
}

// feedEvent is the wire form of a change: the writing store's origin and
// schema, plus the fields needed to reconstruct a store.Event. Encoded as 8
// SEP-joined fields:
//
//	origin SEP schema SEP kind SEP op SEP entityType SEP a SEP b SEP c
//
// where (a,b,c) are (id,"","") for an entity or (from,relType,to) for a
// relation, and entityType is the entity's type ("" for relations).
//
// origin and schema are the two routing fields — the listener drops a
// notification whose origin is its own (already emitted in-process) or whose
// schema is not the one its store reads. Everything after them is event data.
//
// The separator (\x1f, a control char) cannot appear in an id or relation type
// because storeutil.ValidateID rejects control characters, and resolveSchema
// rejects it in a schema name. entityType is NOT validated for control chars on
// write; a type containing \x1f would inject an extra field, fail the 8-field
// parse, and degrade that one notification to a catch-up (handleNotification
// returns needCatchUp) — never a corruption.
type feedEvent struct {
	origin string
	schema string
	ev     store.Event
}

// notifyPayload encodes a store.Event (with this store's origin and schema) for
// pg_notify. Entities use the EntityType/EntityID fields; relations use
// RelationType/From/To.
func notifyPayload(origin, schema string, ev store.Event) string {
	kind, op := feedKindOp(ev.Op)
	var a, b, c string
	switch ev.Op {
	case store.EventEntityCreated, store.EventEntityUpdated, store.EventEntityDeleted:
		// The id slot carries the STATE reference in its boundary
		// serialization (TKT-DOFYR1): "PAGE-1" or "PAGE-1@draft". '@' is
		// distinct from the '\x1f' field separator, and the codec is the
		// one legal producer of the joined form at a wire boundary.
		a = entity.FormatStateRef(ev.EntityID, ev.Pointer)
	case store.EventRelationCreated, store.EventRelationUpdated, store.EventRelationDeleted:
		a, b, c = entity.FormatStateRef(ev.From, ev.Pointer), ev.RelationType, ev.To
	}
	// EntityType is carried for entity events so the listener can rebuild a
	// faithful event; relations don't carry a type beyond RelationType.
	return strings.Join([]string{origin, schema, kind, op, ev.EntityType, a, b, c}, payloadSep)
}

// parseFeedPayload reverses notifyPayload. Returns ok=false for any malformed
// payload (the listener then falls back to the catch-up query rather than
// trusting a bad NOTIFY).
// payloadFields is the number of SEP-joined fields in a notify payload:
// origin, schema, kind, op, entityType, a, b, c.
const payloadFields = 8

func parseFeedPayload(payload string) (feedEvent, bool) {
	parts := strings.Split(payload, payloadSep)
	if len(parts) != payloadFields {
		return feedEvent{}, false
	}
	origin, schema, kind, op := parts[0], parts[1], parts[2], parts[3]
	entType, a, b, c := parts[4], parts[5], parts[6], parts[7]
	feOp, ok := feedOp(kind, op)
	if !ok {
		return feedEvent{}, false
	}
	ev := store.Event{Op: feOp}
	switch kind {
	case "e":
		id, ptr, err := entity.ParseStateRef(a)
		if err != nil {
			return feedEvent{}, false // malformed → catch-up, never a bad event
		}
		ev.EntityType, ev.EntityID, ev.Pointer = entType, id, ptr
	case "r":
		from, fp, err := entity.ParseStateRef(a)
		if err != nil {
			return feedEvent{}, false
		}
		ev.From, ev.Pointer, ev.RelationType, ev.To = from, fp, b, c
	}
	return feedEvent{origin: origin, schema: schema, ev: ev}, true
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

// notify emits a NOTIFY for ev on the shared feed channel, carrying this
// store's origin and schema so receivers can route it. q is the write's
// transaction (or the pool) — calling it inside the write's transaction makes
// the notification atomic with the write: it fires only on commit, never on
// rollback.
//
// A store with no resolved schema (built via New without listener wiring, e.g.
// conformance tests) makes this a no-op. Errors are intentionally ignored: the
// notification is a best-effort hint and the seq catch-up recovers anything
// lost — a failed NOTIFY must never fail the write.
func (s *Store) notify(ctx context.Context, q DBTX, ev store.Event) {
	if s.schema == "" {
		return
	}
	if notifyDisabled.Load() {
		return // test hook: force the catch-up path to be the only delivery channel
	}
	// pg_notify takes (text, text), both bound parameters — no injection surface.
	_, _ = q.Exec(ctx, `SELECT pg_notify($1, $2)`, feedChannel, notifyPayload(s.originID, s.schema, ev))
}
