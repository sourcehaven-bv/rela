---
id: REV-QKFHC
type: review-checklist
title: 'Review: Add native relation-cardinality support to validation rules (relations: block on ValidationRule)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

`just test` green: 107 packages ok, 0 failures, race detector on. `just lint`:
`0 issues`. `just arch-lint`: `OK - No warnings found` — the constraint check
reads relations through the `lua.ReadDeps` helper, so `internal/validation`
still does not import `internal/store`. `just coverage-check`: package floor
(50%) and total (65%) both PASS, total 79.4%. Affected packages:
`internal/validation` 88.5%, `internal/metamodel` 87.2%.

Note on the environment: a second session was running its own `go test` /
`golangci-lint` against this checkout, which serialises on the golangci-lint
lock. The figures above are from runs that completed on the final tree.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

The review found a real defect that the original test suite did not cover, and
the fixes are the most valuable part of this ticket.

**Critical — a `max:` gate silently passed when a target could not be
evaluated.** The counting loop swallowed errors from `filter.MatchAll`,
`GetEntity` and the type lookup, skipping that target. Undercounting is
conservative for `min:` (the gate still fires) but inverted for `max:`: "a done
ticket must have no open critical review-responses" reported *satisfied*
precisely because the responses could not be checked. Reproduced with a failing
test before fixing (`TestRelationConstraint_UnevaluableTargetFailsClosed`). Fixed
by failing closed — an unevaluable target counts as matching whenever `Max` is
set. The 14 migrated gates were safe only by accident (single-target relations,
all enum values in range).

**Critical — checks that could not run reported "satisfied".** A malformed
`where:`, a store/context error from `OutgoingRelations`, and a nil
`VisibleReader` all returned "no violation". The store case is the dangerous
one: a dropped connection or a cancelled context made every relation gate in the
run pass. Now a malformed `where:` is a `LoadError` at rule-compile time (the
same channel and rationale as a malformed `when_condition:`, which this file
already documents), and a read failure or missing reader is reported per entity
instead of waved through.

**Significant — a typo in the relation type disabled the gate silently.** The
allowlist added by this ticket catches `relationz:` at the outer level, but
`has-reviewwww:` one level down loaded clean and counted nothing forever. Now a
load error, following the existing `validateValidationFaces` precedent. Same
check also rejects a constraint with neither bound, negative bounds, and
`min > max`.

**Significant — `where` semantics and visibility scope were undocumented.**
`filter.MatchAll` is type-aware where the replaced Lua did raw string equality:
wildcards, list containment and type coercion all apply. Separately, relations
are read through the ACL-gated reader, so a gate counts what the acting identity
can see and is not a global invariant. Both are now documented in
`docs/metamodel.md` and the `RelationConstraint` godoc. The visibility behaviour
is unchanged from the Lua implementation (which read through the same gated
reader), so this is a documentation gap, not a regression.

**Minor — nondeterministic violation order.** `rule.Relations` is a map; keys
are now sorted before iteration.

Deliberately not addressed, out of scope for this ticket:

- Deriving the key allowlist from struct tags by reflection instead of the
hand-maintained literal. The parity test already catches the direction that
breaks users (a field with no allowlist entry); the reverse only leaves a dead
entry. Worth doing, but it touches `validTopLevelKeys` too.
- Buffering in `ScriptReader.ListRelations` (two full materialisations per
constraint per entity). Not a concern for the checklist-shaped relations in use;
it would matter on a hub entity with very many edges.

**Review Responses:** none created as entities — findings were addressed
directly in this branch (commits `99fb16ef`, `3863cdf0`, `cdfd573c`).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. **`relations:` parsed and evaluated (min/max/where)** — PASS.
`TestRelationConstraint_Min`, `_Max`, `_Boundaries` (22 subtests total) cover
missing/not-done/done targets, `where` filtering, inclusive bounds at exactly
min and exactly max, and both bounds set. Loader parse test confirms the field
populates.
2. **Loader rejects unknown keys within a validation rule** — PASS.
`TestParse_UnknownValidationRuleKeyRejected` (a `relationz` typo fails and the
error names both the key and the rule) and
`TestValidValidationRuleKeysMatchStruct` (the allowlist cannot drift from the
struct tags). The rebase proved this test's worth: it caught three new upstream
fields (`faces`, `when_condition`, `then_condition`) that the allowlist would
otherwise have rejected.
3. **14 gates declarative; `rela validate` output identical to the Lua
baseline** — PASS, verified empirically rather than by inspection. Built the
pre-migration commit's binary with the Lua validator restored and ran both
against the same ticket corpus: byte-identical output. Also compared on a
deliberately-violating fixture (a `done` ticket with its `has-review` relation
removed): both flag the same entity with the same error count. The only
difference is presentation — the native version uses the rule's `description`
as the headline and carries `requires at least 1 'has-review' (status=done)
relation(s), has 0` as structured `Detail` (confirmed in JSON output), where the
Lua put the mechanical string in the headline. Re-verified after the review
fixes: parity still holds.
4. **Conformance + strict-loader tests added; `just ci` green** — PASS. See
Automated Checks and Pull Request below.

Regression check on the new strictness: scanned every `schema.yaml` /
`metamodel.yaml` in the repo; no false rejections.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

`docs/metamodel.md` gains a "Relation Cardinality Validation" section with
worked `min` and `max: 0` examples, a field table, the load-time and check-time
failure rules, the `where` type-coercion semantics, and the visibility scope.
The same content is in the source entity
`docs-project/entities/guides/GUIDE-metamodel.md`, and `just docs-check` passes,
so the generated docs and the entity graph are in sync.

**Docs Checklist:** DOCS-KM8O68

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1571

Local `just ci` components all verified green on the pushed tree: `lint` 0
issues, `arch-lint` OK, `lint-md` 0 issues, `go test ./...` 0 failures,
`coverage-check` PASS (total 79.4%), `build` all binaries, `docs-check` up to
date. They were run individually rather than as one `just ci` invocation
because a concurrent session on this machine held the golangci-lint lock and
triggered OOM kills; each step's result above is from a completed run on the
final tree.
