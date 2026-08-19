---
id: RR-PQ5UN1
type: review-response
title: One *acl.Request per HTTP request is not sufficient for JSON-RPC batches; the planned -race test cannot detect the race
finding: acl.Request memoizes globals/globalsLoaded without synchronization (request.go:34-38, :56-61) and is documented as not goroutine-safe. attachACLRequest attaches one per HTTP request, but one MCP POST may carry a JSON-RPC batch whose handlers dispatch concurrently, sharing that Request. The plan's mitigation (-race with parallel calls) tests parallel HTTP requests, which each get their own Request and will pass — the race needs concurrent handlers within one request.
severity: significant
status: deferred
reason: >-
  Deferred out of TKT-BDG8U9, not fixed. acl.Request.Globals remains an
  unsynchronised read-modify-write and attachACLRequest still attaches one
  Request per HTTP request. Nothing in the shipped handler fans a JSON-RPC
  batch across goroutines, so the race is latent rather than live — but it
  constrains how batch dispatch may be implemented, which is exactly why it
  stays open and visible rather than being closed as "not reproducible". The
  fix is cheap when needed (ForPrincipal does no graph traffic, so a
  per-tool-call Request works); the test must exercise concurrency WITHIN one
  request, since parallel HTTP requests each get their own Request and would
  pass vacuously. Recorded in TKT-BDG8U9 and docs/server-security.md.
---

## Finding

The plan's edge-case list says: *"Concurrent requests: each gets its own
`*acl.Request` (not goroutine-safe by contract, `request.go:47`) — assert with
`-race` and parallel calls."* Necessary, but not sufficient, and the proposed
test cannot fail.

`acl.Request` memoizes `globals`/`globalsLoaded` with **no synchronization**
(`internal/acl/request.go:34-38`, `:56-61`); the contract is explicit that
methods are not safe for concurrent use and one Request should be opened per
*logical operation*.

`attachACLRequest` attaches one `*acl.Request` per **HTTP request**. A single
MCP HTTP POST is not necessarily one logical operation: JSON-RPC permits
batching, and the SDK may dispatch handlers concurrently within one
request/response stream. Two tool handlers sharing that ctx then race on
`globalsLoaded`.

**Why the planned test misses it:** "`-race` with parallel calls" means parallel
*HTTP requests*, and each of those gets its own `Request` — so the test passes
while the real race (concurrent handlers *within* one request) goes untested.

Note also `visibility.DeclarativeGate.Bind` short-circuits when a Request is
already attached (`adapters.go:37-39`), so inheriting the per-HTTP Request is
the behavior you get **by default**, without choosing it.

## Resolution required

Pick one and pin it:

- **(a)** Confirm the go-sdk dispatches strictly sequentially per HTTP request,
and add a test that fails if that ever changes; or
- **(b)** Have the MCP handler open a per-*tool-call* `acl.Request` rather than
inheriting the per-HTTP one.

The test must exercise concurrency **within one HTTP request** (a batch), not
across requests.
