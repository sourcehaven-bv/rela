---
id: REV-RNL3Q9J
type: review-checklist
title: 'Review: Tenant management API behind the edge'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A locally: the full suite does not fit the machine's current load; the changed packages pass under the default and postgres tags, and CI runs the full suite)
- [x] Lint clean: golangci-lint per changed package (default and postgres tags), `just arch-lint`, `just plimsoll`
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: needs the full suite; every new file has tests, CI enforces the floors)

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-WGGX1VL, RR-K3E8SYN, RR-2DLIG3B (significant,
addressed); RR-FK29OG8, RR-2A7IXVE, RR-J5L6Y9B, RR-XLO0N9P (minor, addressed);
RR-AVZH1WH (minor, deferred); RR-5HVYY9C, RR-SA37UF9, RR-FNSF2U9 (nit, wont-fix)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Operator assertion with the configured role reaches the API; user, wrong role or org-bound operator refused: PASS (TestTenantAPI_Auth, TestTenantAPIOperatorAssertion, TestTenantAPI_OperatorRefusedElsewhere).
- PUT create and GET status: PASS (TestTenantAPI_CreateThenRead, SlowCreateIsAccepted).
- PATCH state active/read_only/suspended, plan, entitlements: PASS (TestTenantAPI_States, Patch, PatchRefusals; TestProvisioner_Apply).
- Read-only refuses writes: PASS (TestReadOnly, TestRegistry_LeaseCarriesCurrentState).
- DELETE archives, data kept, access refused: PASS (TestTenantAPI_Archive, TestProvisioner_Archive).
- GET usage users and storage: PASS (TestTenantAPI_Usage, TestClusterResolver_Usage).
- Plan and entitlements visible to the tenant: PASS (TestTenantAPI_StatusShowsPlan).
- Design in the docs: PASS (GUIDE-postgres-backend, "Managing tenants through the tenant API").

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-19JCA3E

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: the requester asked for minimal commit messages; the why is in the ticket and the guide)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened with gh after the ticket is done, per TKT-UFV01M; CI is monitored there)

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
