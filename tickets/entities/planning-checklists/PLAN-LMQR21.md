---
id: PLAN-LMQR21
type: planning-checklist
title: 'Planning: Live views 1/3: sequence-numbered change feed with SSE resume'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

Today the `/api/v1/_events` SSE feed is lossy with no gap signal. Events can be
dropped at the store subscription (64 slots), at the broker (4 slots per
client), and on postgres by a lost NOTIFY. The SPA reconnects by hand with
backoff and re-fetches nothing, so changes made while it was disconnected are
never shown until the next unrelated change of the same type. Frames carry no
`id:` and the server sends no heartbeat.

**Scope:**

In scope:

- A new package `internal/changefeed` with a `Bus`. It reads the store watcher
and keeps, per entity type, the sequence number of the last change, plus one
sequence number for the last relation change. It keeps no event history.
- Opaque resume tokens: AES-256-GCM over the sequence number, with a key made
per `Bus` from `crypto/rand`. A token from another process or an earlier run
does not decrypt, which tells the server the position is unknown.
- SSE: an `id:` on frames; a first frame `ready` (or `refresh` on resync);
resume from `Last-Event-ID` (header, wins) or `last_event_id` (query, because
the SPA rebuilds its EventSource by hand); an `event: ping` with an `id:` every
25 s; a write deadline on every write.
- SPA `useEvents.ts`: remember the last non-empty event id and pass it on
reconnect.
- Per-type ACL gating (TKT-POT9GQ) is unchanged. Resumed changes go through the
same gate as live ones.

Out of scope:

- Store-level sequence numbers (pg `RETURNING seq`, a sqlite seq column). The
bus sequence is enough for resume within one process. A cursor shared across
instances belongs with the postgres Bus in phase 5.
- Drops inside the store's own subscription channel. The bus reads it with a
1024 buffer and does O(1) work per event, so a drop is unlikely, but it stays
invisible as today. Recorded as a risk.
- Changes between the SPA's first data fetch and its first `ready` frame
(pre-existing).
- The desktop app not calling `App.StartWatching`: BUG-FODX82.
- Per-subscription frames with entity ids (TKT-HJ5Z9Y) and MCP watch
(TKT-FMS8BS).

**Acceptance Criteria:**

1. The first frame on a connection is `event: ready` with an `id:` (or
`event: refresh` with `{"reason":"resync"}` and an `id:` when the presented
token is invalid). Every `entity:changed` flush and every `ping` carries an
`id:`.
2. A client that reconnects with a token it was given receives one
`entity:changed` frame for every readable type that changed after that token,
and no `refresh`. This holds however long it was away and however many writes
happened.
3. A client that reconnects with the token from `ready` before the first flush
gets the same changes again (the `ready` token on resume is the presented
position, not the head).
4. A malformed, oversize or foreign token yields exactly one `refresh`
`{"reason":"resync"}` with a fresh `id:`, then live frames; never a 4xx.
5. A principal never receives a frame for a type it cannot read, live or
resumed. Whether and when a frame is sent depends only on changes to types it
can read, and on the fixed ping timer, never on the volume of hidden changes.
6. A Configure save (`Retire`) still delivers `config-changed` as the last
frame.
7. The SPA sends its last event id on reconnect and keeps handling `refresh`
as today (invalidate all entity queries).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-1UCUBA (phase 1, "Live views").

**Existing Solutions:**

- Libraries: r3labs/sse and tmaxmax/go-sse offer replay providers, but both
own the HTTP handler, and ours carries per-connection ACL gating and coalescing
that we would have to rebuild on top. A ring plus fan-out is about 150 lines; no
dependency.
- Codebase: `dataentry.eventBroker` (`watcher.go:62`) is the fan-out pattern
to follow; the per-backend `Subscribe` maps use the same non-blocking send.
`pgstore/listener.go` catch-up is the precedent for "detect a gap, then
re-snapshot".
- Reference: the SSE spec's `Last-Event-ID`; Phoenix/Ably "resume or
refetch"; Replicache/Zero "poke then pull" (RES-1UCUBA).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

`internal/changefeed` (imports `internal/store` only, for `store.Event`):

```go
type Source interface { Subscribe(bufSize int) (<-chan store.Event, func()) } // consumer-side

func New() (*Bus, error)              // key from crypto/rand; error if that fails, never a fixed key
func (b *Bus) Start(src Source) error // nil src, second Start, or Start after Close: error
func (b *Bus) Close()                 // idempotent, safe before Start; cancels src, waits for the pump
func (b *Bus) Subscribe(token string) *Subscription

type Batch struct { Types []string; Relation bool; Seq uint64 }

func (s *Subscription) Wake() <-chan struct{} // cap 1; never closed
func (s *Subscription) Take() Batch           // types with lastSeq > cursor; cursor = head
func (s *Subscription) Start() uint64         // position the subscription began at
func (s *Subscription) Resync() bool          // token present but not usable
func (s *Subscription) Token(seq uint64) string
func (s *Subscription) Close()
```

- Pump: under `b.mu`, `head++`, then `typeSeq[ev.EntityType] = head` (entity
ops) or `relSeq = head` (relation ops), then a non-blocking send on every
subscriber's wake channel. Attachment and other ops are ignored, as today. Seq,
state update and wake happen under one lock.
- Subscribe: an empty token starts at head. A token that decrypts to
`seq <= head` starts at `seq`, and the wake channel is pre-armed so the handler
takes the missed changes at once. Anything else sets `Resync` and starts at
head.
- Take is O(number of types). There is no lag state and no ring: a subscriber
that reads late just gets a larger set of types.
- When the pump exits (Close, or the store closed its channel), the Bus stops
updating. Subscriptions stay valid and are never closed by the bus, so a handler
never ends because of the bus.
- The Bus is a base source with no gate, like `store`. It holds no entity ids.
Consumers gate before anything reaches a client; `.go-arch-lint.yml` allows only
`dataentry` to import it for now.
- Tokens: `RawURLEncoding(nonce[12] || Seal(seq as 8 bytes big-endian))`, AAD
`"rela-changefeed-v1"`, exactly 48 characters (36 bytes). Length is checked
before decoding; decode is strict; `aead.NonceSize()` is used. A random nonce is
safe far beyond the number of tokens one process will mint (2^32). Tokens are
not bound to a principal on purpose: they carry no authority, and every replay
is gated by the current principal.

`internal/dataentry`:

- `NewApp` builds the Bus (an error fails `NewApp`); the field is never
reassigned. `StartWatching` calls `bus.Start(a.store)` in place of
`startStoreEventBridge`; `StopWatching` calls `bus.Close()`. `pumpStoreEvents`
goes away; the broker keeps the non-entity frames.
- `handleSSE` reads the token (header, then query, capped at 128 bytes),
subscribes, and writes `ready` with `id: Token(sub.Start())` or `refresh`
`{"reason":"resync"}` with `id: Token(sub.Start())`.
- `runSSELoop` takes the subscription and a `sseTimings{flush, ping}` value
(defaults 200 ms / 25 s; tests shorten them). It selects on broker, wake, flush
timer, ping ticker and ctx:
  - wake: `b := sub.Take()`; `regate = regate || b.Relation`; add each type to
`pending` (filtered at entry unless `regate`); `cursor = b.Seq`; arm the flush.
  - flush: as today (re-gate first), and the last visible frame carries
`id: Token(cursor)`; then `flushed = cursor`.
  - ping: `event: ping`, `data: {}`, `id: Token(flushed)`. `flushed` only
moves past changes that were resolved and written, so a client resuming from it
loses nothing.
  - Only a closed broker channel or ctx ends the stream (keeps `config-changed`
as the last frame on Retire).
  - Every write sets a write deadline (`http.NewResponseController`, now + 2 ×
ping); `ErrNotSupported` is ignored, any other error ends the stream.

`frontend/src/composables/useEvents.ts`: listen for `ready` and `ping`; read
`event.lastEventId` in the `ready`, `ping`, `refresh` and `entity:changed`
listeners and store it only when non-empty; on reconnect open
`/api/v1/_events?last_event_id=<id>`. `refresh` handling is unchanged.

**Alternatives rejected:**

- An event ring with per-subscriber channels (the first draft). Design review
found that resync and lag then depend on total write volume, which tells a
principal how much changed in types it cannot read; replay also cost up to 600
KB per reconnect. Per-type state removes both.
- Raw store sequence in `id:` (pg `rela_seq`): not available when the event is
emitted, no equivalent on sqlite/fs/mem, and a plain number counts hidden
writes.
- HMAC-signed `epoch.seq`: tamper-proof but still shows the count.
- A library (r3labs/sse, tmaxmax/go-sse): both own the handler, and the replay
they offer is the event ring rejected above.

**Files to modify:**

- `internal/changefeed/{changefeed.go,token.go,changefeed_test.go,token_test.go}` (new)
- `.go-arch-lint.yml`, `.testcoverage.yml`
- `internal/dataentry/watcher.go`, `internal/dataentry/app.go`
- `internal/dataentry/sse_resume_test.go` (new); `sse_acl_test.go` helper
`runSSEConn` updated for the new loop signature, assertions unchanged
- `frontend/src/composables/useEvents.ts`, `useEvents.test.ts`
- `docs/data-entry/api-reference.md` (SSE section)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `Last-Event-ID` header and `last_event_id` query: untrusted. Capped at 128
bytes, exact token length required, strict base64url, then GCM-opened, then `seq
<= head`. Any failure is a resync. The value is never echoed or logged.

**Security-Sensitive Operations:**

- Resumed changes are gated like live ones (AC5). A new connection builds a
fresh `acl.Request`, so membership that changed while the client was away
applies to the whole resume; a resumed relation change also re-gates.
- No volume channel: the bus state is per type, resync depends only on the
token, and nothing lags. Frame timing depends only on readable types and the
fixed ping timer (AC5).
- Tokens are encrypted, so the global change count is hidden.
- Resources: bus memory is O(types); a reconnect costs O(types); a client that
stops reading is dropped by the write deadline instead of holding its goroutine
forever.
- Existing protections stay: same-origin middleware, per-principal gate, audit
isolation (`TestSSE_DoesNotFlowAuditEvents`, now also run through the real store
→ bus → handler path).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- changefeed (table-driven, `-race`): seq advances by one per event; empty
token starts at head; a valid older token yields exactly the changed types and
Relation; a token at head yields nothing; a foreign token, a token beyond head
and malformed tokens set Resync; Take after many writes to one type returns it
once; Close is idempotent and safe before Start; Start twice or after Close
fails; New/Start reject nil; a store closing its channel stops the pump without
closing subscriptions; concurrent Subscribe/Take during publishing loses no
type.
- Tokens: round trip; one flipped byte, truncated, empty, non-base64, padded,
oversize all reject; two tokens for one seq differ.
- dataentry, loop driven with an injected subscription and short timings:
flush puts the id on the last visible frame; ping carries `flushed`, not a
cursor with unresolved changes; a hidden-type-only burst writes nothing;
relation in the same batch re-gates before gating types.
- dataentry, `httptest.NewServer` round trips on memstore with real writes:
AC1 ready + id; AC2 disconnect, write two types, reconnect with token, get two
frames and no refresh; AC3 reconnect with the `ready` token before the flush,
get them again; AC4 garbage token, get one refresh then live frames; AC5 resume
with a hidden type withheld, using the two-principal fixtures; AC6 Retire with a
connected client, `config-changed` is the last frame; audit isolation through
the real path.
- Frontend: reconnect URL carries `last_event_id` after a frame; an empty
`lastEventId` does not overwrite a stored one.

**Edge Cases:**

- Token exactly at head; token at seq 0 on a fresh bus; Bus never started
(desktop) still sends ready and ping; the store closes mid-stream.
- uint64 seq wrap: unreachable, noted in a comment.

**Negative Tests:**

- Malformed, oversize and foreign tokens resync and never return 4xx.
- Start with a nil source and a second Start return errors.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Drops inside the store's channel stay invisible. Mitigation: 1024 buffer and
O(1) work per event in the pump.
- Changing `runSSELoop` can regress the ACL tests. Mitigation: the gate code is
untouched and the `sse_acl_test.go` assertions stay as they are.
- Multi-instance deployments resync on every reconnect that lands on another
node. Accepted until the postgres Bus (phase 5).

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry/api-reference.md`: SSE `id:`, `ready`, resume,
`refresh` reason, ping.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-YS6N3Q, RR-L1BWRO (critical); RR-03H3RH,
RR-YB9VJC, RR-RBR2X9, RR-YDMHZT (significant); RR-ZT6PF4, RR-4K85IL, RR-7E76ZM,
RR-8G0JAQ (minor). All addressed in this plan; the event ring was replaced by
per-type state.
