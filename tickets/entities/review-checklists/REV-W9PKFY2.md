---
id: REV-W9PKFY2
type: review-checklist
title: 'Review: Reject type-disallowed relation edges on every write surface'
started: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

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
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** cranky-code-reviewer and rela-security-reviewer ran on
the diff. Significant: RR-24V4VNO, RR-ZH9ECGO, RR-O547N7Z (all addressed).
Minor: RR-X2PKMXH, RR-7UHXZLH, RR-O29RKYX, RR-YHE98E6, RR-KG2UEOV, RR-92N09RX
(addressed); RR-ASVKR1T (wont-fix); RR-742RM3Y (deferred). Nit: RR-PC4WJGA,
RR-6B1ZFNX (addressed); RR-R5AR5GN, RR-D1MOR5Y (wont-fix). Design review:
RR-9OI141T, RR-QN7IOTA, RR-PWYFN3V (addressed).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: TestPatchRelations_RejectsDisallowedEndpointTypes (wrong-peer cases)
   asserts 422, unchanged edges and title, no audit; repro PATCH gave 422.
2. PASS: the same test's wrong-path-entity cases.
3. PASS: TestCreateEntity_RejectsDisallowedEndpointTypes; repro POST gave 422
   and GET of the new id 404.
4. PASS: TestCreateRelation_RejectsDisallowedEndpointTypes; repro per-edge
   POST gave 422.
5. PASS: the PATCH table covers canonical and inverse names on free,
   max_outgoing and max_incoming relations.
6. PASS: TestCreateRelation_RefusesTypeMismatch and
   TestReplaceRelations_RefusesTypeMismatch; the option is removed; `rela link`
   refuses the edge.
7. CORRECTED: unknown and required meta still warn with 200. The plan was
   wrong about a missing target: on develop it already answered 422
   `target_not_found`, after the entity update. It still answers 422, now
   before any write, and a hidden peer gets the same answer
   (TestRelationWrite_HiddenDisallowedPeerAnswersAsMissing).
8. PASS: .ignored/check_rel_types.py reports 0 violations in tickets,
   docs-project and the prototype projects after removing two relations.

Also: a caller without create permission still gets 403 before any 422
(TestCreateEntity_DeniedCreateIs403BeforeEndpointType).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-43H8III

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

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
