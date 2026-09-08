---
id: IMPL-O4URV0
type: implementation-checklist
title: 'Implementation: pgstore silently substitutes U+FFFD for invalid UTF-8 where fsstore refuses'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

| where | what |
| --- | --- |
| `internal/store/storeutil/props.go` | `ValidateProperties`: valid UTF-8 and no NUL in every key and string value, walking `[]string`, `[]any`, `map[string]string`, `map[string]any` at any depth. Errors name the offending path (`property "p[1].v"`). |
| fsstore, memstore, sqlitestore, pgstore | Called at the top of entity create/update and relation create/update — before any lock, transaction, or serialization, so a refused write touches nothing. |
| `storetest/validation.go` | `RejectsInvalidUTF8Properties`: create, update, relation create, relation update; each refusal checked to have persisted nothing; a valid non-ASCII value checked to survive untouched. |
| `storetest/fuzz.go` | `FuzzPropertyValuesTypeZoo` now applies the directional oracle (rejected by the rule ⇒ store must reject) and round-trips every accepted value, comparing the whole property map. Seeds for both bugs and NUL. |
| `internal/frontmatter` | BUG-NWQA0E: the last frontmatter line keeps its terminator, so a final block scalar reads back whole. Found by the new assertion. |

"Integration" here is the conformance suite: one test body, four real
backends, pgstore against a live Postgres.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The fuzz round-trip compares against `want := e.Clone()` taken before the
write, through a normalizer that maps `[]string` to `[]any` so a typed write
and a JSON/YAML read compare by content. The conformance case reads the
stored value back after each refused write and compares it with the value
written before it, not with a literal.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

| step | observed |
| --- | --- |
| new tests with the wiring reverted (mutation) | fsstore 1, memstore 2, sqlitestore 2, pgstore 2 failing subtests |
| new tests with the wiring in place | all four pass |
| `go test ./internal/store/... ./internal/frontmatter/ ./internal/markdown/ ./internal/analysis/` | ok, pgstore on `RELA_TEST_DATABASE_URL` |
| `FuzzPropertyValuesTypeZoo`, 45s per backend, after the last change | no findings on any of the four |
| `FuzzSplit`, 20s | no findings |

The fuzz runs before that last one are the interesting part. Each time the
round-trip assertion was tightened it found something within a few seconds:
BUG-B1RA3J on develop (expected — that fix is on the branch this one stacks
on), then keys with a leading newline, then NUL on pgstore, then the
trailing-newline loss in `frontmatter.Split`, then the `~` key, then the
negative-modulo hole in the target itself. All recorded in BUGA-YCYAWA under
"Related areas". A target that only checked for an absent error had passed
over every one of them.

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

Pattern: `storeutil.ValidateID` — shared rule, per-backend call site, fuzz
oracle. `checkText` is the one place the rule is stated; the walker only
decides what to name in the error. sqlitestore wraps the error with its
`sqlitestore: create:` prefix like its other validations; the other three
return it as-is, as they do for `ValidateID`.

Security: this closes a silent-substitution path. A caller could previously
store bytes that read back as different bytes on one backend and as an error
on another; a value's validity no longer depends on the deployment.
