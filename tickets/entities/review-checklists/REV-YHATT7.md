---
id: REV-YHATT7
type: review-checklist
title: 'Review: rela acl who-can <verb> <entity> — list principals with access to one entity + provenance (UC3)'
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

**Review Responses:** Design-review: [[RR-7UXWNA]], [[RR-N16SDV]],
[[RR-CY6WYR]], [[RR-GC751G]], [[RR-K72ML0]] (all addressed). Code-review
(cranky): [[RR-C5Q743]] (critical — false negative for an actor that is also a
membership target; fixed by removing topology-based group-exclusion),
[[RR-XC2NTO]] (duplicate rows; fixed by merge-by-effective-principal),
[[RR-2NZRXO]] (non-stable sort; fixed), [[RR-EFFDQL]] (principal_property
untested; test added). All addressed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** All 11 ticket criteria PASS with tests:
all-six-route-kinds, read-vs-runtime two-way conformance (the
anti-false-negative guard that caught the critical bug), missing-entity error,
everyone-once, multi-route redundancy, unknown-verb fail-closed,
principal_property resolution + merge, versioned JSON, delete-verb narrowing.
CLI-level: text/JSON/missing/no-policy.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: single subcommand, self-documenting via `--help`; a `docs/cli-reference.md` entry belongs with the follow-up `map`/`can` slice when the full command family lands)
- [x] User-facing documentation updated — command godoc + `--help` document the data-entry-transport caveat and the group-entity reporting behavior
- [x] ~~Docs-checklist marked as done~~ (N/A per above)

**Docs Checklist:** N/A (see above)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass — every code check green on the first CI run; the only red was the "Rela Tickets" workflow gate (ticket not yet `done`), which this review completion resolves
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1141
