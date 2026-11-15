---
id: PLAN-I1P8NMQ
type: planning-checklist
title: 'Planning: Lua rela.attach_file: store script bytes on a file property'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `rela.attach_file` in actions, schedules, flow job steps and
background automations (single-project and tenant mode). Out: the synchronous
cascade and document renders (binding absent, as for `attach_render`);
declarative webhooks run no Lua, so they reach it only through a flow or
background automation they trigger.

**Acceptance Criteria:**
1. A script that may update the entity stores the bytes; the result names the
stored file, size, sha256 and MIME (dataentry test).
2. A hidden or absent entity, a non-file property and a denied write each
return nil and a message (dataentry tests).
3. Over-limit content and a MIME outside the allowlist are refused like an
upload (dataentry tests).
4. A `mime` that does not match the sniffed bytes or the filename extension
is refused (dataentry test).
5. Bad arguments raise (lua tests).
6. Schedules and flow steps get the binding through the appbuild holder
(appbuild test).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small binding on an existing pattern)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: in-repo pattern)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-repo pattern)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `rela.attach_render` (internal/lua/attachrender.go,
internal/dataentry/attach_render.go, internal/appbuild/attachrender.go) already
runs the upload preflight and write for a script. attach_file reuses its
helpers.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** New consumer-side interface `lua.AttachFiler` and
`WriteDeps.AttachFile`. The data-entry implementation binds request state, runs
`attachmentWriteCheck`, takes an upload slot and calls `storeUpload`. The
attach_render helpers take the binding name so both report the same refusals. A
late-bound holder in appbuild, set from dataentrywire, gives schedules and flow
steps the binding.

Alternative rejected: a raw store write from Lua. It would skip the upload
policy and the field gates.

**Files to modify:** internal/lua/{attachfile.go,deps.go,runtime.go},
internal/dataentry/{attach_file.go,
attach_render.go,write_handler.go,actions_ops.go,app.go},
internal/appbuild/{attachfile.go,appbuild.go}, internal/dataentrywire/wire.go,
docs/lua-scripting.md (via docs-project).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Script arguments; content from external HTTP.
The bytes go through the upload MIME allowlist (sniffed), scan and size cap. The
filename is normalized by the attachment service as for an upload.

**Security-Sensitive Operations:** A write as the script's principal. The read
gate is bound fail-closed; hidden targets get the uniform not-found message.
Internal errors go to the log.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] ~~Integration test approach defined~~ (dataentry tests drive the real app and store)

**Test Scenarios:** See acceptance criteria.

**Edge Cases:** binary content with NUL bytes; empty content; filename with no
extension and a `mime`; a `mime` with parameters.

**Negative Tests:** missing entity/property/content/filename, non-string values,
unknown option, call inside a store transaction.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Memory: content is held in Lua already; the upload slot bounds
concurrent writes. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/lua-scripting.md.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: same design as TKT-1TS55R, reviewed there; covered by code review)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none
