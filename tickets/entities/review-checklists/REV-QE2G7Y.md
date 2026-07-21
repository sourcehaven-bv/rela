---
id: REV-QE2G7Y
type: review-checklist
title: 'Review: ACL doc-fields: RoleDef.description + top-level policy description (rela-docs phase 1b)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `go test ./internal/acl/...` green; cross-package `./internal/dataentry/... ./internal/appbuild/...` green (shared `Policy` struct change breaks nothing)
- [x] Lint clean — `just lint` 0 issues; `just arch-lint` OK; `just lint-md` 0 issues
- [x] Coverage maintained — `just coverage-check` PASS (package floor 50% + total 65% both satisfied; internal/acl 77.4%)

## Code Review

- [x] Ran `/code-review` (cranky-code-reviewer) — **verdict: ship it.** No critical, no significant. Two nits, both defensible as-is.
- [x] All critical review-responses addressed — none raised
- [x] All significant review-responses addressed — none raised
- [x] Self-reviewed the diff for unrelated changes — scoped to `internal/acl/policy.go` (+2 fields, +1 allowlist key), new `docfields_test.go`, new example `acl.yaml`, ACL guide source + regenerated output

**Review Responses:** None. No critical/significant findings, so no review-response
entities required. Nits recorded here for the record:
- (nit) `TestPolicyDocFields_RoundTrip` compares p2-vs-p1 (tag fidelity), not string
  literals — correct assertion for a round-trip; the actual regression risk (a
  wrong yaml tag dropping the value) IS caught. Left as-is.
- (nit) `TestLoadPolicy_DocFields_Present` asserts non-empty rather than exact prose
  — deliberately loose (prose churns); AC1/AC2 only require parse+round-trip. Combined
  with the round-trip test, adequate. Left as-is.

Reviewer specifically confirmed: (a) `Validate()` byte-identical, fields never on the
authz path (grep-verified); (b) the unknown-key-suppression test is sound — it asserts
`description` is absent from warnings AND a sibling `bogus_key` IS present, proving the
warning path is live not muted; (c) the example `acl.yaml` needs NO membership-hardening
warning because it uses raw-principal assignments with no `membership_relation` /
`role_relations`, so the RR-7O6Q self-promotion vector does not exist — a wildcard
editor role is self-evidently wildcard, categorically different from the phase-1a
minimal-group-policy footgun.

## Acceptance Verification

- [x] Each acceptance criterion tested (see IMPL-2NM7HZ)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- AC1 RoleDef.description parse+round-trip: PASS (TestLoadPolicy_DocFields_Present, TestPolicyDocFields_RoundTrip)
- AC2 Policy top-level description parse+round-trip: PASS (same two)
- AC3 description key not warned: PASS (TestLoadPolicy_DescriptionKeyNotWarned; + sanity-negative that bogus_key still warns)
- AC4 backward-compat + Validate unaffected: PASS (TestLoadPolicy_DocFields_Absent asserts empty + Validate()==nil; round-trip covers marshal path)
- AC5 example acl.yaml loads clean: PASS (TestLoadPolicy_PrototypeExample over prototypes/data-entry/project/acl.yaml)
- AC6 tests per field + suppression: PASS (5 tests)

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs` (DOCS-M5CJG5)
- [x] User-facing documentation updated — ACL guide (GUIDE-acl-overview.md → docs/acl-overview.md) documents both fields + the non-authz note
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-M5CJG5

## Final Checks

- [x] Commit message explains the why — `feat(acl): doc-fields RoleDef.description + policy description (TKT-JO2SAD)`
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr`~~ (done-before-PR gate: PR runs AFTER this ticket is `done`, via `/pr`)
- [x] ~~All CI checks pass~~ (verified locally: go test / just lint / arch-lint / lint-md / coverage-check all green; CI confirms on the PR)
- [x] ~~PR URL documented below~~ (recorded when `/pr` opens it)

**PR:** _pending — opened via `/pr` after done_
