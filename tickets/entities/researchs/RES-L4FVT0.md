---
id: RES-L4FVT0
type: research
title: Real-time multi-user editing of entity bodies with Milkdown
summary: Yjs via @milkdown/plugin-collab with a Go relay (ygo) and client-side saves; spike confirms interop and zero drift over 1,312 bodies after two y-prosemirror fixes
status: done
---

## Problem

Two people editing the same entity body today get silent last-write-wins: the
autosave sends the whole markdown body on every PATCH, without `If-Match`
(`useAutoSave.ts:430-470`). TKT-2VDVHF fixes the *lost update* with per-field
preconditions and a diff3 merge, and explicitly left real-time co-editing out of
scope. This research answers whether and how rela can use Milkdown's multi-user
editing support so that several people can type in one body at the same time,
with live cursors.

## Context

### What Milkdown offers

- `@milkdown/plugin-collab` (7.22.2, released in lockstep with Milkdown) is a thin
wrapper around y-prosemirror: `ySyncPlugin`, `yUndoPlugin` and `yCursorPlugin`.
It binds a Yjs document (a CRDT, a data structure that merges concurrent edits
without a central arbiter) to the editor. It ships **no transport and no
server**; the application brings a provider such as `y-websocket`.
- `applyTemplate(markdown)` seeds an empty shared document. Two clients that
both see an empty document before sync both seed it, and the CRDT keeps both
copies ("double content"). The maintainers' answer is that the server must own
seeding (Milkdown discussions #1993, #1756).
- Pin `yjs` 13.6.x and `y-prosemirror` 1.3.x. The y-prosemirror main branch has
moved to a Yjs v14 pre-release with renamed APIs.

### Server-side options for the Yjs protocol

- The y-websocket sync protocol is small: SyncStep1 (state vector), SyncStep2
(missing updates), Update, plus the awareness (presence) message. A read-only
peer is enforced by dropping its Step2/Update messages.
- Node servers (Hocuspocus) and Rust servers (y-sweet) exist; both break
rela's single-binary, no-Node-at-runtime deployment.
- Go: `reearth/ygo` is a pure-Go Yjs port (v1.50, MIT, fuzzed against yjs
13.6.30) with XmlFragment support, sync, awareness and a websocket server. It is
young (created April 2025, small user base). Other Go ports are immature.
- Converting the shared document back to markdown needs Milkdown's
remark-based serializer. On the server this means either a Go re-implementation
for rela's schema (drift risk against Milkdown's output) or an embedded JS
runtime (goja / QuickJS-on-wazero; unproven for this bundle).

### rela constraints found in the codebase

- **Markdown is the source of truth** and has many writers besides the editor:
Lua `update`, webhook `append_section`, history restore, CalDAV, the
rendered-view checkbox toggle (`EntityDetail.vue:463`), the app bridge.
- **Serialization is not byte-stable.** Milkdown reformats about 44% of bodies
(`writeBackGuard.ts` hides this until the user edits), and fsstore reflows every
body to 80 columns on write (`fsstore/markdown.go:181`). PostgreSQL stores raw
content.
- **No WebSocket exists.** Server push is SSE only (`/api/v1/_events`), and
frames carry the entity *type* only, by design (TKT-POT9GQ). `coder/websocket`
is already an indirect dependency. `WriteTimeout` is 0 for SSE; `ReadTimeout`
(30s) does not affect a hijacked connection, because `net/http` clears the
deadline on hijack.
- **Deployment tiers.** fs and sqlite are single-process. PostgreSQL supports
several `rela-server` nodes behind a load balancer, with a `LISTEN/NOTIFY`
change feed (`rela_changed`) that carries entity ids only. Desktop (Wails)
serves the SPA through an in-process handler, not a TCP listener, and is a
single local user.
- **ACL.** A principal can read but not update an entity. Body content is not
redacted per principal (`policyreader.go:215`, TODO). Write checks run in
`PatchEntity` via `authorizeAndAudit`.
- **Comments are anchored by quote** (text + prefix/suffix, re-located with
`textanchor`), not by offset, so concurrent edits do not break them.
- The editor already replaces its whole document when `modelValue` changes
from outside (`MilkdownEditor.vue:695-722`); with a live shared document that
path must be disabled.

## Options

### Option 0: No real-time editing; presence only

Ship TKT-2VDVHF (conflict-safe autosave), and add an "also editing: Alice"
indicator. Presence needs a per-entity heartbeat and a way to push it; SSE is
type-only on purpose, so it would need a small per-entity presence endpoint that
is gated by read access.

- Pros: smallest change; no new transport; users avoid collisions socially.
- Cons: no simultaneous typing, no live cursors.
- Effort: S on top of TKT-2VDVHF.

### Option A: Yjs with a Go relay; clients serialize and save (recommended)

The shared Yjs document is **ephemeral session state**, not stored. Markdown in
the store stays the only source of truth.

1. **Transport:** a WebSocket endpoint `/api/v1/_collab/{type}/{id}` speaking
the standard y-websocket protocol, so the off-the-shelf `y-websocket` client
provider works. It sits behind the existing middleware (Host, Origin, JWT or
principal header), which also stops cross-site WebSocket hijacking.
2. **Room:** one in-memory room per entity per process. The server relays
Update and awareness messages and replies to SyncStep1. It uses `ygo` to hold
the merged document so a joiner gets one compact state instead of a replay of
every update, and so the server can validate that updates parse. The server
never needs to understand markdown.
3. **Seeding:** the server marks the first joiner as the seeder. That client
parses the stored markdown with Milkdown and sends the initial state; other
joiners wait for `synced`. This removes the double-content race without a
server-side markdown parser.
4. **Saving:** every client keeps the existing autosave, now fed from the
shared document. Because every client runs the same serializer over the same
converged document, their PATCHes carry identical content and are idempotent.
The existing ACL, validation, audit, automations, versioning and search indexing
all keep working unchanged, because a save is still an ordinary PATCH. In collab
mode the editor ignores the server's echo of the content (fsstore reflow must
not re-seed the document).
5. **ACL:** joining requires read access (a hidden entity is a 404, as today).
A principal without update permission gets awareness only; the relay drops its
Step2/Update messages. Permission is re-checked on save by the normal PATCH
path.
6. **External edits** (Lua, webhook, restore, CalDAV) while a room is open: the
server sees the store event, compares the stored content hash with the last
content the room saved, and on a mismatch sends the new markdown to one client.
That client parses it and applies it with y-prosemirror's `updateYFragment` (a
tree diff, so cursors and concurrent edits survive). Collab saves carry a
content precondition so an in-flight autosave cannot overwrite the external edit
before it is merged.
7. **Multi-node (postgres):** two users on different nodes land in different
rooms. Phase 1 documents sticky routing by URL path for `/api/v1/_collab/`
(hash-by-URI in nginx/HAProxy). Phase 2 relays updates between nodes through a
`collab_updates` table plus `NOTIFY`, following the change-feed pattern (NOTIFY
payloads are capped at 8000 bytes, so the payload is a row id).
8. **Version skew:** a browser tab left open across a server upgrade runs the
old SPA against the new server, and two builds may serialize the same document
differently. The collab handshake carries the build version; the server refuses
a mismatched client and the SPA reloads. This applies to Option B as well.
9. **Desktop:** disabled. It is single-user, and WebSocket through the Wails
asset handler is unverified.

- Pros: standard client stack (plugin-collab, y-websocket, cursors via
awareness); no Node at runtime; no server-side markdown; every existing
write-path guarantee is kept because saves are still PATCHes; works offline
briefly and reconnects cleanly.
- Cons: if the last client closes before its debounced save flushes, the final
edits are lost (same risk as today; flush on unload mitigates). N clients send N
identical PATCHes per debounce window (idempotent; version history attributes
the version to the last saver). The external-edit merge depends on a connected
client. `ygo` is young; the relay can fall back to replaying stored updates
without it, because the server never needs to read the document.
- Effort: L overall. Relay and room (M), client wiring with seeding and
collab-mode autosave (M), external-edit merge (M), cross-node relay (M,
deferrable).

### Option B: Yjs with the Go server as the authority

As Option A, but the server holds the document and writes markdown itself, using
a Go serializer (and parser, for seeding and external edits) over `ygo` for
rela's ProseMirror schema.

- Pros: a save does not depend on any client; seeding and external edits are
handled on the server; attribution per update is possible.
- Cons: the server needs its own markdown parser (for seeding and external
edits) and serializer (for saving) covering CommonMark, GFM and rela's custom
nodes (entity refs, HTML comment chips, task lists). Because the SPA and the
server ship in one binary, the editor schema and the Go code are fixed together
at build time; a mismatch is an ordinary bug, caught in CI by a differential
test that runs a markdown corpus through Milkdown (Node is available at build
time) and through Go and compares the resulting ProseMirror trees and markdown.
Byte-level formatting differences only cause one-time reformatting churn, as
fsstore's reflow already does. The cost is implementation effort plus
maintaining that test, not a runtime risk.
- Effort: XL.

### Option C: prosemirror-collab (operational transformation)

The server is a version-checked, append-only log of ProseMirror steps (a step is
one JSON-encoded edit). It accepts a batch only if the client's version is
current; clients rebase and retry. Transport can be SSE down and POST up, which
reuses the existing middleware and needs no WebSocket.

- Pros: a very small Go authority; maps well to a postgres table with a
unique `(doc, version)` key for multi-node; no CRDT dependency.
- Cons: no presence or cursors (must be built); no offline editing; many
rejections under contention; the step log cannot be read on the server, so
saving is still client-side; external edits force a session reset (new base
document) instead of a merge; no official Milkdown integration.
- Effort: L, with more custom client code than Option A.

### Option D: Node sidecar (Hocuspocus) or hosted service (y-sweet, Liveblocks)

- Rejected: breaks the single-binary deployment, adds an operational component,
and a hosted service sends entity content to a third party.

## Recommendation

**Option A, delivered in phases, after TKT-2VDVHF.**

1. **Phase 0:** TKT-2VDVHF (conflict-safe autosave). It is needed anyway for
everyone not in a live session: CLI, Lua, and users whose tier has collab
disabled.
2. **Phase 1:** single-node collaborative editing of the body. WebSocket relay
with `ygo`, server-assigned seeder, client-side save, read-only enforcement,
presence and cursors. Covers fs, sqlite, and postgres behind sticky routing.
3. **Phase 2:** external-edit merge through `updateYFragment`.
4. **Phase 3:** cross-node relay over postgres.

Tradeoffs accepted: saving depends on connected clients; a small window where
edits exist only in the browser; `ygo` as a young dependency, behind an
interface narrow enough to swap for a plain relay.

Before committing to Phase 1, a one-day spike should confirm three unknowns: (1)
the standard `y-websocket` client interoperates with a `ygo`-backed room; (2)
two Milkdown instances produce identical markdown from the same converged
document, including rela's custom nodes; (3) a WebSocket upgrade passes through
the existing middleware chain (Origin check, JWT gate).

Out of scope throughout: properties (form fields) stay on the PATCH and
precondition path; only the markdown body is co-edited.

## Spike results (2026-09-25)

Artefacts: `.ignored/spike-collab/` (Go server, Node interop client, corpus
test, y-prosemirror patch). Nothing was committed.

### 1. Standard client against a `ygo` server: works

`y-websocket` 2.1 + `yjs` 13.6.33 against `ygo` v1.50.0 (`provider/websocket`).
All checks pass: seeding, late joiner receives state, concurrent edits converge,
the server's document matches the clients', awareness propagates and is removed
on disconnect, and a peer marked `ReadOnly` through `Server.Authorize` receives
state but its edits are dropped.

- `ygo/provider/websocket` does not link `modernc.org/sqlite` or Redis
(`go list -deps`), so the build-tag rules hold. It does pull in
`gorilla/websocket`.
- **The SPA must set `disableBc: true`** on the provider. Otherwise
`y-websocket` syncs tabs of the same browser directly over `BroadcastChannel`,
bypassing the server (and its read-only check).

### 2. Serializer agreement over the repo corpus: works, after two fixes

1,312 bodies (tickets, bugs, decisions, research, ideas, features, review
checklists, docs-project) through rela's editor preset (`RELA_COMMONMARK`, gfm,
`entityRefNode`, `RELA_OUTPUT_NODES`, `taskList`) with
`@milkdown/plugin-collab`:

| Check | Unfixed | With both fixes |
| --- | --- | --- |
| Two collaborating editors serialize differently | 0 | 0 |
| Concurrent edits (both ends) fail to converge | 0 | 0 |
| Collab output differs from the plain editor | 653 | 0 |

The 653 were semantic drift, which the write-back guard would refuse. Two
y-prosemirror 1.3.7 limitations cause it:

- **Marks on non-text inline nodes are dropped.** y-prosemirror stores marks
only as Y.Text formatting. An `entityRef` chip or a hard break inside bold loses
its bold (`**a `X-1` b**` becomes `**a** `X-1` **b**`). Fix: carry the node's
marks as a reserved attribute on the Y element (four small edits in
`sync-plugin.js`; patch saved). Ship as `patch-package` and offer it upstream.
- **`null` attributes are dropped and come back as the schema default.** GFM
cells parse with `alignment: null` but default to `'left'`, so `---` becomes
`:-`. Fix: extend `tableCellSchema`/`tableHeaderSchema` so the default is
`null`; no library change needed. Any future node with a non-null default and a
meaningful null has the same problem, so the corpus test must run in collab mode
in CI.

Also: `@milkdown/plugin-collab` pins `@milkdown/core`/`ctx` to its own **exact**
version. A patch-level mismatch with `@milkdown/kit` loads two cores and fails
every editor (`Timer "EditorViewReady" not found`). Pin both to the same
version.

### 3. WebSocket upgrade through the middleware chain: works, one change

- `requestStats` wraps every response in `statsResponseWriter`, which
exposes `Unwrap` but not `Hijack`. `gorilla/websocket` (used by `ygo`) asserts
`w.(http.Hijacker)` directly and fails the upgrade with a 500; reproduced in the
spike. `coder/websocket` follows `Unwrap` and would work. Fix: add a `Hijack`
forwarder, which the wrapper's own comment asks for.
- Host, Origin, JWT and principal middlewares run before the upgrade and
apply unchanged; browsers send `Origin` on WebSocket upgrades.
- The `acl.Request` attached per request lives as long as the connection.
Permission changes during a session need a periodic re-check or a disconnect on
policy reload.
