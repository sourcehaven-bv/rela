---
id: PLAN-NG9GW1
type: planning-checklist
title: 'Planning: Lua list_entities face option'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a `face` option on `rela.list_entities` and
`admin.list_entities`. Out: a `world` option (needs the compiled worlds in
ReadDeps; the runtime holds only the default world), `rela.search` and
`rela.md.entity_refs` options. The empty-result bug in #1761 itself was fixed by
#1753.

**Acceptance Criteria:**
1. `list_entities("ticket", {face="draft"})` returns only draft rows, each with `face == "draft"`.
2. `face` composes with `filter`.
3. No option keeps the world selection.
4. An undeclared face, a faceless type, a malformed face or a non-string raises.
5. `admin.list_entities` takes `face` and refuses other keys.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: one option on an existing call)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal API option)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `store.AtFaces` (internal/store/faceselect.go) selects
named faces; `parseReadOpts` (internal/lua/readopts.go) already rejects unknown
keys; `entity.ParseFace` validates face grammar.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `listEntitiesArgs` accepts `face` and parses it with
`faceOption` (entity.ParseFace). `listSelection` maps a face to `store.AtFaces`,
else `store.InWorld(world)`. The gated binding raises when the metamodel
declares no such face. The elevated binding parses an options table whose only
key is `face`.

**Files to modify:** internal/lua/readopts.go, internal/lua/runtime.go,
internal/lua/elevation.go, internal/lua/listface_test.go,
docs-project/entities/guides/GUIDE-lua-scripting.md (+ generated
docs/lua-scripting.md).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** `face` comes from a script. It must be a string
that passes entity.ParseFace (allowlist grammar); the gated path also requires a
declared face. Anything else raises a Lua error.

**Security-Sensitive Operations:** The gated read still goes through
VisibleReader, so a named face is row-gated and redacted like a world read. The
elevated read already bypasses the ACL by design and keeps its read audit mark.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** TestListEntities_FaceOption (1-3),
TestListEntities_FaceOptionRejectsBadValues (4),
TestElevatedListEntities_FaceOption (5).

**Edge Cases:** Absent face keeps the world. Empty string fails ParseFace. An
entity without a row at the face is left out. The 2000-row bound still applies.

**Negative Tests:** Undeclared face, faceless type, uppercase face, numeric
face: Lua error naming the option.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Low. A face selection could bypass world gating only if it skipped
VisibleReader; it does not. Mitigated by reusing the same reader call.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/lua-scripting.md via GUIDE-lua-scripting.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: one validated option over an existing selection type; the code review covers it)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review, see above)

**Design Review Findings:** N/A
