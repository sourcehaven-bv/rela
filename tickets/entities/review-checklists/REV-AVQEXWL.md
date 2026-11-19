---
id: REV-AVQEXWL
type: review-checklist
title: 'Review: Flow guard may read a field every visible: block shows'
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): acl, flowcfg, realtimecfg, affordances, appbuild and flowengine pass locally; CI runs the full suite
- [x] Lint clean (`just lint`): golangci-lint on the changed packages, arch-lint clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): only tests added; CI enforces the floors

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): cranky-code-reviewer and rela-security-reviewer
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-LK2PXSJ)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-LK2PXSJ, RR-CCLOMOW, RR-AIXFUKC, RR-TMYPV24,
RR-IBGH7HK, RR-W2TAPQF, RR-D1H0LDK, RR-N2SO2E3

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: guard over a property every block lists loads (flowcfg test; harbourmaster copy validated).
2. PASS: guard over a left-out or when-gated property is refused (flowcfg and realtimecfg tests).
3. PASS: face and `*` keys still restrict (TestFieldRestricted).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-KICFJWI

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
