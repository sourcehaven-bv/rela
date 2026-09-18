---
id: IMPL-0Z6SS3
type: implementation-checklist
title: 'Implementation: analysis.faceDeclared treats a bare row as always-declared on a faced type'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests~~ (N/A: `CheckStates` is a read-only scan; the test
drives it through a real store with seeded rows, which is the full flow.)
- [x] Happy path implemented — `faceDeclared` asks the type: the zero face is
declared only where no `faces:` are.
- [x] Edge cases handled — alias-typed rows still resolve through
`GetEntityDef` (unchanged); a type the metamodel does not define still reports,
as before.
- [x] Error handling in place — no new error paths; the scan's existing
fail-loud store-error policy is untouched.

## Test Quality

- [x] Using fixture builders — reuses the package's `newServiceWith`,
`addEntity` and `mustFace` helpers.
- [x] No hardcoded values in assertions when the object is in scope
- [x] Only specifying values that matter — the metamodel declares two types,
one faced and one not, which is the whole discriminator.
- [x] ~~Interpolated values constructed from objects~~ (N/A: the assertions
check a finding code and an example ref, both operator-facing strings that are
literals in the test's own fixture by design.)
- [x] Property comparisons use the finding's own fields, not golden output.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**

- End-to-end against the in-tree reproducer, not only a unit test. Seeded
`prototypes/perf/project` with `rela dev seed --scale 0.01` (199 entities) and
ran `rela analyze states`:

  ```text
  ⚠ [bare-row-on-faced-type] : 35 row(s) — stored at the bare id on a type that
    declares `faces:`, so it names no declared face and no world can reach it
    (e.g. DOC-0001, DOC-0002, DOC-0003, DOC-0004, DOC-0005, …)
  ```

35 = 15 policies + 20 documents, every one written by `perfseed` at the bare
coordinate. Before the fix the same command reported nothing.

- Mutation-verified in **both** directions, each failing independently:
  - reverting to `return true` → "want exactly one bare-row finding, got 0"
  - changing to `return false` → "count = 2, want 1" and
"got [CTL-1 PAGE-1], want [PAGE-1]" (over-reports the faceless type)

- Full suite green: `go test ./...` with no failures, `arch-lint`,
`comment-lint`, `golangci-lint`, `docs-check`, `coverage-check` (analysis at
81.8%, both thresholds satisfied).

## Quality

- [x] Code follows project patterns — the new finding joins the existing
aggregate-and-emit shape rather than adding a parallel path.
- [x] Checked for DRY opportunities — the bare-face branch shares the same
`ptrAgg` aggregation; only the emitted sentence and code differ, which is the
actual difference.
- [x] No security issues introduced — detection only, no read-path or ACL
change, no stored data touched.
- [x] No silent failures — the point of the change is that a previously silent
condition now reports.
- [x] No debug code left behind — the seeded project was written to a temp dir
and removed.
