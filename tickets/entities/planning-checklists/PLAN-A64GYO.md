---
id: PLAN-A64GYO
type: planning-checklist
title: 'Planning: Live views 3/3: MCP watch tool'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

An agent that wants to react to changes must poll a read tool today. The `watch`
tool lets it wait: it takes a list of read-tool calls ("targets") and a cursor,
and returns when the result of one of them changes for the calling principal, or
when a timeout passes. It applies the slice 2 idea (poke, then a gated re-read,
then compare a hash) to MCP tool results instead of HTTP GETs.

**Scope:**

In scope:
- One tool, `watch`, on both transports (stdio `rela mcp` and remote MCP in
rela-server).
- Targets are calls to the read tools `show_entity`, `list_entities` and
`list_relations`, with their normal arguments. `search_entities` is left out:
its ranking uses index-wide term statistics, so a hidden write can reorder
visible hits (RR, design review).
- An opaque cursor that carries a keyed digest of each target's result.
- Limits: targets per call, timeout, concurrent watches per identity and per
server, and a shared replay limit.
- Docs in `docs/mcp-server.md`.

Out of scope:
- MCP `subscriptions/listen` and resource subscriptions (go-sdk v1.8.0 has
them; adopt when clients support them; the research names `watch` as the bridge
until then).
- Progress-notification heartbeats during the wait.
- Sharing replays between watches (slice 2 shares per principal and URL;
here each watch replays its own targets).
- Multi-instance: a remote watch sees what its instance's store feed sees;
pgstore's listener already delivers other processes' commits.

**Acceptance Criteria:**

1. `watch` without a cursor returns at once with a cursor and `changed: []`.
Test: call it; the result has a non-empty cursor, `changed` empty, `timed_out`
false, and it took less than one tick (500 ms).
2. `watch` with a cursor blocks until a target's result changes and returns
the indices of the changed targets and a new cursor. Test: watch `show_entity
TKT-1` and `list_entities type=feature`; update TKT-1 from another goroutine;
the result is `changed: [0]`; calling again with the new cursor blocks.
3. A change the caller cannot see does not end the wait. Test: under a
policy that hides type `secret`, watch `list_entities type=ticket` and
`show_entity SEC-1`; update SEC-1 and create another secret entity; the watch
times out with `changed: []`.
4. With no change, the watch returns at the timeout with `timed_out: true`
and a cursor that holds the same digests. A target added to the call but missing
from the cursor is in the returned cursor, so a change after that call is
reported by the next one. Test: timeout 1 s, no writes; then add a target, time
out, change it, and call again.
5. A change made between two calls is reported by the second call at once.
Test: get a cursor, update TKT-1, then call with the cursor; it returns at once
with `changed: [0]`.
6. Invalid input is a tool error, not a hang: an unknown or write tool in a
target, more than 16 targets, a malformed cursor, a timeout outside 0-50 s.
Test: a table of each.
7. Limits: the 5th concurrent watch of one identity and the 33rd of a
server get a tool error that says to retry later. Test: start 5 watches of one
identity concurrently; exactly 4 block and 1 fails; one finishes, the next
succeeds.
8. Access is resolved again for every replay pass. Test: under a policy
where alice reads `ticket` through a group she is a member of (global roles come
from the membership walk), watch `show_entity TKT-1` with a cursor; delete the
membership relation; the watch reports `changed: [0]`, and its replay held the
not-found result, never TKT-1's content. Mutation check: without `Rebind` the
test fails (the cached roles keep the old result, so no change is reported).
9. Cancelling the call (client disconnect or `notifications/cancelled`)
ends the wait and releases the subscription. Test: cancel ctx; the handler
returns, and the store has no subscriber left.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-1UCUBA (FEAT-GWF8YH). It names "a `watch` tool that
long-polls with a resume cursor" as the agent surface until go-sdk
`subscriptions/listen` is usable.

**Existing Solutions:**

- go-sdk v1.8.0 has `subscriptions/listen` and `ResourceUpdated`, but
clients do not yet use them, and remote MCP runs `Stateless: true`, where the
server can only send notifications during an in-flight request. A long-polling
tool works on every client today.
- go-sdk runs every `tools/call` asynchronously (`server.go:2001`,
`jsonrpc2.Async`), so a blocked `watch` does not stall other tools on the stdio
connection.
- Slice 2 (`internal/dataentry/live.go`): poke from the store feed, gated
replay, hash, compare; a fixed-clock round so wait times do not depend on hidden
writes; shared replay limit; per-identity cap. `watch` reuses the ideas, not the
code: `internal/mcp` may not import `dataentry` or `changefeed` (arch-lint), and
the stdio server has no HTTP router to replay through. Replaying the MCP read
tools is the equivalent.
- `store.Watcher.Subscribe(bufSize)` (`internal/store/store.go:1869`) is the
feed: post-commit on pg/sqlite, cross-process on pgstore through its listener
(`pgstore/listener.go:248`), external edits on fsstore when the file watcher
runs (stdio starts it).
- Long-poll with an opaque cursor is the model of the Kubernetes watch
`resourceVersion`, CouchDB `_changes?feed=longpoll&since=`, and Dropbox
`list_folder/longpoll`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Tool schema, `watch`:
- `targets` (array, required): each `{tool, args}`, `tool` one of
`show_entity`, `list_entities`, `list_relations`; `args` is the object that tool
takes. 1-16 targets.
- `cursor` (string, optional): from a previous `watch`.
- `timeout` (number, optional): seconds, default 30, 0-50. 0 checks once
and returns. 50 s stays under a common 60 s proxy read timeout with room for a
replay pass.
- Result (text JSON like every tool): `{cursor, changed: [indices],
timed_out}`.

Handler (`internal/mcp/tools_watch.go`, type `watchHandler`; unrelated to the
file watcher `Deps.Watcher`). `NewServer` builds one `watchHandler` holding the
limits, the read-tool map and the cursor key, and registers it through a
closure, so `Server` gains no methods (it is at 25 of 27).
1. Validate targets, timeout and cursor. Errors are `errorResult`.
2. Take a watch slot: a server-wide count (max 32) and a per-identity
count (max 4, identity by `principal.SameIdentity` over the active watches),
checked and registered under one mutex and released in a `defer`. Over a cap:
`errorResult("too many concurrent watches; retry later")`.
3. Subscribe to the change source (buffer 64) BEFORE the first replay, so a
change during the replay still wakes the loop. Cancel on return.
4. A replay pass: first `Deps.Rebind(ctx)` gives a ctx with freshly
resolved read access (see below), then each target is replayed and digested. A
replay calls the registered handler for that tool: `addTool` now returns the
wrapped handler, and `registerTools` puts the three allowlisted read tools in a
map that never changes after construction, so unknown arguments are refused the
same way. The request is `&mcpgo.CallToolRequest{Params:
&mcpgo.CallToolParamsRaw{Name, Arguments}}`; no handler reads `Session`. Each
handler still reads the current deps snapshot, so a schema reload reaches it.
The digest is HMAC-SHA256 (process key) over the caller's
`principal.ShareKey()`, the target key, `IsError` and the text content,
truncated to 16 bytes. Replays run one after another within a watch (the ctx's
`acl.Request` is not safe for concurrent use, the slice 2 data race). Each
replay holds one slot of a server-wide semaphore of 4, and an identity holds at
most one slot at a time, so one identity cannot starve the others. Each replay
has a 5 s timeout and a pass stops starting replays after 10 s; the next pass
starts at the first target it skipped. A replay that panics, times out or is
skipped is "not finished": the target keeps its previous digest and is not
reported.
5. No cursor: return `{cursor: current, changed: [], timed_out: false}`.
6. Compare with the cursor. Any target whose digest differs is reported at
once. A target missing from the cursor takes the current digest as its baseline
and is not reported (like a new slice 2 subscription; the docs tell agents to
read after the first `watch`). Every return carries a cursor built from the
latest finished digests, with unfinished targets keeping the digest they had, so
a target added on this call is baselined in the returned cursor even on a
timeout.
7. Wait loop on a fixed 500 ms ticker: a store event marks the watch dirty;
on a tick, if dirty, run a replay pass and compare. Return on a change, on the
deadline (`timed_out: true`), or on ctx done (return the ctx error). The fixed
clock keeps the return time from depending on writes the caller cannot see, as
in slice 2.

Cursor: base64url of JSON `{"v":1,"h":{"<target key>":"<digest>"}}`. The target
key is the hex sha256 of the canonical JSON of `{tool, args}` (decoded into
`map[string]any`, so `encoding/json` sorts keys and numbers are uniform),
truncated to 16 bytes. The digest is keyed with a random per-process key and
bound to the principal, so a cursor read from a log cannot be used to test
guesses offline and does not work for another principal. A cursor from another
process (restart, other instance) or principal does not match, so every target
is reported changed once: the agent re-reads, which is safe, and the new cursor
is valid.

Fresh read access per pass: on remote MCP the call's ctx carries the
`*acl.Request` that `attachACLRequest` opened, which caches global roles for its
lifetime. A pass must not reuse it. `appbuild.GatedReadBundle` gains `Rebind
func(ctx) (context.Context, error)`: it detaches the attached request
(`acl.WithRequest(ctx, nil)`) and binds a fresh one with
`visibility.DeclarativeGate.Bind` for the ctx principal; without a policy it
returns ctx unchanged. Both wirings pass it as the required `Deps.Rebind`. The
principal itself (including its `principal_property` resolution) is the one the
call was authenticated as; a watch lasts at most 50 s, the same bound as a slow
request on that token.

Change source: a consumer-side interface in `internal/mcp`, `ChangeSource{
Subscribe(bufSize int) (<-chan store.Event, func()) }`, as a new required
`Deps.Changes` field, read once from the snapshot when a watch starts. It has
the method set of `store.Watcher`; it is declared at the call site so MCP test
stubs need not import store's watcher contract, per the consumer-side interface
rule. Both wirings pass `svc.Store()`; `reload()` reuses the old store, so a
subscription survives a schema reload. `NewServer` rejects nil.
`internal/changefeed` is not used: its type-level batches and sealed tokens
solve the SSE problem, and a watch needs only "something changed" pokes.

**Alternatives rejected:**
- Leave the stale request on the ctx and accept up to 50 s of stale roles
(the architecture review's option): a re-resolution per pass is cheap and
removes the gap.
- Return the type-level change feed (like the SSE `entity:changed`) with a
changefeed token: agents would re-read everything on every change, and
`internal/mcp` cannot import `changefeed`.
- Server-side watch registrations with an id (like the slice 2 PUT): remote
MCP is stateless, so the state would need a shared store; the cursor carries it
instead.
- Return the new result in the watch response: duplicates the read tools'
output and enlarges every response; the agent re-reads what it needs.

**Files to modify:**
- `internal/mcp/tools_watch.go` (new): tool definition, `watchHandler`,
cursor, limits.
- `internal/mcp/tools_watch_test.go` (new).
- `internal/mcp/toolspec.go`: `addTool` returns the wrapped handler.
- `internal/mcp/tools.go`: register `watch`; build the read-tool map.
- `internal/mcp/server.go`: `ChangeSource`, `Deps.Changes`, `Deps.Rebind`,
nil checks, the `watchHandler` built in `NewServer`; server instructions mention
`watch`.
- `internal/appbuild/appbuild.go`: `GatedReadBundle.Rebind`.
- `internal/cli/mcp_wiring.go`, `cmd/rela-server/mcp.go`: pass
`svc.Store()` and `reads.Rebind`.
- Test stubs constructing `Deps`.
- `docs/mcp-server.md`: a "Watching for changes" section.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `targets[].tool`: allowlist of three read tools. Anything else (write
tools, `lua_*`, attachments, `watch` itself) is a tool error naming the allowed
set.
- `targets[].args`: passed to that tool's handler, which validates as it
does today (including the unknown-argument check of `addTool`).
- `targets` length 1-16; `timeout` 0-50, rejected outside.
- `cursor`: at most 4 KiB, base64url JSON with `v == 1`; anything else is
"invalid cursor". Unknown keys ignored.

**Security-Sensitive Operations:**
- Reads: every replay runs the read tool under the caller's principal,
through `Deps.Store`, the gated reader, with an `acl.Request` opened fresh for
the pass. A hidden entity reads as not found before and after its writes, and
list totals are counted after gating (`tools_entity.go:79`), so hidden writes do
not change a digest. `search_entities` is excluded because its ranking does not
have this property.
- Timing: rechecks run on a fixed 500 ms clock, so when a watch returns
does not depend on hidden writes. The residual is that replay duration can vary
with hidden data, as in slice 2 (`docs/acl-security.md`).
- Cursor forgery or reuse: digests are keyed with a per-process secret and
bound to the principal, so a leaked cursor cannot be tested offline and matches
nothing for another principal. A forged cursor only causes "changed" reports,
followed by reads the caller is allowed anyway.
- Token lifetime: a watch runs on the principal it was authenticated as
for at most 50 s; access (roles, grants) is re-resolved every pass.
- Resource use: at most 32 watches per server and 4 per identity, 16
targets each, one replay pass per 500 ms tick, 4 replays at once across the
server and 1 per identity, 5 s per replay, 10 s per pass. On rela-server this
pool is separate from the slice 2 pool, so at most 8 gated replays run at once.
Store subscriptions are released on every return path.
- Stdio runs as the system principal with no ACL; nothing changes there.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** one test per acceptance criterion in
`internal/mcp/tools_watch_test.go`, against a memstore-backed server built with
`NewServer` and a real client over `mcpgo.NewInMemoryTransports`
(`dispatch_test.go` `connect`), with a new ctx-taking `callToolCtx`. AC3 and AC9
use `NewServer(policyDeps, WithPrincipal(alice))`. AC8 uses a fake
`ChangeSource` that counts subscribe and cancel calls.

**Edge Cases:**
- Duplicate targets: both indices are reported.
- A target that errors (unknown id): its error result is hashed; it is
reported when the entity appears.
- Entity deleted: `show_entity` turns into a not-found error, reported.
- Timeout 0: one check, returns immediately.
- A burst of 1000 writes: one replay pass per tick.
- A panicking replay: recovered; the target keeps its baseline.
- A replay over 5 s: not finished, keeps its digest; the pass continues.
- A cursor from another process key or principal: every target reported
changed once.

**Negative Tests:** unknown tool, write tool, `search_entities`, `watch` as a
target, 0 or 17 targets, timeout -1 or 51, malformed cursor (not base64, bad
JSON, `v: 2`, over 4 KiB), unknown argument inside a target's args.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Proxies may cut a long request on remote MCP. Mitigation: max 50 s and a
default of 30 s; rela-server has no write timeout.
- Cursors do not carry across restarts or instances: the cost is one
spurious "changed" per target, documented.
- Many watches multiply replays. Mitigation: caps and the shared semaphore.
- Lossy feed: a full subscriber buffer drops events, but a full buffer
means the watch is already dirty, so no change is missed.
- fsstore without the file watcher misses external edits. Stdio starts the
watcher; rela-server's data-entry app starts it.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `docs/mcp-server.md`: the `watch` tool, the cursor protocol (call
without a cursor, read, then wait), limits, and the ACL behaviour.
- [x] `docs/acl-security.md`: `watch` follows the live-view rules
(fixed clock, fresh access per pass, search excluded and why).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** security (rela-security-reviewer) and architecture
(go-architect) reviews of this plan. Significant RR-MA9F2E, RR-YGYXWM,
RR-XZWLF7; minor RR-WH0GPX, RR-3BNIXB, RR-86XC4V, RR-C8VT1L, RR-GNUUOC,
RR-WU343D, RR-2ACY57; nit RR-V5FXGI, RR-COLYCX. All addressed in this plan.
