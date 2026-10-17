---
id: PLAN-4O7K1U
type: planning-checklist
title: 'Planning: File properties show the file name in table and list cells'
started: "2026-10-16"
completed: "2026-10-16"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
In: `file` properties on dense surfaces (list columns, `display: table` view
columns, card fields) and the `formatCellValue` text fallback.
Out: the entity detail page and forms (FileWidget, unchanged); image previews
in cells; any server or config change.

**Acceptance Criteria:**
1. A cell with one file shows a paperclip and the file name, linked to the download URL the entity page uses, never the storage path.
2. A cell with several files shows "N files"; the names are in its title and in a list it opens to.
3. An empty file cell is blank.
4. A long name truncates with an ellipsis and carries the full name in `title`.
5. The link addresses the face the row was served in.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small frontend change)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: in-repo widget)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `densePropertyRoutingHint` (TKT-S9C14S) routes dense surfaces; `file` went to text to avoid FileWidget's per-cell `<img>` previews.
- The server's `computeAttachments` builds the download href as `_self + /_attachments/{property}/{fileName}`; `metamodel.FileRefs` derives the file name as the path's base name.
- `entityRef` gives a row's address, face included.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
A `file` member of `WidgetHintKind`. `densePropertyRoutingHint` routes a file
property to it (also when `list: true`). `resolveFromHint` serves it from a
dense-only map with `FileCellWidget`, so it is not a registered widget and not
a `widget:` override. The widget reads names with `fileRefNames` and links
them with `attachmentFileUrl(entityType, address, property, name)`. List and
table cells pass the row address as `entityId`. `formatCellValue` shows file
names for the text fallbacks.

Alternatives rejected: FileWidget in display mode (needs `_attachments`, which
list rows do not carry, and renders previews); registering a new widget name
(becomes a config `widget:` override and needs the Go widget table); a
per-type `v-if` in the views (frontend/CLAUDE.md forbids it).

**Files to modify:**
`frontend/src/widgets/{types.ts,viewRouting.ts,registry.ts,FileCellWidget.vue}`,
`frontend/src/utils/{fileRefs.ts,format.ts}`, `frontend/src/api/attachments.ts`,
`frontend/src/components/entity/{ViewTableCell.vue,EntityDetail.vue}`,
`frontend/src/components/lists/EntityList.vue`, docs.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
The value comes from the server, already redacted for the reader. The file
name is escaped with `encodeURIComponent` in the URL and rendered as text.

**Security-Sensitive Operations:**
Download links. The server gates the download route on entity read, field
visibility and the face's own value, so a link opens nothing the entity page
would not.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1-5: `EntityDetail.table.test.ts` file cells (mounted view section) and `FileCellWidget.test.ts`.

**Edge Cases:**
Single string value, legacy bare name, backslashes, duplicates, special
characters in the name, unknown entity type, no owning entity (card fields).

**Negative Tests:**
Empty value, non-string value, no plural registered: no link, no crash.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
A link inside a row link could open the row too: the widget stops click
propagation. Effort: s.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/metamodel.md - how a file property shows in cells
- [x] frontend/CLAUDE.md - the dense `file` rule

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small change inside the existing dense-routing design)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A)

**Design Review Findings:** none
