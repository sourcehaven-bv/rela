---
id: REV-9yda
status: in-progress
title: 'Review: Add metamodel cleanup/trim command'
type: review-checklist
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [ ] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [ ] All critical review-responses addressed
- [ ] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** <!-- List IDs of review-response entities created, e.g., RR-xxxx -->

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- AC1: PASS - Shows unused entity types with references
- AC2: PASS - Shows unused relation types with references
- AC3: PASS - Shows unused custom types (enums)
- AC4: PASS - --threshold flag works for low-usage types
- AC5: PASS - --cleanup removes only safe types
- AC6: PASS - --dry-run previews changes
- AC7: PASS - Custom types cleanup included
- AC8: PASS - JSON output format works
- AC9: PASS - MCP analyze_schema tool available
- AC10: PASS - 82.5% test coverage

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/235
