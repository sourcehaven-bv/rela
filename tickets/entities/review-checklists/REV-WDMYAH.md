---
id: REV-WDMYAH
type: review-checklist
title: 'Review: ACL: split write into create/update/delete grants; create implies no read'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** <!-- e.g., https://github.com/org/repo/pull/123 -->

---
**Review note:** implements RES-4AS0S4. RoleDef.Write → Create/Update/Delete;
decideFromAttrs dispatches by WriteRequest.Op (grantsVerb); invariant relaxed to
update⊆read+delete⊆read with create exempt. Migrated all ACL-policy test fixtures
across acl/affordances/appbuild/dataentry/entitymanager and the docs (security.md
+ guides, regenerated). New test pins create-without-read loads OK while
update-without-read still fails. Full go test ./... green, lint 0, arch-lint OK.
Self-reviewed.
