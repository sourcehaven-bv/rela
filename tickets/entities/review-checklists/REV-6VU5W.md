---
id: REV-6VU5W
type: review-checklist
title: 'Review: MCP update_entity should support deleting properties'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full suite green, no regressions
- [x] Lint clean (`just lint`) — 0 issues
- [x] Coverage maintained (`just coverage-check`) — total 72.9%, all package floors met

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none surfaced)
- [x] All significant review-responses addressed (4 of 4)
- [x] Self-reviewed the diff for unrelated changes — only `internal/mcp/*` modified

**Review Responses:**

Significant (all addressed):
- RR-9K0L1 — `parsePropertiesArg` now rejects JSON `null` as malformed
- RR-6F1AF — required-property deletion now blocked at the MCP boundary with an actionable error
- RR-OQKCD — automation-on-delete contract documented precisely (no code change; matches user expectations)
- RR-15OVI — top-level tool description extended to match per-arg description

Minor (5 addressed, 1 won't-fix):
- RR-S9QFJ addressed (helpers consolidated into `filterProperties`)
- RR-XQRP3 addressed (over-clever no-op test reworked using new `priority` property)
- RR-5H2H0 addressed (handler-level JSON-string test added)
- RR-QU6BK won't-fix (empty-string-as-no-op preserves create-path symmetry; documented in tool description)

Nit (3 addressed, 1 won't-fix):
- RR-Q2C2D addressed (doc comment tightened)
- RR-ZRP4R addressed (helpers converted to free functions)
- RR-DDL04 addressed (description-text test now matches canonical phrase)
- RR-CGJ2O won't-fix (`-race` already covers this; no signal in a dedicated test)

Earlier design-review responses (all addressed): RR-3H7IW, RR-8N1CZ, RR-RUI15,
RR-1JHC2, RR-CDGL1, RR-OXOIT, RR-8S190.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
| --- | --- | --- |
| 1 (null deletes) | PASS | `TestHandleUpdateEntity_DeletesPropertyOnNil` |
| 2 (description discoverable) | PASS | `TestUpdateEntityToolDescriptionMentionsNullDelete` |
| 3 (existing semantics) | PASS | `TestHandleUpdateEntity_SetAndOverwriteStillWorks` + existing tests |
| 4 (delete-absent no-op) | PASS | `TestHandleUpdateEntity_DeleteAbsentPropertyIsNoOp` (using `priority`) |
| 5 (unknown property rejected) | PASS | `TestHandleUpdateEntity_DeleteUnknownPropertyRejected` |
| 6 (mixed set+unset) | PASS | `TestHandleUpdateEntity_MixedSetAndUnset` |
| 7 (delete-only survives guard) | PASS | `TestHandleUpdateEntity_DeleteOnlyCallSurvivesGuard` |
| 8 (empty string no-op) | PASS | `TestHandleUpdateEntity_EmptyStringIsNoOp` |
| 9 (JSON-string null) | PASS | `TestExtractPropertiesAllowNil_StringJSONNullDeletes` (helper) + `TestHandleUpdateEntity_JSONStringPropertiesNullDeletes` (handler) |
| Bonus (required prop guard) | PASS | `TestHandleUpdateEntity_DeleteRequiredPropertyRejected` |

## Documentation

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: the MCP tool description IS the user-facing reference; no separate docs-checklist needed for an internal-API enhancement.)
- [x] User-facing documentation updated — tool description in `tools.go` extended with the null-delete contract
- [x] ~~Docs-checklist marked as done~~ (N/A: see above)

**Docs Checklist:** N/A — MCP tool description is the API doc surface.

## Final Checks

- [x] Commit message explains the why, not just what — to be drafted at `/pr` time
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — null-as-delete is the JSON-Merge-Patch convention; tool description carries the contract

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass (Test, Lint, Coverage, Build, E2E, Frontend, Fuzz, Architecture, Demos, Docs, CodeQL, Vulnerability Check, Analyze (actions/go/javascript-typescript), Lint Markdown — all pass; Rela Tickets passes once this commit lands)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/645
