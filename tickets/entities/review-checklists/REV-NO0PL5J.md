---
id: REV-NO0PL5J
type: review-checklist
title: 'Review: Flows M5c: delete the old transition and automation paths and dogfood the tickets workflow'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A locally: `go test -race` passes on every changed package: flowcfg, cli, appbuild, entitymanager, affordances, dataentry, attachment, remotesync, comments, acl, metamodel, automation, autocascade and archguard; CI runs the full suite)
- [x] Lint clean (`just lint`) (golangci-lint on the changed packages: 0 issues)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: CI enforces the floors; no package was added, and .testcoverage.yml had no statemachine entry)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent; rela-security-reviewer also ran and found nothing)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-VSMLC99, RR-Q2WOYS3 (significant, addressed);
RR-8PA7PQA, RR-Q0KY335, RR-5536IEQ, RR-ZC2NWR0, RR-7Q2B6HN, RR-83NRBYF (minor,
addressed); RR-JLGQ0AE, RR-EVV1FB6 (nit, addressed); RR-01VK33D, RR-IWWKAJU,
RR-NI6DDGI (nit, wont-fix with reason). Design review: RR-YXYX9EQ, RR-G4VKS89,
RR-LNAV79T, RR-3FUM5QZ, RR-U5V89VY, RR-0TLEFA1, RR-N9UEIVX, RR-NIBXSM1.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: internal/statemachine is gone; no Go file, arch-lint rule or coverage entry names it, and its tests pass in internal/flowcfg.
2. PASS: entitymanager, affordances, dataentry, appbuild, remotesync, automation and autocascade pass with -race; the scratch tickets walk created every checklist as before.
3. PASS: TestFlowShow_Headless and TestFlowShow_Run list and render headless flows with no flows.yaml; JSON carries kind.
4. PASS: DEC-EJ00DNZ records the unset decision; TestEnforceUpdate_UnsetIsNotInitial covers the entry value.
5. PASS: cardinality, properties and validations are clean for the new entities.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-8EI1J3V

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per the user's rule; the ticket carries the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: the PR is opened right after the ticket is done, as the workflow requires)
