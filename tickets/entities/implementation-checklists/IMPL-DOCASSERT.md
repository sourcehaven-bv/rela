---
id: IMPL-DOCASSERT
type: implementation-checklist
title: 'Implementation: Executable manuals — assertions in the rela-docs doc language'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built the example handbook against the real project
(`prototypes/data-entry/manual/tickets-manual.md`) and verified each assertion
both passes and *fails for the right reason* — a passing assertion that has
never been seen to fail proves nothing:

- Widened `viewer` with `update: ["*"]` in `acl.yaml` → the build stopped at
  `manual:80` with `claimed: refused / actual: PERMITTED / rule: role-grant/viewer`,
  then restored the file and confirmed the build goes green again.
- `api{status=200}` → forced to 201, failed printing the actual status and body.
- `api{error=...}` → forced a wrong code, failed naming both codes.
- `identical_to` on two missing entities passes; against a live entity vs a
  missing one it fails, so it is not vacuously true. Its `instance` exclusion
  was verified in BOTH directions (differing `type` and differing status are
  still caught).
- Mutation testing: 7 mutants, all COMPILING, all killed — removing the
  claimless-call guard, the `exactly` over-inclusion check, the presence-vs-
  emptiness distinction in `hasField`, the nil-policy guard, the `instance`
  normalization (over-broad), the body comparison, and the api claimless guard.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
      patterns extracted to a helper / constant / type where it
      sharpens the contract (don't extract for its own sake; CLAUDE.md
      "three similar lines is better than a premature abstraction"
      still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Notes:**

- `APIClient` mirrors the existing `Capturer` consumer-side interface, with the
  same build-tagged seam (`apiclient_fs.go` / `apiclient_postgres.go`) so the
  core docs package never imports a server, and postgres fails loud rather than
  seeding into a live database.
- The pure assertion cores (`checkShows`, `checkAuthz`, `checkAPI`,
  `checkIdentical`) are split from the Lua bindings so the failure PROSE is
  under test — a doctest's value is its failure output.
- `docs → principal` was added to `.go-arch-lint.yml` with a stated reason:
  `acl.Declarative`'s API takes a `principal.Principal`, so it is implied by the
  existing `acl` dependency; `principal` is a dependency-free leaf and the
  sibling `docscapture` already lists it.
