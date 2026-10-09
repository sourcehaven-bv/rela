---
id: BUGA-WOG3JF
type: bug-analysis-checklist
title: 'Analysis: Attachment upload and delete ignore the fields: write policy'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

TestAttachmentWrite_HonorsReadOnlyField: alice holds update on ticket, the field
resolver marks `screenshot` read-only, and a PUT to `_attachments/screenshot`
returned 200 and replaced the file. Any backend; the gap is in the dataentry
handler.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Fix: call `validateFieldWrite` in `attachmentWritePreflight` after the update
check; deny via `denyAffordance`. Upload and delete share the preflight. MCP
`attach_file` is not affected in the same way: `fields:` is a data-entry
affordance and no MCP write tool applies it, so MCP is consistent with its own
`update_entity`.
