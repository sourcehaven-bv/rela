---
id: IMPL-ESVDOOC
type: implementation-checklist
title: 'Implementation: Analyze reports hidden fields as missing'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestACLAnalyze_PropertiesSkipsHiddenFields)
- [x] Integration tests written (test full flow, not just units) (the test calls the `/_analyze` handler with a bound ACL request and a field resolver, so it runs the gated reader, the redaction and the Properties check together)
- [x] Happy path implemented
- [x] Edge cases from planning handled (a hidden field with a value and without one both give no finding, because Redacted lists every hidden field; an entity-level finding keeps its message; a property without a label uses its name)
- [x] Error handling in place (errors surfaced, not swallowed) (no new error paths)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: uses the package's seedEntity and newAppFromParts helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: the expected message is the contract under test)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no object comparison)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** rela-server (fs build) on a copy of the harbourmaster
project whose customer role has a `visible:` list for account without org_id. As
the customer, `GET /api/v1/_analyze` on develop returned one Properties error
"This field is required" on ACC-001 (org_id holds a value but is hidden). With
the fix it returned no issues, for the customer and for staff.

## Quality

- [x] Code follows project patterns (check similar code) (uses the header's Redacted list, as the element range code does)
- [x] Checked for DRY opportunities (no existing property label helper with this fallback in dataentry)
- [x] No security issues introduced (Redacted is the full hidden set from policy, whether or not the field holds a value, so skipping it reveals nothing about the value)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
