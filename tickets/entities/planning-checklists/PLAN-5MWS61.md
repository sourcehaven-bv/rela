---
id: PLAN-5MWS61
type: planning-checklist
title: 'Planning: Lua rela.attach_render: render, convert and attach'
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

- Lua binding `rela.attach_render{entity, property, document|list, entry, format, filename}`
returning `att, err` with `att = {name, size, sha256, mime}`. Errors are
returned as `nil, err`; only argument misuse (wrong Lua types) raises.
- Works where a principal deliberately runs a script and the data-entry
app is wired: data-entry actions, background automations and schedules of
rela-server and rela-desktop. On the other write runtimes that run schedules or
background automations (`rela scheduler`, CLI-triggered background automations)
the binding is present but returns `nil, "attach_render is not available in this
runtime"`, so one `schedules.yaml` fails clearly instead of with a nil call.
Absent from document renders, validation and the synchronous automation cascade
(reader runtimes or no write deps).
- Render as the calling principal through the SAME gate chain the export
routes run (`resolveStandaloneDocument`, `resolveAnchoredDocument`, the list
export pipeline), refactored to return a typed error instead of writing an HTTP
response, as `visibleReader.writeTarget` already does for `writeTargetOr404`.
- Convert with the App's single shared transform engine (cmdexec pool), or
`format = "md"` for the markdown itself.
- Attach through `attachment.Service.WriteAttachment` after the same
preflight the upload route runs (gated write target, declared non-hidden file
property, not locked, `update` ACL with audit on deny, `validateFieldWrite`).
MIME allowlist, scan and transforms apply as for an upload.
- SHA-256 for every attachment: computed by each store backend while it
writes the bytes, persisted beside them, returned in
`store.AttachmentInfo.SHA256`, the API `Attachment.sha256`, MCP
`list_attachments`, and shown in the SPA attachment list. Closes G13. Existing
attachments: PostgreSQL uses a generated column `sha256 TEXT GENERATED ALWAYS AS
(encode(sha256(bytes), 'hex')) STORED` (PostgreSQL 11+), which covers every
writer including the upsert; SQLite by a Go migration step (read rowids, then
hash and update, in the one migration transaction); fsstore hashes on write from
the stream and, for files it did not write, lazily on the first list, cached in
the index keyed on size and mtime and re-checked by stat on each list; memstore
on write.
- The audit record of the attachment write made by `attach_render` carries
the sha256 and the render source (document or list, entry, format). The stored
column is for display and download checks; the audit entry is the record that
cannot be rewritten together with the bytes.
- Docs: Lua scripting guide, transforms guide, attachment security guide.

Out of scope:

- An async/job variant (see Approach, jobs decision).
- Rendering single-entity exports and pile exports (the brief names documents
and lists only).
- The CLI `rela run` and stdio MCP runtimes.

**Acceptance Criteria:**

1. An action on an `audit` entity attaches a PDF of a document. Test:
dataentry integration test runs a real action script that calls
`rela.attach_render` with a `pdf` transform and asserts the file is on the
property and listed in `_attachments`.
2. The recorded hash matches the bytes. Test: the binding's `sha256`, the
API `_attachments[].sha256` and `sha256(download bytes)` agree; storetest
asserts every backend returns the SHA-256 of what was written.
3. A principal without read access to some rows gets a PDF without them.
Test: list and document renders as a principal whose grant hides one row; the
attached bytes do not contain that row's title.
4. A principal without write access to the property is refused. Test: the
binding returns `nil, err` (permission), nothing is stored, and an audit
`attachment-write` denial is recorded; same for a `fields:` freeze.
5. Works on fs, SQLite and PostgreSQL. Test: storetest attachment hash cases
on all backends; the dataentry test is backend-agnostic (fixture store);
appbuild-level test on sqlite and postgres tags exercises migration backfill.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (the brief is the approved design; no open choice of
approach beyond those recorded below)

**Existing Solutions:**

- `crypto/sha256` from the standard library; no dependency.
- Render gate chains: `internal/dataentry/export_document.go`
(`resolveStandaloneDocument`, `resolveAnchoredDocument`) and `export_list.go`
(`handleV1ExportList`). Transform engine: `exportHandler.engine` built once in
`newExportHandler`.
- Attachment write: `handlers_attachment.go` `attachmentWritePreflight`,
`attachmentService`, `aclAttachmentAuthorizer`; `internal/attachment`
`Service.WriteAttachment`.
- Transport-free core beside an HTTP writer: `visibleReader.writeTarget`
next to `writeTargetOr404`.
- Optional write binding wired per runtime: `lua.WriteDeps.VersionTags`,
set by `dataentry.SetVersionTags` and `appbuild.luaWriteDepsFor`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Store hash. `store.AttachmentInfo` gains `SHA256 string`:
- pgstore: migration adds the generated column; list selects it.
- sqlitestore/sqlitedb: schema version bump, `sha256 TEXT NOT NULL DEFAULT ''`
column, Go step backfills existing rows; insert and upsert set it.
- fsstore: hash on write (TeeReader); lazy hash on list for unhashed or
changed files (size+mtime key). Index load stays stat-only.
- memstore and both tx wrappers: hash on write.
- storetest: a conformance case asserts the hash equals sha256 of the bytes,
survives rename, and changes on replace.
2. API and UI. `v1.Attachment.SHA256 string json:"sha256,omitempty"`, set
from `info.SHA256`; `AttachmentInfo.sha256?` in `frontend/src/types/entity.ts`;
the attachment list shows a shortened hash with the full value in a title and
copy affordance; MCP `list_attachments` adds `sha256`. Wire golden files
updated.
3. Transport-free gate chain in dataentry. A `requestDenial` error type
(status, code, title, detail) is returned by new cores for the standalone and
anchored document gates, and the list export pipeline; the HTTP functions keep
their names and only write the denial. The upload route's full sequence (upload
slot, configured size cap, preflight, write, rejection audit) moves into one
core that the PUT route and the binding share. One chain, two transports.
4. Renderer. A separate `attachRenderer` type (App is at its plimsoll cap)
in `internal/dataentry/attach_render.go`, built by a constructor that rejects
nil collaborators. Per call:
- `ctx = withACLEnforced(ctx)` and `attachBoundReadGate(ctx, acl)`, failing
closed; idempotent on the action path. Without this, background paths would see
the allow-all read gate.
- Bind the declared default world when the context has none.
- Validate inputs (document xor list; names must be configured keys; format
a registered transform name, or `md` when no transform has that name; filename
non-empty; its extension must match the transform's output type or is appended
when absent).
- Resolve the target: stored type through the gated reader, then the
write target and preflight.
- Document: gate chain as the export routes, but an elevated document
(`AllowACLBypass`) is refused. List: query built by `navItemsListQuery`, type
from the list config, unknown list id is an error.
- Convert with the shared engine, write through the shared upload core,
take size and hash from `attachment.Result`, which the service fills from the
bytes it hands to the store (no read-back that could race a replace), audit with
sha256 and source.
5. Lua. `lua.AttachRenderer` interface (one method, consumer side) and
`WriteDeps.AttachRender`; the binding uses the runtime context (Lua deadline).
Not on the `bypass_acl` handle: it always acts as the context principal. Returns
`att` table or `nil, msg`.
6. Wiring. Data-entry actions: set from the attachRenderer. Scheduler and
background automations: an atomic holder created at assembly and always passed
by `luaWriteDepsFor`; `dataentrywire.Services` stores the renderer in it. Before
it is set (or where it never is) the binding returns the "not available" error.
No race with automation workers started earlier.

**Jobs decision (sync).** The binding runs inline. `jobs.Queue` is for external
side effects (mail, HTTP, AI) whose failure or latency must not hold a write
path. Here the transform is a local confined process through the same bounded
`cmdexec` pool interactive exports and attachment scans already use inline, and
the attachment is a store write through entitymanager. Scripts that call it run
in actions, schedules and background automation jobs, none inside
`store.Store.Tx`. Background automations already run off the write path. Running
it inline is what lets the binding return `att, err` to the script, which the
brief requires. Its time limit is the script's deadline (30 s for an action);
the guide points long renders at schedules and background automations. A loop
that renders many entities shares the transform pool with interactive exports;
documented.

**Loop hazard.** Attaching to the trigger entity is a save and re-enqueues an
on-save background automation. The guide documents this and shows a `when:`
guard; a test pins one render with the guard.

**No new capability.** Calling a write binding is already the action author's
choice, and transforms are operator-registered. No `capabilities:` grant is
added.

**Alternatives rejected:**

- Replaying the HTTP handler with `httptest` in-process: avoids the refactor
but routes a script through HTTP plumbing and leaks status-code semantics.
- Hash only for this binding (stored in a property or the audit): two
sources of truth; the per-backend cost of a column is small.
- Hash computed in `attachment.Service`: the hash would not cover bytes
written by other paths (sync, `rela db load`), and it would not be the stored
bytes the acceptance names.

**Files to modify:**

- internal/store/store.go, internal/store/{pgstore,sqlitestore,fsstore,memstore}/attachment*.go,
pgstore migration, internal/sqlitedb (schema, migrate), internal/store/storetest
- internal/apiwire/v1/responses.go, internal/dataentry/affordances.go, wire goldens
- internal/mcp/tools_attachment.go and golden
- frontend/src/types/entity.ts and the attachment list component
- internal/dataentry/export_document.go, export_list.go, handlers_attachment.go,
standalone_document_handler.go, new attach_render.go, actions_ops.go, app.go
- internal/lua/deps.go, runtime.go (registration), new attachrender.go
- internal/appbuild (setter, luaWriteDepsFor), internal/dataentrywire/wire.go
- docs-project guides: Lua scripting, transforms, attachment security

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- All arguments come from operator-authored Lua, but values may derive from
entity data (for example a filename from a title). Document and list names must
be configured keys (allowlist) and pass `isSafePathSegment`; entry and entity
are addresses parsed by `entity.ParseRef`/`ParseAddress`; format must be a
registered transform name or `md`; filename goes through the attachment
service's name normalisation as an upload's does. Unknown keys raise.

**Security-Sensitive Operations:**

- Render: same gate chain as the export routes, including `permission:` and
the elevated-document switch, as the bound principal; hidden rows and fields
never reach the renderer.
- External command: only a registered transform via the shared engine
(`cmdexec` confinement, pool, timeout, cap). No command, flag or path from the
script.
- Write: the upload route's preflight plus the attachment service's
in-lock re-authorization; audit on deny; MIME allowlist and scan run.
- Errors: denial errors name the document, permission or property (config,
not secret); a hidden or missing entity gives one uniform not-found message;
render, transform and store failures are logged and returned generically.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- Unit (lua): argument parsing table tests with a fake renderer; nil deps
means no binding.
- Integration (dataentry): action script attaches standalone document,
anchored document and list renders; checks bytes, sha256, API listing, audit
records; ACL scenarios for hidden rows, hidden entity, write denial, field
freeze, permission-gated and elevated (refused) documents.
- The same hidden-row and permission scenarios run without an HTTP request
context (as the scheduler and automation jobs call it), plus a faced target type
there, a call inside `rela.bypass_acl`, an expired deadline mid-transform
(nothing stored), and an on-save background automation with a `when:` guard (one
render).
- storetest: hash on write, replace, rename, list, all backends.
- Migration: sqlite old-version database backfills hashes; pg migration
backfill checked in the pgstore suite.
- Manual: rela-server with a pandoc `pdf` transform, action on an audit
entity, inspect the PDF and the hash in the UI.

**Edge Cases:**

- Both or neither of document/list; entry on a standalone document or
missing on an anchored one (the existing kind-mismatch denials).
- `format` unknown, empty, or `md`.
- Filename empty, with path separators, or at `max` capacity (ErrAtCapacity).
- Target entity hidden, absent, locked, or a bare id on a faced type.
- Render longer than the cmdexec timeout or larger than the cap.
- Upload bigger than the configured attachment limit.

**Negative Tests:**

- Unknown option key raises; wrong types raise.
- Permission-denied document returns `nil, err` naming the permission.
- No update grant: `nil, err`, nothing stored, audit denial recorded.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Refactoring the gate chains could change HTTP behaviour. Mitigation: HTTP
wrappers keep names and messages; the existing export and document tests pin
them.
- Migration numbering collides with concurrent branches. Mitigation: rebase
before PR; pg migrations are numbered, sqlite version is a constant.
- Large attachment tables. pg: adding the generated column rewrites the
table once at upgrade; stated in the release note. sqlite: one migration step,
memory bounded by hashing row by row.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- GUIDE-lua-scripting (binding), GUIDE-transforms (scripted export),
attachment security guide (hash), data-entry API reference (sha256 field).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

cranky-code-reviewer reviewed the plan: 1 critical (background paths saw the
allow-all read gate), 11 significant (world binding, list query, elevated
documents, plimsoll, wiring race, fs hashing cost, audit evidence, loop,
deadline, pg backfill, upload preflight parts), minors and nits. All folded into
the sections above and recorded as review responses on TKT-1TS55R.
