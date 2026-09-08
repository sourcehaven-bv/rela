---
id: IMPL-YQO4HK
type: implementation-checklist
title: 'Implementation: Document why rela import bypasses transition guards'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Documentation only — a godoc on `importEntity`. No behaviour changed.

Marking the test boxes checked with no tests added needs justifying rather than
waving through: there is no behaviour to test. A test asserting "import bypasses
guards" would pin the CURRENT design as if it were a requirement, making the
deliberate exception harder to revisit than the code it documents — the opposite
of this ticket's intent.

What the comment DOES is state three reasons weighed together, because each
alone is weaker than the set: import loads states that already exist; the
importer is CLI-only; a guard is a speed bump against someone who already has
store access.

It also records what would CHANGE the answer — a non-CLI caller — so the
decision fails loudly rather than being silently inherited.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

N/A — no tests added, for the reason above.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

The comment asserts facts about OTHER files. A confidently-wrong comment gets
trusted, so each claim was checked rather than assumed:

| claim | command | result |
| --- | --- | --- |
| the importer is CLI-only | `grep -rn "importer.New(" --include="*.go" . \| grep -v _test \| grep -v ^./internal/importer/` | one hit: `internal/cli/import.go:28` |
| `create` / `restore` go through EntityManager | `grep -rn "CreateEntity\|UpdateEntity" internal/cli/*.go \| grep -v _test` | `create.go:54`, `restore.go:61,68` all `svc.EntityManager.*` |
| `normalize` cannot change status | `grep -n "status\|Status" internal/cli/normalize.go` | no matches |
| `importEntity` writes to the store | read `internal/importer/importer.go:423,427` | `imp.store.UpdateEntity` / `CreateEntity` |

The third row is the one that saved the argument from being sloppy. The
intuitive summary — "the CLI writes directly anyway" — is FALSE: two of the
three other entity-writing CLI paths are guarded, and the third cannot change
state. Import is a single deliberate exception, not a general posture, and the
comment now says so.

Gates: `just lint` 0 issues, `just comment-lint` no unresolvable doc links
across 11461 comments, `just plimsoll` clean, `go test ./internal/importer/` ok.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

DRY: N/A — a comment.

Security: no code changed. The security-relevant artefact is the decision being
recorded, so the next reviewer evaluates the reasoning rather than rediscovering
the absence — which is what happened here, and why the issue came from an
external review.

Stated plainly in the comment, because it is the part most easily
over-generalised: this is NOT a claim that unguarded import is harmless in
general. It is a claim that the guard defends nothing against the only actor who
can reach this code. Naming the invalidating condition is what keeps that
honest.

The divergence from sync (RR-NB135) is documented as deliberate. An undocumented
inconsistency between two write paths is exactly what invites the next review
finding, so the comment explains the difference: sync is an ongoing channel
carrying a peer's new transitions; import is a one-shot operator load of
historical fact.
