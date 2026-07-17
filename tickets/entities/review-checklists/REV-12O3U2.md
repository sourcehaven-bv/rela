---
id: REV-12O3U2
type: review-checklist
title: 'Review: ACL read-side: close the /_search match-on-hidden-field oracle (drop hits matching only visible:-hidden fields)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test -race` green: search, store, dataentry, mcp, appbuild)
- [x] Lint clean (`golangci-lint` 0 issues, default + postgres tags; `just plimsoll`, `just arch-lint` OK)
- [x] Coverage maintained (new code covered by conformance suite + handler + fail-closed tests)

## Code Review

- [x] Run `/code-review` (cranky-code-reviewer agent) — completed
- [x] All critical review-responses addressed (none critical)
- [x] All significant review-responses addressed (RR-8W40EW, RR-DILMO4, RR-DH2IPR)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-8W40EW, RR-DILMO4, RR-DH2IPR (significant, addressed);
RR-6757U2, RR-P1ID4S (minor, addressed); RR-T80MJ1 (nit, addressed).

## Acceptance Verification

- [x] Oracle closed: hit matching only a hidden field dropped — conformance
`HiddenOnlyMatchDropped` (bleve+linear+pg) + e2e
`TestACLSearch_HiddenFieldOracleClosed`.
- [x] Visible-field / id / content match survives — conformance + e2e control.
- [x] Fail-closed on missing provenance and on hidden-func error — pinned tests.

**Acceptance Status:** PASS — all criteria have automated evidence.

## Documentation (enhancement)

- [x] ~~Docs-checklist created~~ (N/A: docs updated inline in this ticket's scope)
- [x] User-facing documentation updated (GUIDE-acl-security + GUIDE-server-security: oracle marked closed; regenerated; `just docs-check` green)
- [x] ~~Docs-checklist marked done~~ (N/A: no separate docs-checklist)

## Final Checks

- [x] Commit messages explain the why
- [x] No TODOs/FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created and CI monitored
- [x] All CI checks pass (except the `review`-status ticket gate, cleared on done)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1089
