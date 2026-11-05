---
id: PLAN-QB0QNEN
type: planning-checklist
title: 'Planning: Real-time co-editing of entity bodies (single node)'
started: "2026-11-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope (phase 1 of DEC-OHJEKG, single server process):

- WebSocket relay `/api/v1/_collab/{type}/{id}` on `reearth/ygo`
  `provider/websocket`, one room per entity address, registered on the outer
  mux next to SSE so the full middleware chain (auth, ACL request, same-origin,
  request stats) runs on the upgrade.
- Collab in the inline body editor on the entity detail page
  (`EntityBody.vue`). The body editor already works on a draft snapshot, so the
  shared document can own it.
- Presence and remote cursors through Yjs awareness.
- Build-version check on join; a stale SPA reloads once.
- y-prosemirror node-marks patch and GFM cell `alignment` default `null`.
- Corpus round-trip test in collab mode.
- rela-server only. Desktop and the `just dev` fs server without a build id
  still work; the desktop reports collab disabled.

Out of scope: the body field inside DynamicForm (create/edit page) and the
conversion modal, which stay non-collab; merge of external (non-collab) edits
into a live room beyond the existing autosave merge (phase 2); cross-node relay
(phase 3, TKT-WVSTXC); properties.

Deviations from the ticket text, with reasons:

1. **Deterministic seed instead of a server-assigned seeder.** The client that
   finds the synced room empty seeds it from a temporary Y.Doc whose clientID
   is derived from a hash of the stored markdown. Two clients that seed the
   same markdown produce byte-identical updates, and Yjs dedupes them. No
   custom protocol and no server seeder state. Residual race: two clients that
   loaded different stored bodies (a save landed between their GETs) both seed
   an empty room; the result is duplicated text that a user can delete. This
   needs two joiners within one save of each other on an empty room and is
   documented.
2. **One saver per room instead of every client autosaving.** Since
   TKT-2VDVHF every content save carries a precondition token. Two clients
   saving the same live document would 412 each other and diff3 would report a
   conflict whenever both edited the same line. Instead the writable peer with
   the lowest awareness clientID saves. Every saved body's hash is recorded in
   a shared Y.Map (`saved`). On a content 412 whose server body hash is in that
   map, the body came from this room and the live document already contains it,
   so the save resends ours against the fresh token instead of merging. Any
   other 412 (an external write) goes through the existing merge path; the
   merged result is applied to the editor, which propagates it to the room.
   This also covers handover: a new saver with a stale token hits a 412 whose
   body is in `saved`.
3. **The room carries its save base.** A shared Y.Map `base` holds the last
   saved body and its version token; the seeder writes the stored body and the
   GET token, each saver updates it after a save. Savers precondition on the
   room's token, not their own GET, so a store write from outside the room
   412s into the merge path instead of being overwritten (RR-QEQ6711).
4. **Fallback saver.** Any writable peer saves when the live document's hash
   is not in `saved` 5s after the saver's debounce would have fired, so a
   crashed or misbehaving saver cannot stop saves (RR-PB4EXEM).
5. **Room lifetime.** The relay keeps an empty room for 5 minutes
   (`collabRoomIdle`), so a reconnect after the 10 minute connection lifetime
   cap, a network blip or a slow leave detection finds its room. A client
   that hears nothing from the room for 3 minutes ends its session: it
   flushes and saves its pending change, closes the editor and says so. The
   next edit joins a fresh room, so stale and reseeded content never merge
   (RR-QEQ6711). The client gives up before the room is discarded.

**Acceptance Criteria:**

1. Two browsers on the same entity see each other's body edits within a second
   and see each other's cursor with the principal's name.
2. The body saved to the store equals the live document after the last edit
   settles; exactly one client sends the saves.
3. A principal without read access gets 404 on the WebSocket path; one with
   read but no update access joins read-only (sees edits, cannot change the
   document, server drops its updates).
4. Concurrent typing on the same line by two users never shows a conflict.
5. A client with a build id that differs from the server's is refused with
   close code 4409 and reloads once.
6. The full editor corpus round-trips identically in collab mode.
7. Desktop config reports `collab_enabled: false` and the body editor works as
   before.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-L4FVT0 (Option A), decision DEC-OHJEKG, spike TKT-T4ZC8X.

**Existing Solutions:**

- Client: `yjs`, `y-prosemirror`, `y-protocols`, `y-websocket` (provider with
  `disableBc`), `@milkdown/plugin-collab` pinned to `@milkdown/kit` 7.22.3.
- Server: `github.com/reearth/ygo` `provider/websocket`: `Authorize` returns a
  per-connection `ReadOnly`, room eviction when the last peer leaves
  (`RoomIdleTimeout: 0`), size caps, and close frames from a context cause
  implementing `CloseCode()`.
- Codebase patterns: SSE registration on the outer mux
  (`internal/dataentry/router.go`), `readAddressedOr404` for the uniform 404,
  `translateVerb` + `AuthorizeWrite` for update permission
  (`affordances.go`), `HistoryEnabled: a.versions != nil` for the config flag.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Server (`internal/dataentry/collab.go`):

- `collabRelay` wraps a `*websocket.Server` with limits (max peers per room,
  max rooms, message and update byte caps). `App.SetCollab(build string)`
  enables it; rela-server calls it, desktop does not. The relay is shut down
  when the app generation retires.
- Handler `GET /api/v1/_collab/{type}/{id}`: resolve the address with
  `readAddressedOr404` (404 when hidden or missing), compute writability with
  `AuthorizeWrite(translateVerb("update", ...))`, room name is the canonical
  `type/id@face` of the resolved row. The verdict is passed to `Authorize`
  through the request context. If the client's `build` query parameter differs
  from the server build (both non-empty), the context is cancelled with a
  cause carrying close code 4409. Every connection gets a lifetime cap (10
  minutes) through `context.WithTimeoutCause` with close code 4001, so a
  permission change applies on the provider's reconnect.
- `statsResponseWriter` gains `Hijack`; requestStats treats the collab path as
  a stream (no slow-request log).
- `_config` gains `collab_enabled`.
- Build id: Vite writes `build-id.txt` into `static/v2`; the server reads it
  from the embedded FS at startup. Empty in dev and tests: check skipped.

Frontend:

- `useCollab(type, id)` composable: Y.Doc, WebsocketProvider (`disableBc`,
  `params: {build}`), awareness user `{name, color}`, saver election (lowest
  writable clientID), `saved` Y.Map, close-code handling (4409 reloads once
  via sessionStorage; 4001 reconnects silently).
- `MilkdownEditor` `collab` prop: skips `history`, uses `collab` plugin,
  `ySyncPlugin`/`yCursorPlugin`/`yUndoPlugin` through CollabService, seeds
  deterministically after `synced`, toolbar undo/redo use y-undo.
- `EntityBody`/`EntityDetail`: when `collab_enabled`, open the editor in collab
  mode; only the saver schedules content saves; `useAutoSave` gets an
  `ownsContent(theirs)` hook consulted in `resolveConflicts` before `mergeText`.
- `patch-package` patch for y-prosemirror node marks; GFM cell schema
  extended with `alignment` default `null`.

Alternatives rejected: server-side markdown serialization (needs a Go
ProseMirror schema; DEC-OHJEKG); every client saving (412 storm, see
deviation 2); a custom seeder protocol (deviation 1).

**Files to modify:**

- `internal/dataentry/collab.go` (new), `collab_test.go` (new),
  `router.go`, `router_walk_test.go`, `requeststats.go`, `api_v1.go`
  (`_config`), `pkg/api/v1` config type, `cmd/rela-server/serve.go`
- `frontend/package.json`, `frontend/patches/y-prosemirror+1.3.7.patch`,
  `frontend/vite.config.ts`, `frontend/src/composables/useCollab.ts` (new),
  `useAutoSave.ts`, `components/forms/milkdown/MilkdownEditor.vue`,
  `editorPreset.ts`, `components/entity/EntityBody.vue`, `EntityDetail.vue`,
  corpus test
- `docs-project` deployment page: sticky routing note.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Path `{type}/{id}`: resolved through the visibility reader; anything not
  readable is the uniform 404 before the upgrade.
- `build` query parameter: compared for equality only, never echoed.
- WebSocket frames: bounded by ygo's message and update byte caps; peers and
  rooms capped. Read-only peers' updates are dropped by ygo.
- Origin: rela's `requireSameOrigin` already checks every sensitive `/api/`
  request including upgrades; ygo's own check is disabled with a comment.

**Security-Sensitive Operations:**

- The relay never writes to the store. Saves remain PATCHes through
  entitymanager with the saver's own ACL and audit.
- Room contents are only reachable by principals who passed the read gate at
  join; the lifetime cap bounds how long a revoked read stays connected.
- A writable peer can push content a read-only peer sees before any save; that
  is the same exposure as reading the saved body.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1/AC2/AC4: Playwright E2E with two pages on one entity: type in both,
  including the same line; assert both editors converge and the stored body
  equals the document; count PATCH requests per page.
- AC3: Go tests on the handler with a real ygo server and a gorilla client:
  hidden entity 404, read-only principal's update not applied, writable
  principal's update applied.
- AC5: Go test: mismatched build closes with 4409; empty server build skips.
- AC6: corpus test parameterised over collab mode.
- AC7: Go test on `_config` with and without collab.
- Unit: `useAutoSave` `ownsContent` path resends ours on 412; saver election.

**Edge Cases:**

- Empty body; room evicted and rejoined; saver leaves mid-debounce (flush on
  unmount); entity renamed or deleted while a room is open (next save 404s;
  room drains); same principal in two tabs.

**Negative Tests:**

- Unauthenticated upgrade rejected by the existing middleware; foreign Origin
  rejected; oversize update closes the peer.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Serializer drift between peers on different builds: build check.
- Double seed race: documented, needs near-simultaneous joins on an empty room
  across a save.
- y-prosemirror patch lost on upgrade: patch-package fails install when the
  patch no longer applies; upstream PR offered.
- Memory: rooms are evicted when empty; caps on rooms and peers.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- docs-project: data-entry editing page (co-editing behaviour) and deployment
  (sticky routing for postgres multi-node, desktop has no co-editing).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-PB4EXEM and RR-QEQ6711 (significant, addressed
by deviations 3 to 5), RR-F2XOSS0 and RR-OPGU3N7 (minor, accepted).
