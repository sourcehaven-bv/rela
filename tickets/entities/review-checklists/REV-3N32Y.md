---
id: REV-3N32Y
type: review-checklist
title: 'Review: Metamodel parsing of encrypted: declarations + groups config (slice 2)'
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

**Review Responses:**

Cranky-code-reviewer surfaced zero critical and six significant
findings. All significant were addressed directly in the
implementation; no `review-response` entities were filed.

- **#1 Nondeterministic error reporting** — fixed by sorting entity
  names and property names before iteration in
  `Metamodel.ValidateEncryption`.
- **#2 Identity validation** — added `GroupErrorInvalid` kind + reject
  empty group names, empty identities, and whitespace-stripped
  identities at `LoadGroups` time. Added `ErrInvalidIdentity` sentinel
  + tests for each case.
- **#3 `projectRoot` parameter on `LoadWithGroups`** — dropped;
  defaults to `filepath.Dir(path)` internally. YAGNI: single caller
  shape.
- **#5 Error context** — `groups.go` error messages now include the
  full path (not just `groups.yaml`) so multi-project logs stay
  diagnosable.
- **#6 `sort.Strings` over hand-rolled insertion sort** — replaced
  `sortStrings` in `entity_def.go` with stdlib `sort.Strings`; deleted
  the hand-rolled helper and its test. `sort` is already imported in
  other files in the package.
- **#10 Test path construction** — `wantPath` values in
  `validation_encryption_test.go` now built via `fmt.Sprintf` from the
  same format string as production, matching the team's test-writing
  convention.

Not addressed (with reason):

- **#4 `FSGroupsLoader` service interface** — legitimate sharp edge
  but premature abstraction before slice 3 actually consumes it.
  Flagged in the slice-3 ticket as a decision point.
- **#7 Duplicate error filename context** — minor; error already
  includes group name + identity which uniquely locates the problem.
- **#8 `slices.Clone` on `Recipients`** — "do not mutate" godoc is
  enough for the single intended caller (slice 3 reads + passes to
  wrap).
- **#9 Empty-file semantics doc** — readable in the code; unnecessary
  to duplicate in test comments.
- **#11 Delete `TestSortStrings`** — done via #6 (function removed,
  test removed).
- **#12 `SchemaPath` typed** — only two call sites, both in
  `validation.go`. YAGNI.
- **#13 Cache scan result on Metamodel** — defer to slice 3 pending a
  profile.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

All 10 acceptance criteria from TKT-OGLXI have matching tests, all passing:

1. Property `encrypted: engineering` parse → `TestEntityDef_EncryptedProperties_Mixed` PASS
2. Body `encrypted_body: exec` parse → `TestEntityDef_BodyGroup_Encrypted` PASS
3. `LoadGroups` recipients → `TestLoadGroups_Basic` PASS
4. Missing + no encryption → `TestLoadWithGroups_EndToEnd_MissingGroupsNoEncryption` PASS
5. Missing + encryption declared → `TestLoadWithGroups_EndToEnd_MissingGroupsWithEncryption` PASS
6. Unknown group → `TestValidateEncryption_Property_UnknownGroup`, `TestValidateEncryption_Body_UnknownGroup` PASS
7. Duplicate identity → `TestLoadGroups_DuplicateIdentity` PASS
8. Recipients ordering + unknown group lookup → `TestLoadGroups_Basic` PASS
9. No `internal/encryption` import → `go-arch-lint check` PASS
10. Coverage ≥ 90% → new code at 100%; package baseline unchanged

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: slice 2 is metamodel plumbing with no user-facing surface; docs land in slices 5/6)
- [x] ~~User-facing documentation updated~~ (N/A)
- [x] ~~Docs-checklist marked as done~~ (N/A)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** (to be created after commit/push)
