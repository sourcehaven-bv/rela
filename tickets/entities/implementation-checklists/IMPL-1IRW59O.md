---
id: IMPL-1IRW59O
type: implementation-checklist
title: 'Implementation: Scheduled-tasks guide uses peer naming instead of remotes'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: documentation only, no code)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: documentation only, no code)
- [x] Happy path implemented
- [x] ~~Edge cases from planning handled~~ (N/A: no edge cases for a wording fix)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: documentation only, no code)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no tests, documentation only)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no tests, documentation only)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no tests, documentation only)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no tests, documentation only)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no tests, documentation only)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: no edge cases for a wording fix)

**Verification Evidence:** Edited the source guide and ran `just docs`; only
`docs/scheduled-tasks.md` changed in `docs/`. A case-insensitive grep for "peer"
finds nothing in either file, and the new link target `docs/remote-sync.md`
exists.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] ~~Checked for DRY opportunities~~ (N/A: documentation only, no code)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: documentation only, no code)
- [x] No debug code left behind
