---
id: REV-S577SC
type: review-checklist
title: 'Review: pgstore migrate and write advisory locks are not schema-qualified'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

Full default-build `go test ./...` green; both build tags compile; pgstore suite
green against a real PostgreSQL 15 (`ok ... 19.5s`, including the storetest
conformance harness, fuzz corpus and tx stress tests). `just arch-lint` OK.
`just docs-check` OK (generated docs regenerated from `docs-project/`).
`just coverage-check`: 76.3%, package and total floors satisfied.
golangci-lint clean for this change — the only remaining findings in the package
are three pre-existing issues in `graphquery_explain_test.go` (govet shadow,
misspell, whitespace), confirmed not mine via `git diff`. Markdown lint clean.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

The review ran against the **earlier** Go-side implementation of this fix and
found 6 issues (1 critical, 2 significant, 2 minor, 1 nit). Five of them were
defects *in that mechanism* — a NULL-vs-empty scan, a `current_schema()` /
table-schema divergence, a hash too narrow, a debug-only starvation log, and
tautological key-derivation tests.

That mechanism was then **abandoned** on rebase in favour of #1217's SQL-level
`hashtext(current_schema())` form, which is simpler and never had any of those
problems. The corresponding review-responses were deleted rather than marked
addressed: keeping critiques of code that no longer exists would misrepresent
the change. Their substance is preserved where it still applies — the starvation
warning was kept, and the "assert on captured versions, not lock acquisition"
insight now drives the sweep test.

**RR-NAOG9H (significant) survives and is addressed**: the rolling-deploy window
where old and new binaries compute different keys. Assessment updated for the
new mechanism; note #1217 already shifted the sweep lock's key space the same
way, so this is the second and last such transition.

Self-review: the diff is 3 production files (`migrate.go`, `tx.go`, `sweep.go`),
1 new test file, 1 docs source + its generated output, and the ticket graph.
`purge.go`, `export_test.go` and `tx_stress_test.go` were reverted to develop —
the earlier draft had touched them only because of the abandoned mechanism.

**Review Responses:** RR-NAOG9H (significant, addressed). Six others deleted as
obsolete — see above.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Two schemas migrate independently — **PASS**.
  `TestMigrateDoesNotBlockAnotherSchema`, verified to FAIL against the bare key.
- Two schemas write independently — **PASS**.
  `TestWriteTxDoesNotBlockAnotherSchema`, verified to FAIL against the bare key.
- Locks still mutually exclusive *within* a schema — **PASS**.
  `TestXactAdvisoryLocksAreSchemaScoped` asserts the negative case too.
- Sweep behaviour unchanged and still correct — **PASS**. Upstream's
  `TestSweepAdvisoryLockIsSchemaScoped` plus
  `TestSweepCapturesWhileAnotherSchemaHoldsLock`, which asserts captured versions.
- pgstore conformance suite still passes — **PASS**.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix, not an
  enhancement)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix, not an enhancement)

The postgres guide claimed write transactions were serialized "deployment-wide",
which this change makes false. Corrected in the **source**
(`docs-project/entities/guides/GUIDE-postgres-backend.md`) and regenerated —
`docs/` is auto-generated and CI enforces it matches.

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

Branch `fix/pgstore-schema-scoped-advisory-locks`, rebuilt on current `develop`
(a2b78c0c) after #1217 landed the sweep half of this defect.

**PR:** not created yet
