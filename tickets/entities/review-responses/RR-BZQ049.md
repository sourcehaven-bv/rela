---
id: RR-BZQ049
type: review-response
title: Provision/reject seam misses sync, Lua-action, attachment, git-sync writes — reject is bypassable
finding: 'The plan hooks provisionIfUnmatched at the top of each CRUD write handler (write_handler.go). But an unmatched verified JWT principal reaches four OTHER data-entry write paths, all under /api/ (past the JWT gate + attachACLRequest), all taking writeMu: the sync API (sync_handlers.go:98,134 ApplyEntity, authorizes against the raw sub), the Lua-action endpoint (actions.go:80, /api/v1/_action/, runs operator Lua that writes under the unmatched principal), attachment writes (handlers_attachment.go), and git-sync (handlers_git.go). Under `reject`, a graph-is-authority deployment is trivially bypassed by POSTing to /api/v1/_action/<any> or /api/sync/... — the mutation is not rejected. Under `provision`, no stub is created there. Directly breaks AC2 (unmatched write gets 403) and AC8. The CRUD handlers are a SUBSET of the data-entry write surface, not the whole of it. Verified: sync_handlers.go:98/134 and actions.go:80 both take writeMu and write under principal.From(ctx) with no provision/reject check.'
severity: critical
resolution: 'Resolved for reject (provision half stays parked). Every data-entry write — CRUD, sync (ApplyEntity), Lua-action — funnels through entitymanager -> acl.ACL.AuthorizeWrite (manager.go authorizeAndAudit), the ONE shared authorization choke point. So reject enforces there and catches all write paths, no bypass. The transport-agnostic problem (entitymanager takes acl.ACL, can''t tell JWT from header; RR-9THKDO) is solved by carrying the fact, not the transport: the dataentry middleware — which knows a.jwtGate != nil AND that resolvePrincipalEntity found no entity — marks an ''unmatched verified'' flag on ctx (a typed context value in internal/acl or principal). AuthorizeWrite reads that flag + policy.UnmatchedPrincipal and denies when reject. Header/CLI/scheduler never set the flag -> untouched (AC4). No writeMu, no manager-learns-transport, no per-handler drift. Attachment/git writes: attachments go through entitymanager too (verify in impl); git-sync is a git commit not an entity write — confirm whether it needs the gate or is out of the entity-ACL surface. Pinned by a test that sync + action writes are rejected, not just CRUD.'
status: addressed
---

## Finding

`provisionIfUnmatched` at the top of each CRUD handler misses every other
data-entry write path. Verified as writes reachable by an unmatched verified
principal, all under `/api/`, all under `writeMu`:

- **Sync API** — `sync_handlers.go:98,134` (`ApplyEntity`), authorizes against
the raw sub.
- **Lua actions** — `actions.go:80` (`/api/v1/_action/`), runs operator Lua that
writes under the unmatched principal.
- **Attachments** — `handlers_attachment.go` (writeMu, writes under `/api/`).
- **Git sync** — `handlers_git.go`.

Under `reject`, POSTing to `/api/v1/_action/<any-action>` or `/api/sync/...`
mutates without being rejected — a **`reject`-bypass**. Under `provision`, no
stub is created on those paths. Breaks AC2 and AC8.

## Resolution

The hook must sit at a **choke point every write shares**, not per-handler.
Candidates: a write-method-gated branch inside `attachACLRequest` (it already
runs on every `/api/` request and already 500s on `ErrUnstampedPrincipal`), or a
middleware wrapping the whole API surface. If it must stay per-handler,
enumerate + cover sync/action/attachment/git AND add a router-walk drift test
asserting every write route runs the hook — else the next write handler silently
reopens the hole.
