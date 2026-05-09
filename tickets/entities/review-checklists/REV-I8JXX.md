---
id: REV-I8JXX
type: review-checklist
title: 'Review: Resolve entity-ID references to titled links in Lua markdown output'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full suite, race-enabled
- [x] Lint clean (`just lint`) — golangci-lint v2.11.4
- [x] Coverage maintained (`just coverage-check`) — 73.1% total, all package thresholds satisfied

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (RR-XY8HC adjacency, RR-V59XZ title injection, RR-VWUAM Unicode slug, RR-HK4NH store error)
- [x] All significant review-responses addressed (RR-3S0PV UTF-8 boundary, RR-MC0HE idKeyRe, RR-QR0X7 nil deps, RR-50CK0 type-set breadth)
- [x] All minor review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** All 24 RRs `status=addressed`.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** All 20 ACs from PLAN-KK2SE pass. Adjacency cases
(`TKT-1TKT-2`) and Unicode-boundary cases added during code review additionally
pass.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via has-docs~~ (N/A: docs are
edits to a generated guide source in `docs-project/`; no separate docs work).
- [x] User-facing documentation updated — `GUIDE-lua-scripting` source
in `docs-project/` gained `rela.md.resolve_refs` and `rela.md.entity_refs`
reference sections; `docs/lua-scripting.md` regenerated via `just docs`.
- [x] ~~Docs-checklist marked as done~~ (N/A: see above)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** <https://github.com/sourcehaven-bv/rela/pull/646>
