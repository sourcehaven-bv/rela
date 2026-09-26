---
id: PLAN-DHSSFX
type: planning-checklist
title: 'Planning: MCP attachment tool: attach/list/read for agents, ACL-gated'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: four MCP tools over the existing attachment service, on both
transports (stdio `rela mcp` and remote `rela-server -mcp`):

- `list_attachments(id)`: files on an entity, per property.
- `read_attachment(id, property, file_name)`: file bytes, inline.
- `attach_file(id, property, file_name, content)`: upload; `content` is base64 (user decision: base64 only, no local `path`).
- `delete_attachment(id, property, file_name?)`: detach; `file_name` optional when the property holds one file (same rule as `rela detach`).

Out of scope: an MCP resource template for attachments (`resources/read`), a
REST download URL in results, a local-path upload input, streaming/chunked
transfer, changes to the attachment storage model.

**Acceptance Criteria:**
1. An agent can list, read, attach and delete attachments through MCP on both transports.
2. Reads are gated like the web path: hidden entity, hidden face, or hidden (`visible:`-redacted) file property gives the same "not found" as a nonexistent one.
3. Writes (attach, delete) are authorized for `update` on the owning entity BEFORE any bytes are written; a deny is audited as `denied-write` (`op=attachment-write`) and nothing reaches the store.
4. Uploads apply the same policy as the web path: MIME allowlist, scan/transform (runner-dependent), per-property `max` (append/suffix/at-capacity), and the size limit. On the remote transport the limit is the operator's configured `max_attachment_bytes`.
5. `read_attachment` returns `image/*` as image content, UTF-8 `text/*` as text, anything else as an embedded base64 blob. A file above the 10 MiB inline cap returns an error naming its size and the cap.
6. Invalid input (bad base64, non-file property, unknown property, empty file name) returns a tool error, never a panic or a partial write.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: FEAT-870YCY already has RES-INM8JP; the approach reuses existing services)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-INM8JP (on FEAT-870YCY)

**Existing Solutions:**
- `internal/attachment/attachment.go`: `Service.WriteAttachment` (cap/suffix/processor/write-then-delete ordering), `Service.Detach`, `Service.List`. Reused as-is; add entity-id entry points.
- `internal/dataentry/handlers_attachment.go`: the web read path (`handleV1GetAttachment`) and write preflight (`attachmentWritePreflight`: read gate, file-property check, hidden-property 404, locked check, up-front `AuthorizeWrite`, denied-write audit). The MCP handlers mirror this order.
- `internal/visibility/policyreader.go`: the gated `GetEntity` MCP already uses does row + face gating and field redaction, and records withheld names in `entity.Redacted`. That gives the hidden-property check without a new dependency.
- go-sdk v1.7 content types: `ImageContent`, `TextContent`, `EmbeddedResource{ResourceContents{Blob}}`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `internal/attachment`: add two entity-id entry points so callers holding only a gated read never pass an entity into a whole-entity save.
   - `Put(ctx, entityID, property, fileName, r) (*Result, error)`: loads the RAW entity (write-prep read), validates the file property, calls `WriteAttachment`. `Attach` becomes "open file, call `Put`".
   - `Open(ctx, entityID, property, fileName) (io.ReadCloser, error)`: `store.ReadAttachment`.
   - `Detach` and `List` already take an entity id.
2. `internal/mcp`: new `attachmentHandler` group (own file `tools_attachment.go`), registered like the other groups.
   - Consumer-side interfaces in `internal/mcp`: `AttachmentFiles` (List/Open/Put/Detach, satisfied by `*attachment.Service`) and `WriteAuthorizer` (`AuthorizeWrite`, satisfied by `acl.ACL`).
   - New `Deps.Attachments AttachmentDeps{Files, Authorizer, Audit, MaxBytes}`; required by `Deps.validate`.
   - Every handler first reads the entity through the gated `Deps.Store.GetEntity`, then checks the property is a declared `file` property and not in `e.Redacted`. Any failure gives one uniform "entity not found" error.
   - Writes then call `Authorizer.AuthorizeWrite(update, type, id, face)`; on deny, record `audit.OpDeniedWrite` and return the forbidden error. Then decode base64 (bounded by `MaxBytes`), wrap in `store.CapAttachmentReader`, call `Put`.
   - `read_attachment` checks size from `List` before reading and reads at most cap+1 bytes.
3. Wiring:
   - `internal/cli/mcp_wiring.go` (stdio): builds `attachment.Service` from the raw store with `NewPolicyProcessor(meta, nil)` exactly like `cli_wiring.go`; authorizer = `svc.ACL()` (NopACL); limit = `store.MaxAttachmentBytes`. Rebuilt on schema reload with the other deps.
   - `cmd/rela-server/mcp.go` (remote): `MCPHandlerFactory` gains an `MCPHostDeps` argument carrying the App's attachment command runner and effective `max_attachment_bytes`, so the remote path shares the web path's runner pool and limit without adding a method to `App`.
   - `internal/dataentry/mcp_http.go`: wrap the MCP route in `http.MaxBytesReader` sized for a base64 upload at the limit plus envelope headroom. The route is unbounded today; base64 upload makes large bodies legitimate, so it needs a ceiling.

**Alternatives rejected:**
- Pass `store.Store` into MCP Deps: undoes the narrow `GraphReader` that makes ungated raw reads unavailable.
- Have MCP pass the gated entity into `WriteAttachment`: that path does a whole-entity `UpdateEntity`, so a redacted entity would erase hidden fields.
- Rely on entitymanager's own ACL check inside `UpdateEntity`: it runs AFTER the bytes land, so a denied caller could overwrite an existing same-name file at `max: 1`.
- REST URL / local path inputs: declined by the user.

**Revised after design review (supersedes the points above where they differ):**
- Attachment stamping uses `PatchEntity` naming only the file property (RR-7I4002). `attachment.EntityUpdater` becomes `EntityPatcher`; `WriteAttachment` reads only the entity's id/type/face, so MCP passes the gated entity directly. No `Put` and no raw re-read, which removes the gated-vs-raw race (RR-KPQD8A). `Result.Entity` carries the post-write entity for the web response. Side effect: attachment writes now also pass the manager's field-level write gate on every ingress.
- `Detach` splits into a raw-loading wrapper plus `DetachFile(ctx, e, propDef, property, fileName)`: a named file deletes idempotently, and an unnamed delete requires exactly one file (RR-QZ98EZ).
- Denied-write and rejected-upload audit records are built by `audit.AttachmentWriteDenied` / `audit.AttachmentRejected`, used by both web and MCP (RR-CN70H8, RR-ZARZTR).
- MCP `AttachmentDeps{Snapshot, Authorizer, Audit, WriteLock}`. `Snapshot()` is called once per tool call and returns `{Meta, Service, MaxUploadBytes}` from one metamodel snapshot. Remote builds it from the App's live schema, so policy edits apply to MCP like the web path (RR-JQ25OF). Stdio builds it per reload.
- Write tools hold `WriteLock` from preflight through the write: `&app.writeMu` remotely, a process-local mutex on stdio (RR-20DN1Y).
- Preflight order: gated read (row + face) → declared file property → not in `e.Redacted` → not locked (RR-47I4H6) → `AuthorizeWrite(update)` with a denied-write audit → write.
- The handler uses consumer-side `attachmentReader{List, Open}` and `attachmentWriter{WriteAttachment, DetachFile}`; read tools never hold the writer (RR-0LVR13).
- One predicate for "visible file property" shared by all four tools; `list_attachments` drops files under undeclared or non-file properties (RR-H24203).
- MCP upload cap `MaxUploadBytes` = 16 MiB (remote: `min(max_attachment_bytes, 16 MiB)`). A length check on the base64 string runs before decoding. The go-sdk `MaxRequestBodyBytes` is set from `EncodedLen(16 MiB)` + 1 MiB headroom via `HTTPHandler`; there is no outer `MaxBytesReader` (RR-5109KC).
- `read_attachment`: raster images (png/jpeg/gif/webp) as image content, UTF-8 `text/*` as text, everything else (including SVG) as an embedded blob with URI `rela://attachment/{id}/{property}/{file}` (not resolvable via `resources/read`) (RR-JI6F81, RR-DXIJG9).
- `.go-arch-lint.yml`: `mcp` may depend on `acl` and `attachment` (RR-31CANV).
- `dataentry.MCPHandlerFactory` takes an `MCPHost{AttachmentPolicy, AttachmentRunner, WriteLock}` argument; no new `App` method.
- Accepted residual risk: the manager's field-level write gate runs after the bytes are stored, so a field-level deny at `max: 1` with the same file name can overwrite bytes before the patch is refused. This predates the ticket and affects the web path equally; it is a follow-up.

**Files to modify:**
- `internal/attachment/attachment.go`, `processor.go` (+ tests)
- `internal/audit/attachment.go` (new), `internal/dataentry/handlers_attachment.go`, `attachment_handler.go`
- `.go-arch-lint.yml`
- `internal/mcp/server.go`, `internal/mcp/tools.go`, new `internal/mcp/tools_attachment.go` (+ `tools_attachment_test.go`, `acl_test.go`, goldens, `test_helpers_test.go`)
- `internal/cli/mcp_wiring.go`
- `cmd/rela-server/mcp.go`, `internal/dataentry/mcp_http.go`, `internal/dataentry/app.go` (factory call)
- `docs/mcp-server.md`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `id`: resolved only through the gated reader; miss and deny look the same.
- `property`: must be a declared `file` property of the entity's type (metamodel allowlist) and visible to the caller.
- `file_name`: used only as a store key; `store.NormalizeFileName` / `ValidateFileName` reject separators. Never a filesystem path built by MCP.
- `content`: base64 (std, padded) decoded with a size bound; invalid input is a tool error.

**Security-Sensitive Operations:**
- Attachment read: row + face + field gate before any store access to bytes.
- Attachment write/delete: `update` authorization before any store write; denied-write audit.
- Upload content: same `PolicyProcessor` (MIME allowlist, scan fail-closed) as web/CLI.
- Resource exhaustion: decoded size capped at the configured limit; inline read cap 10 MiB; MCP HTTP body capped.
- Store errors are not echoed on the read path (log server-side, generic tool error), as in `handleV1GetAttachment`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. AC1: in-process MCP server over memstore; attach -> list -> read -> delete round trip, for `max: 1` and `max: 3` properties (tool_calls golden + table test).
2. AC2: `gatedServer`-style fixture (production `GatedReads` + `acl.Declarative`): hidden entity vs nonexistent give identical errors for list/read/attach/delete; `visible:`-hidden file property is omitted from list and 404s on read.
3. AC3: principal with read-only role: attach and delete return forbidden, the store holds the original bytes unchanged, and an audit record with `op=attachment-write` exists.
4. AC4: disallowed MIME (e.g. `.exe` body to a property with `accept: [image/png]`) is rejected; `max: 2` at capacity gives the at-capacity error; content above `MaxBytes` is rejected.
5. AC5: PNG returns image content, `.md` returns text, `.pdf` returns an embedded blob; a file over the inline cap returns the size error.
6. AC6: bad base64, unknown property, non-file property, missing args.
7. Remote: HTTP test through `HTTPHandler` with the body cap: oversized body fails cleanly.
8. Manual: run `rela mcp` against a scratch project with an MCP client (or JSON-RPC over stdio) and do the round trip.

**Edge Cases:**
- Empty file (0 bytes): allowed if MIME policy allows; read returns empty content.
- `file_name` with `../` or `/`: normalized to a safe name (write) or rejected as not found (read).
- Unicode file name: normalized per `NormalizeFileName`.
- `delete_attachment` with no `file_name` on a multi-file property: error listing file names.
- Deleting a missing named file: idempotent success (matches web path). Unnamed delete on an empty property: error.
- Locked (git-crypt) entity: write refused.
- Entity with a stale property value but no stored file: list shows store truth.

**Negative Tests:** listed in AC6 and AC3.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Large base64 payloads inflate memory (about 2.3x the file). Mitigation: size caps above; the default 64 MiB limit is the ceiling.
- Changing `MCPHandlerFactory`'s signature touches the rela-server wiring. Mitigation: small surface, covered by existing startup tests.
- Stdio has no command runner, so properties with `scan:` configured reject MCP uploads (fail closed), the same as `rela attach`. Documented.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/mcp-server.md: the four tools, the inline cap, the gating rules.
- [x] docs/attachment-security.md: MCP is a third upload ingress under the same policy.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-5109KC, RR-CN70H8, RR-20DN1Y, RR-7I4002,
RR-JQ25OF (significant); RR-KPQD8A, RR-H24203, RR-31CANV, RR-47I4H6, RR-0LVR13,
RR-QZ98EZ, RR-ZARZTR (minor); RR-JI6F81, RR-DXIJG9, RR-9DDPMV (nit). All folded
into the revised approach above.
