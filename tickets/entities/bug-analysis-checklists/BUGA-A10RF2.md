---
id: BUGA-A10RF2
type: bug-analysis-checklist
title: 'Analysis: analysis.faceDeclared treats a bare row as always-declared on a faced type'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced — in-tree, not synthetically. `prototypes/perf/project`
seeded with `rela dev seed --scale 0.01`; `perfseed` writes every policy and
document at the bare coordinate while both types declare `faces:`. `rela analyze
states` reported **no findings** against 35 stranded rows.
- [x] Reproduction is deterministic — the seed is fixed, so the row set is
stable run to run.
- [x] Minimal case isolated: `faceDeclared(type, "")` returns `true` for a type
declaring `faces:`.

## Root cause

- [x] Root cause identified: `analysis.faceDeclared` returned `true`
unconditionally for the zero face, so a bare row was treated as declared for
every type.
- [x] Why the code was written that way: it was correct when written. Before
BUG-HC6I2T a named face required a zero-coordinate sibling, so every entity
genuinely had a declared bare state. The doc comment said so explicitly — "every
entity has one by construction (the bare id addresses it)".
- [x] 5-whys completed to a systemic cause (recorded on the bug). The sharp
point: BUG-HC6I2T deleted the `headless-family` finding, which reported a
**missing** bare row, but did not revisit the predicate assuming a **present**
one is always fine. Same fact, opposite directions, one updated.

## Scope

- [x] Blast radius assessed: detection only. `CheckStates` reads raw storage
truth and reports; it writes nothing and gates nothing, so the fix cannot change
stored data or any read path.
- [x] Related surfaces checked: `rela acl audit` already reports
`B12-bare-grant-on-faced-type`, the ACL-side mirror of this. Its existence is
evidence the data-side gap was an oversight rather than a decision.
- [x] Two stale references to the removed `headless-family` finding found and
fixed. One, in the published guide, told operators `analyze states` reports
"face rows whose bare row is missing" — the opposite of the truth.

## Fix plan

- [x] Fix chosen: the predicate asks the type. The zero face is declared only
where the type declares no `faces:`.
- [x] Alternative rejected: drop the `IsDefault()` early return entirely. That
swings the error the other way and reports every entity of every faceless type
in the project, which is most of them.
- [x] New finding code `bare-row-on-faced-type` rather than reusing
`undeclared-face`: the latter's `Subject` would be the empty string, printing
`face ""`, and the remedy differs — a named undeclared face wants a rename or
delete, a bare row wants adopting into a face via `migrate_face`.
- [x] Regression test planned and mutation-verified in both directions
(AM-analyze-reports-bare-row-on-faced-type).
