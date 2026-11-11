---
id: PLAN-JQPLCZM
type: planning-checklist
title: 'Planning: Attachments in channel messages'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

- A `file` message property type in `realtime.yaml`, with `max` (files per
message, default 1, at most 4), `max_size` (bytes per file, default and ceiling
10 MiB) and the entity file property's `accept`, `scan` and `transform` keys,
with the same semantics as on an entity property: `accept` replaces the global
allow list, the deny list stays absolute, `scan: off` opts out of the global
`scan_cmd`. All files of one message together are capped at 16 MiB.
- Bytes stored by each channel backend beside the message (filechannels: a
`files/` directory per channel; sqlchannels: a `channel_files` table), keyed by
channel, seq and a minted file id. The store contract gains the file lifecycle,
pinned in `channelstest.RunAll`.
- Post with files in one request: multipart POST on
`.../_channels/.../messages` (a JSON `message` part and file parts). The
JSON-only POST keeps working.
- Download: `GET .../messages/{seq}/files/{id}`, gated by the channel read
check (`Service.open`), with `setHardenedDownloadHeaders`.
- Remove, channel delete and purge erase the bytes with the content
(tombstone and rekey move them with the channel). Edit keeps the files.
- MCP `post_message` takes base64 files (16 MiB decoded total) and a new
`read_message_file` tool returns one (capped at `MaxInlineReadBytes`). CLI `rela
channel post --file prop=path` and `rela channel file get`.
- Refused files are audited as denied writes, as entity uploads are.
- Web: composer file picker for a `file` property; message shows file chips
with download; inline preview for png, jpeg, gif and webp only.

Out of scope (follow-ups):

- `promote_to.map` copying a message file into an entity `file` property
(refused at load, as outputs already are).
- Changing files on edit: an edit keeps the stored files as they are.
- Staged uploads (upload first, post later) and drag-and-drop paste.
- Per-channel byte quota beyond the per-message caps.
- Files on `ephemeral` message types (refused at load).

**Acceptance Criteria:**

1. A `file` property loads with `max`/`max_size`/`accept`; invalid keys
(`list`, `to`, `max_size` over the ceiling, `file` on an ephemeral type) are
refused at load with the property named.
2. A member posts a message with a PNG via multipart; readers see a file
entry (name, size, MIME, id) and download identical bytes with hardened headers.
3. A principal without channel read gets the same response as for reading
the channel (403 `read_denied`, or 404 for an unreadable anchor). A file of a
removed message, of a property no longer declared `file`, or an id not listed on
the message is 404.
4. A file whose sniffed MIME is outside `accept` (or the global allow), a file
over `max_size`, or more files than `max` is refused with 422/413 and nothing is
stored.
5. A configured scan that rejects refuses the post; a transform's output is
what is stored.
6. Removing the message, deleting the channel and purging a tombstone leave
no bytes in the live rows or files on fs, sqlite and postgres; rename keeps them
reachable. A JSON post that sends a value for a `file` property is refused.
7. MCP `post_message` and CLI `channel post --file` attach a file;
`read_message_file` and `channel file get` return it.
8. A refused file writes a denied-write audit record without file content.
9. The web composer uploads a file and the message shows a download chip and
an image preview.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the codebase research below settles the open options; the storage choice is documented under Approach)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: Slack/Matrix model files as message content with their own media store, which matches the chosen per-backend storage)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- No library needed: multipart parsing is stdlib, already used in
`internal/dataentry/handlers_attachment.go:264` (`readMultipartUpload`).
- Entity attachment pipeline: `internal/attachment/processor.go:31`
(`Processor`), `policy.go:27` (`PolicyProcessor`), `mimecheck.go:124`,
`command.go:42` (`applyTransforms`), `limiter.go` (`Limiter`).
`PolicyProcessor.Process` resolves an entity property and passes bytes through
unchecked when none matches (`policy.go:41-44`), so it cannot be reused as is
for channels.
- Entity blob storage `store.AttachmentManager` (`internal/store/store.go:644`)
is keyed by entity id and property, and attachment bytes live in the graph's
lifecycle (versioning, rename, delete). Channels are deliberately outside the
graph (`internal/channels/channels.go:1-37`).
- Message prop validation: `internal/realtimecfg/props.go:123`
(`Property.Normalize`), config validation `validate.go:511`.
- Content erasure points: `sqlchannels/sql.go:771` (Remove), `:928`
(`eraseRows`), `retention.go:340`; `filechannels/ops.go:354`, `:562`,
`retention.go:584`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Policy as a value. Extract from `internal/attachment` an exported
`FilePolicy{Accept, ScanCmd, Transform, MaxSize}` and a `CheckBytes(ctx,
FilePolicy, ProcessContext, io.Reader) ([]byte, Info, error)` that runs MIME
sniff, scan and transforms with no metamodel lookup and never passes through.
After transforms it re-sniffs the final bytes against `accept`, re-applies the
size cap and returns the final MIME and name. `PolicyProcessor` builds a
`FilePolicy` from the `PropertyDef` and calls it, so entity uploads keep their
behavior. The command runner moves to appbuild and is shared by dataentry and
realtime; without a runner a configured scan refuses every file. Scan
diagnostics (`HasConfiguredScan`, boot warnings) also read channel file
properties.
2. Config. `realtimecfg`: new `PropertyFile` type, raw keys `max`,
`max_size`, `accept`, `scan`, `transform`; `list`, `to`, `notify`, `max_items`
refused; refused on `ephemeral` types. Every type switch gets a `file` case
(`jsonschema.go:61` leaves it out of `props`, `MessageType.PredicateFields`
gives `when:` the file count, `props.go:266`, `buttons.go`, `template.go:413`,
`choices.go`, `reload.go:78`); templates, triggers, presets and `promote_to.map`
refuse it. `Property.Normalize` refuses any client value for a `file` property.
3. Store. `channels.Message` gains `Files []File{ID, Property, Name, MIME,
Size}`, stored in the row's JSON so list pages need no per-row lookup.
`NewMessage.Files []NewFile{Property, Name, MIME, Data []byte}`: the bytes are
already processed and capped. Caps at the store boundary: 4 files per message,
10 MiB per file, 16 MiB per message. `Store` gains `OpenFile(ctx, key, seq, id)
(io.ReadCloser, File, error)`, which serves only ids listed in the message's
`Files` and refuses a removed message. Edit never touches `Files`; Remove clears
it.
   - sqlchannels: table `channel_files(channel_id REFERENCES channels(id)
ON DELETE CASCADE, seq, file_id, data)`, written in the Append transaction;
Remove deletes the seq's rows in its transaction; `eraseRows`, Rekey, Tombstone
and Purge work through `channel_id`. Added to `Raw`. DDL in
`sqlitedb/sqlitedb.go` and a new pg migration.
   - filechannels: the service-side bytes are written to a staging dir
outside the project lock; under the lock Append removes any stray
`files/<seq>/`, renames the staged files in, then writes the log line. Remove
moves `files/<seq>` to trash and deletes it before the segment rewrite, so a
crash fails closed (a reference without bytes reads 404). Staging leftovers
older than an hour are removed on the next Append.
4. Service. `realtime.PostInput.Files`; `checkMessageKeeping` checks count
and caps per property and runs each file through a consumer-side `fileChecker`
(one method, `CheckBytes`) supplied at wiring. A refused file is audited as a
denied write in the global audit log (channel key, property, display name,
reason). `Service.OpenFile(ctx, addr, seq, id)` goes through `open` and refuses
a property no longer declared `file`. Posts with files use a second, lower rate
bucket. `MessageView.Files` renders the stored entries without the config.
5. HTTP. `channels_handler.post` accepts `multipart/form-data`. It takes an
`attachment.Limiter` slot before reading the body, reads parts in order with
`multipart.Reader`, requires the `message` JSON part first, runs the read,
post-role and rate checks, then spools each `file:<prop>` part to disk under a
running 16 MiB total cap. Unknown parts are refused. New `GET
.../messages/{seq}/files/{id}` with `setHardenedDownloadHeaders`; the id is
checked against the minted-id grammar first.
6. MCP/CLI. `post_message` gains `files: [{property, name, content}]`
(base64, 16 MiB decoded total, shared Limiter); new `read_message_file`. CLI
`--file prop=path` (repeatable) and `rela channel file get`.
7. Frontend. `ChannelPropertyKind` `file`, a file widget in
`RlChannelComposer` using `RlFileField`; `RlChannelMessage` renders chips and
`<img>` previews for png, jpeg, gif, webp; `api/channels.ts` sends multipart
when files are present.

**Alternatives rejected:**

- Reuse `store.AttachmentManager` on the anchor entity: ties chat bytes to
graph versioning and rename, and breaks "remove erases content".
- Shared blob seam (`state.KV` or a new blob store): a second store whose
erasure must stay in sync with each channel backend's transaction; a crash
between the two leaves either orphans or dangling refs.
- Two-step staged upload: needs an orphan GC and an upload-token ACL; one
request is enough for chat-sized files.

**Files to modify:**

- `internal/attachment/{policy.go,mimecheck.go,command.go}` (+ new `filepolicy.go`)
- `internal/metamodel/attachments.go`
- `internal/realtimecfg/{raw.go,compiled.go,validate.go,props.go,jsonschema.go,programs.go,buttons.go,template.go,choices.go,reload.go}`
- `internal/realtimelimits`
- `internal/sqlitedb/sqlitedb.go`, `internal/store/pgstore/migrations/` (new
`channel_files` migration), `internal/cli/db_importfs_sqlite_test.go`
- `internal/dataentry/{app.go,handlers_attachment.go}` (runner moved, scan
diagnostics), `internal/audit`
- `internal/channels/{channels.go,store.go}`, `channelstest/`,
`filechannels/{ops.go,retention.go}`, `sqlchannels/{sql.go,retention.go,schema}`
- `internal/realtime/{service.go,ops.go,validate.go,read.go,render.go}`
- `internal/dataentry/channels_handler.go`, `internal/apiwire/v1/channels.go`
- `internal/mcp/tools_channels.go`, `internal/cli/channel.go`; plimsoll
directives on the store types (35 to 36 exported methods)
- `internal/appbuild/realtime_service.go` (wiring the checker)
- `frontend/packages/rela-components/src/components/channel/*`,
`frontend/src/api/channels.ts`, `frontend/src/utils/channelValues.ts`,
`frontend/src/types/channels.ts`
- `docs-project` entities for realtime and attachment security

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- File bytes: sniffed MIME checked against `accept` or the global allow
(allowlist); size capped while streaming (`store.CapAttachmentReader`); scan and
transform run confined through `cmdexec`.
- File name: `attachment.DisplayName` and `capNameLen`; on download
`safeAttachmentFilename`. The name is display only; storage uses the minted id,
so no path is built from user input.
- File id in the download URL: validated against `[a-z0-9]{n}` before any
lookup.
- Multipart body: `http.MaxBytesReader`, part count cap, unknown parts refused.

**Security-Sensitive Operations:**

- Download: read check through `Service.open`; denied and missing give the
same 404 as a hidden channel. Hardened headers (`nosniff`, CSP sandbox,
`attachment` disposition); image preview served the same way.
- Erasure: removal erases bytes in the same step as content, so a removed
secret cannot be fetched afterwards.
- Upload concurrency: shared `attachment.Limiter`.
- Scan/transform are on the write path today for entity uploads; channels
follow the same synchronous model (no AI, no jobs needed).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. AC1: table test in `realtimecfg` for valid and refused `file` configs.
2. AC2/AC3: `dataentry` handler test: multipart post, GET file with headers;
non-reader gets 404.
3. AC4/AC5: `realtime` service test with a fake runner (reject, transform);
handler maps to 422/413; nothing appended.
4. AC6: `channelstest.RunAll` cases for append-with-files, OpenFile, Remove,
Delete, Tombstone+Purge, Rekey; runs for file, sqlite and postgres.
5. AC7: MCP tool test and CLI test posting a file.
6. AC8: Vitest for composer/message components; one Playwright e2e posting an
image and downloading it.
7. Attachment refactor: existing `internal/attachment` tests stay green;
a test pins that `CheckBytes` never passes through.

**Edge Cases:**

- Zero-byte file (refused), exactly `max_size` (accepted), one byte over;
three files that pass `max_size` but exceed 16 MiB together (refused).
- A transform whose output is outside `accept` or over the cap (refused).
- A JSON post with a forged `file` value (refused); an edit cannot add,
change or drop files.
- A reload that changes a `file` property to `string`: stored files still
render from `Message.Files`, download is refused.
- Unicode, control characters, `../` and very long names.
- Same name twice in one message (suffixed, as entity uploads do).
- Reply with a file; edit of a message with files keeps them.
- Concurrent posts with files to one channel (seq stays gapless).
- File backend crash after the files are renamed in but before the log
line: the next append at that seq does not inherit them. Crash during Remove:
the message never keeps readable bytes.

**Negative Tests:**

- Files for a property not declared `file`, or on a type without one: 400.
- Multipart without the `message` part, or with an unknown part: 400.
- Download of a removed message's file, a wrong id, another channel's id: 404.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Store contract change across three backends: mitigated by conformance tests
in `channelstest` first.
- Postgres DB size growth from BYTEA: capped per message; a per-channel quota
is a follow-up. Deleted bytes stay in dead tuples or free pages until vacuum, as
for message text today.
- Memory pressure from uploads: Limiter slot before the body, parts spooled
to disk, 16 MiB total per message.
- Attachment refactor could change entity upload behavior: keep entity tests
untouched and green.
- Large ticket: implement as separate PRs (policy refactor + config; store;
service + HTTP + MCP/CLI; frontend + docs).

Effort: xl.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- `docs-project` realtime entity (Message types, HTTP API, Agents): the `file`
type, multipart post, download route.
- `docs-project` attachment security entity: channels use the same policy.
- `docs/cli-reference.md` via the CLI help: `channel post --file`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-DQWP3T4, RR-B3BW79V, RR-K09BZ2V, RR-DI9T66A,
RR-NOCSW3T, RR-YLUPO3O, RR-NXYFUR5, RR-XCTD6X9, RR-5DUDWNZ (significant,
addressed in the plan); RR-V5EDSSH, RR-ZYNU642, RR-1S59FXJ and the remaining
minor and nit findings (addressed in the plan).
