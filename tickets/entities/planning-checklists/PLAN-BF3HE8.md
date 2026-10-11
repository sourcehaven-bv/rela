---
id: PLAN-BF3HE8
type: planning-checklist
title: 'Planning: Live views 2/3: per-subscription SSE frames via poke and gated re-read'
started: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

Today a write produces a type-level `entity:changed` frame. Every client then
refetches every active query of that type, whether or not the result changed.
Configured views (`/_views/{type}/{id}`) never update at all, because the SPA
loads them imperatively and they span several types.

This ticket lets a stream subscribe to specific GET resources. On a change, the
server re-runs each subscribed GET for the subscriber and compares a hash of the
response. Only when the gated response changed does it send a frame naming that
subscription. The client then refetches that one query.

**Scope:**

In scope:

- Subscribing to three GET shapes: a single entity
(`/api/v1/{plural}/{id}`), an entity list (`/api/v1/{plural}?...`) and a view
(`/api/v1/_views/{type}/{id}?...`).
- A connection id on the stream, and an endpoint to replace a connection's
subscription set.
- Server-side re-read through the real handler and the real ACL middleware,
a hash per subscription, and a `live:changed` frame when it differs.
- Coalescing, per-connection caps, dedupe of identical re-reads per
principal within one window, and a budget test.
- SPA: subscribe for entity detail (PageView), entity lists (EntityList) and
views (EntityDetail `loadView`), and refetch the named query on a frame.

Out of scope:

- Pushing the response body in the frame. The frame is a precise poke; the
client refetches. Body push is a follow-up once this is measured.
- Multi-instance. The subscription endpoint must reach the instance that
holds the stream. Day one is single-instance; the seam is documented.
- The MCP watch tool (TKT-FMS8BS).
- Removing the type-level `entity:changed` frames. They stay for nav
counts, documents and anything not yet subscribed.

**Acceptance Criteria:**

1. The stream's `ready` frame carries a connection id (`{"conn": "..."}`).
The legacy `/api/events` stream does not; live views are v1 only.
2. `PUT /api/v1/_events/{conn}/subscriptions` with a list of
`{key, path}` replaces the set and returns 204. A path that does not parse as
one of the three shapes, or carries a query key outside that shape's allowlist,
gets 400 naming the key.
3. A write that changes a subscribed resource as the subscriber sees it
produces one `live:changed {"key": ...}` frame within the coalescing window. A
write that does not change it produces no frame.
4. A write the subscriber cannot see produces no `live:changed` frame. This
holds for each shape, including an entity GET of a hidden id (404), and two
re-reads with no write in between hash the same.
5. When the subscriber loses read access to a subscribed entity (a
member-of edge is removed), a frame is sent, and the refetch returns the same
404 an ordinary GET returns.
6. A view subscription is notified when an entity it reaches through a
relation changes, not only the entry entity.
7. A `PUT` by a different identity to a connection id gets 404, the same as
an unknown id. A `PUT` by the same identity whose claims changed (token refresh)
gets 409 `stream_principal_stale`, and the SPA reconnects.
8. A write that lands between the client's fetch and its subscription is
reflected: after the PUT returns 204 the SPA refetches each newly added key
once.
9. Re-read cost per round is one replay per distinct (principal, path)
across all connections, however many writes landed in the window. Pinned by a
budget test with `storetest.Counting`.
10. A re-read returns exactly what a real GET returns: for a redacted field,
a `?world=` path and a principal resolved through `principal_property`, the
replay body equals the GET body.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-1UCUBA (option A3: poke, then re-read through visibility,
batched).

**Existing Solutions:**

- Hasura live queries group subscribers by query and role and re-run the
query; Replicache sends a contentless poke and lets the client pull; Mercure and
Ably use an HTTP call beside a one-way stream to change subscriptions. This plan
combines the three.
- The read handlers are tangled with `http.ResponseWriter`:
`handleV1GetEntity` (`api_v1.go:945`), `handleV1ListEntities` (`:760`) and
`viewsHandler.handleV1Views` (`views_handler.go:854`). Extracting DTO builders
from all three is large and creates a second read path that can drift from the
REST one.
- The router middleware already builds a fresh `acl.Request` and read gate
per request (`attachACLRequest`, `router.go:402`) and checks the world grant
after it (`attachWorld`). A fresh Request per re-read also solves the memoized
member-of closure noted in `freshReadGate`.
- Budget test template: `TestQueryBudget_ListPageIsSizeIndependent`
(`querybudget_test.go:293`).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Replay target.** In `NewRouter`, build a second `http.ServeMux` with
`registerAPIV1Routes`, so the replay reaches the same handlers as production but
cannot reach `_events`, `/hooks/` or the SPA. Wrap it in every wrapper
production applies that changes what a GET returns: the reload lock,
`attachWorld`, and `attachACLRequest` with the same `d` and JWT flag (a plain
chain when the ACL is not `*acl.Declarative`). Leave out the transport checks
(`requireLocalHost`, `requireSameOrigin`, CSRF, request stats): they judge the
browser request, which the stream already passed, and do not change a response
body. A parity test (AC10) pins that a replay and a GET agree.

2. **Replay principal.** A small middleware, placed after the JWT gate and
before `attachACLRequest`, saves the stamped, not yet resolved principal on the
context. `handleSSE` and the PUT handler read it from there. A replay puts that
principal on a fresh context with no `acl.Request`, so `attachACLRequest`
resolves it exactly as it does for a real GET, with a fresh membership walk each
time. The replay context is derived from an App-owned context that
`StopWatching` cancels, with a 5 s deadline per replay.

3. **Paths are parsed, not matched.** `parseLivePath` turns the client's
string into a typed `livePath{shape, plural, id, viewType, query}` and the
replay URL is rebuilt from it, never taken from the client:
   - the path must equal its `path.Clean` form, with no `//`, `..`, `%2F`
or fragment, and no scheme or host;
   - entity: `/api/v1/{plural}/{id}`, where `plural` is in the current
schema's plural set and `id` passes `isSafePathSegment` and does not start with
`_`; query keys: `world`, `include`;
   - list: `/api/v1/{plural}`, same plural rule; query keys: the list
control parameters `listPage` reads (enumerated in `live.go`, with a test that
each one is accepted) plus the type's declared property names, which are
filters;
   - view: `/api/v1/_views/{type}/{id}`, both single safe segments; query
key: `world`. Everything else is 400, including `_export`, `_documents`,
`_history` and `_comments`. No shape can reach Lua, documents, exports or
commands.

4. **Live worker.** One App-owned worker, started by `StartWatching` and
stopped by `StopWatching`, owns all re-reads. It subscribes to the change feed.
On a wake it waits out a 500 ms window, takes the batch, and runs one round:
   - collects the distinct (fingerprint, path) pairs over all live
subscriptions;
   - replays each once, with at most 4 in parallel, and hashes status plus
body (sha256); headers are never hashed;
   - compares each subscription's stored hash with the new one, stores it,
and hands changed keys to the subscription's connection;
   - logs one line per round: replay count, duration, store queries
(`store.WithQueryStats`).

Rounds never overlap; changes during a round start the next one. A round caps at
256 replays. Pairs beyond the cap are not replayed: their subscriptions get a
frame and their hash is cleared, so the next round takes a fresh baseline
without a frame. That falls back to today's refetch cost instead of queueing. A
replay that fails without an HTTP status (timeout, cancellation) keeps the old
hash and is logged.

The fingerprint is a sha256 of the principal's full JSON form
(`principal.MarshalJSON`), which covers user, tool, roles, principal type,
scopes and organisation: every field `principal.Equal` compares. Two principals
that differ in any field never share a replay.

5. **Registry.** `handleSSE` mints a random connection id (128 bits,
base64url), registers a `liveConn{principal, subs, changed}` in the App's
mutex-guarded `liveRegistry`, puts the id in the `ready` data, and deregisters
on exit. `changed` is a mutex-guarded key set plus a cap-1 wake channel, the
same pattern as `changefeed.Subscription`. The stream loop selects on that
channel and writes one `live:changed` frame per key. Streams only write frames;
they never replay.

6. **PUT endpoint.** Registered on the inner mux, so the CSRF, origin and
JWT checks apply. PUT only; body limited with `http.MaxBytesReader` to 16 KiB;
`{"subscriptions":[{"key","path"}]}`; keys at most 64 chars of
`[A-Za-z0-9_:.-]`, unique; at most 16 per connection and 64 per principal across
connections (429 above that). Ownership is decided on identity (`User`, `Tool`,
`RawUser`, organisation of the stored pre-resolution principal): a different
identity or an unknown id gives 404; the same identity with different claims
gives 409 `stream_principal_stale`. New keys start with no hash; the next round,
or an immediate round the PUT triggers, takes the baseline without a frame. A
key whose path is unchanged keeps its hash.

7. **SPA.**
   - `useEvents` exposes the connection id (from `ready`).
   - `useLiveSubscription(key, path, onChange)` registers with a manager
that sends the full set in one debounced PUT whenever it or the connection id
changes. On 204 it calls `onChange` once for each key that was added (AC8). On
404 it waits for the next `ready`; on 409 it reconnects the stream.
   - The subscribed path comes from the same URL builder the query uses
(`api/` client functions export a `...Path(params)` helper), so the tracked
response is the fetched one. A test asserts the two are equal.
   - PageView and EntityList invalidate their exact query key on change;
EntityDetail re-runs `loadView`. Type-level `entity:changed` handling stays as
it is.

**Alternatives considered:**

- *Subscriptions in the SSE URL, reconnecting on every change.* Stateless
and multi-instance friendly, and the resume token makes the reconnect lossless.
Rejected because pagination and filters change often, and each change would tear
down the stream and redo every baseline. Kept as the fallback if the
multi-instance seam proves hard.
- *DTO builders extracted from the three handlers.* Rejected for this
slice: large, and it creates a second read path that can drift from REST in
gating. The handler-replay approach makes "same as GET" structural.
- *Push the body in the frame.* Saves the client refetch, but the SPA's
query functions post-process responses, so setting query data needs work per
query. Follow-up.
- *Re-read only subscriptions whose types changed.* Wrong for views and ACL
changes; see step 3.

**Files to modify:**

- `internal/dataentry/livepath.go` (new): `parseLivePath`, allowlists,
replay URL.
- `internal/dataentry/live.go` (new): registry, live worker, replay,
fingerprint, PUT handler.
- `internal/dataentry/livepath_test.go`, `live_test.go`,
`live_budget_test.go` (new).
- `internal/dataentry/watcher.go`: connection id, ready data, live frames in
`sseStream`.
- `internal/dataentry/router.go`: pre-resolution principal middleware,
replay chain, PUT route.
- `internal/dataentry/app.go`: registry and worker fields; start and stop in
`StartWatching` / `StopWatching`.
- `frontend/src/composables/useEvents.ts`, new
`frontend/src/composables/useLiveSubscription.ts`, path helpers in
`frontend/src/api/`, `PageView.vue`, `EntityList.vue`, `EntityDetail.vue`,
tests.
- `docs/data-entry/api-reference.md`, `docs/acl-security.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Connection id in the PUT path: registry lookup, then identity check.
Unknown and different identity both give 404.
- Subscription paths: parsed into typed parts as in step 3, then the replay
URL is rebuilt from those parts. Allowlist per shape for path segments and query
keys.
- Keys: length and character allowlist, unique, count caps. Body capped at
16 KiB.

**Security-Sensitive Operations:**

- The replay runs as the subscriber through the same wrappers as a GET
(step 1), starting from the pre-resolution principal (step 2), so row gating,
field redaction, world grants and principal resolution match a real GET. AC10
pins it. No raw store read happens in this code.
- A frame carries only the client's own key, and is sent only when that
principal's GET response changed. AC4 pins that responses are stable under
hidden writes, which is what makes this no more revealing than polling.
- Replays are shared only between connections with an identical principal
(full JSON fingerprint), never across principals or ceilings.
- Connection ids are random and bound to an identity.
- Residual, accepted and documented in `acl-security.md`: the stream
already outlives the JWT's expiry, and now a frame can follow a change for a
principal whose IdP claims were revoked after connecting. It is one bit per
subscription; the refetch itself needs a valid token, and a config reload ends
every stream. Ending streams at token expiry is follow-up TKT-5I6REX, because
the verified principal does not carry `exp` today.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** (real HTTP server, real store writes, as in
`sse_resume_test.go`)

1. AC1: ready data has a `conn` field of the expected length.
2. AC2: three valid shapes give 204. 400 for: `/api/v1/_events`,
`/api/v1/tickets/_export`, `/api/v1/_documents/x`, `/api/v1/_views/ticket/a/b`,
`%2F` in an id, `//`, `..`, an absolute URL, an unknown query key, a 17th entry,
duplicate keys, bad key characters.
3. AC3: subscribe to TKT-1 and to the ticket list; update TKT-1 and get
frames for both keys; update an unrelated feature and get none.
4. AC4: with ACL, alice reads tickets only. For each shape, plus an entity
GET of a hidden id: two rounds with no write give equal hashes; a hidden write
gives no frame.
5. AC5: alice reads TKT-1 through group membership; subscribe; remove the
member-of edge; get a frame; a GET returns 404.
6. AC6: a view whose section lists related features; update a related
feature; get a frame for the view key.
7. AC7: bob's PUT to alice's id and a random id give 404; alice with an
extra role gives 409.
8. AC8: SPA unit test: after a 204 the manager calls `onChange` for added
keys only.
9. AC9: budget test: two connections of one principal subscribe to the same
list; 1 write and 20 writes in one window each cost one replay. Two principals
differing only in scopes cost two replays.
10. AC10: parity test: replay body equals GET body for a redacted field, a
`?world=` path, and a `principal_property`-resolved principal.

**Edge Cases:**

- PUT before the first `ready` (no id yet): the SPA waits for the id.
- Stream closes while a PUT is in flight: 404, the SPA re-sends after the
next `ready`.
- Subscription whose GET returns an error status: the status is part of the
hash; a change from 500 to 200 sends a frame.
- Empty subscription list: clears the set.
- Duplicate keys in one PUT: 400.
- Resume (slice 1): subscriptions do not survive a reconnect; the SPA
re-sends them for the new connection id, and the baseline re-read covers the
gap. A change that landed while disconnected is also signalled by the replayed
`entity:changed` frames.

**Negative Tests:** invalid JSON, oversize body, bad key characters, path with
host, path with `..`, a POST-only route, an unknown connection id, and another
principal's connection id.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Re-read cost on busy projects.* Bounded by one round at a time, 500 ms
coalescing, 16 subscriptions per connection and 64 per principal, a 256 replay
cap per round with refetch fallback, and dedupe per principal. The per-round log
line shows the cost; the replay bypasses request stats, so the request log does
not.
- *Search index lag.* A list with `q=` could be re-read before the search
observer indexed the write. Check the store's observer notification order during
implementation; if the index can lag, the round waits for it or the `q=` key is
left out of the allowlist for now.
- *Middleware drift.* A new wrapper that changes responses but is added only
to the production chain would make replays diverge. The parity test (AC10)
catches the gates it covers; a comment at both wrap sites points to the other.
- *Multi-instance.* The PUT must reach the stream's instance. Documented as
needing sticky routing until a bus forwards subscriptions.
- Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: no metamodel change)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI change)
- [x] docs/data-entry.md - UI changes (api-reference.md SSE section: connection id, PUT endpoint, live:changed; acl-security.md: why a live frame reveals no more than polling)
- [x] ~~CLAUDE.md - New patterns or conventions~~ (N/A: the handler-replay rule is documented in live.go and acl-security.md)
- [x] ~~README.md - Project-level changes~~ (N/A)
- [x] ~~N/A - Internal change, no user-facing docs needed~~ (N/A: there are user-facing docs)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** critical RR-0Z5C87, RR-EHRNBL; significant
RR-BJ2ZWT, RR-FWZFGG, RR-WASIKX, RR-ZL2D8E, RR-7SO85U, RR-FFPNE9; minor
RR-HNXABX. All addressed in this plan. Token-expiry follow-up: TKT-5I6REX.
**Architecture review** (go-architect, run after implementation against the code
and this plan): the architecture is sound. Significant RR-0YX25V, RR-UC90C0 and
RR-7AW45P, minor RR-36N6SA and RR-RE0WCF, and nit RR-02T2UP are addressed. Minor
RR-6VNZWR and nit RR-G1EVIB are deferred to TKT-H6LD1Y (move the hub to
`internal/livequery`, with the multi-instance work).

**Changes to the approach above, from code and security review:**

- Rounds run on a fixed 500 ms ticker when anything changed, not in a window
opened by the first change (RR-0JGGEY).
- The PUT takes each new subscription's baseline before answering 204; a
baseline it could not take is taken by a round, with a frame (RR-LAX75I).
- A round checks at most 256 pairs and starts no replay after 10 s; the cut
rotates, and skipped pairs are framed and keep their baseline (RR-HJ310L,
RR-0H0LOU).
- One replay limit (4) covers rounds and PUT baselines (RR-7AW45P).
- The identity cap does not apply to unauthenticated callers (RR-U74D0D).

**Multi-instance:** each instance replays its own streams' subscriptions from
its own change feed. A PUT must reach the instance holding the stream (the
connection id is opaque, so an instance prefix can be added without client
changes), and the identity cap counts per instance. Resume tokens are sealed per
process, so a reconnect to another instance resyncs (slice 1).
