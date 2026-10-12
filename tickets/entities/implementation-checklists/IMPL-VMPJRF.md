---
id: IMPL-VMPJRF
type: implementation-checklist
title: 'Implementation: Section hide_empty'
status: done
---

## Development

- [x] Unit tests written for new code — `validate_hideempty_test.go` (each
refusal, the side panel, YAML decoding) and
`TestOmitHiddenEmpty_KeepsIncomplete`.
- [x] Integration tests written — `sections_hideempty_test.go` drives
`handleV1Views`: hide vs. show, create on a later section, and read access.
- [x] Happy path implemented — `omitHiddenEmpty` drops empty opted-in sections
after `resolveSectionCreate`.
- [x] Edge cases handled — incomplete collections stay; rows the principal
cannot read count as absent; config refuses `empty_message`, `create:`, `source:
entry`, a missing source and side panels.
- [x] Error handling in place — every refusal is a config load error naming the key.

## Test Quality

- [x] Using fixture builders or factories for test data (`seedCreateView`,
`sectionCreateConfig`).
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with a test scenario
- [x] Edge cases manually verified

**Verification Evidence:** Copied `prototypes/data-entry/project` to a temp dir,
set `hide_empty: true` on the category view's "Blocked Tickets" section and
removed the only `blocks` edge reaching DevOps. `rela-server` returned
`_views/category/devops` without "Blocked Tickets" and `_views/category/backend`
with it. Adding `empty_message` next to it made `rela validate` fail with `sets
both hide_empty and empty_message`.

## Quality

- [x] Code follows project patterns (refusal style of `validateSectionCreate`)
- [x] Checked for DRY opportunities
- [x] No security issues introduced — emptiness is decided after the read
gate, and `isEmpty` was already on the wire.
- [x] No silent failures
- [x] No debug code left behind
