---
id: BUGA-YCYAWA
type: bug-analysis-checklist
title: 'Analysis: pgstore silently substitutes U+FFFD for invalid UTF-8 where fsstore refuses'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Reproduced through the conformance suite rather than by hand, because the
defect is a DIVERGENCE and a single backend cannot show one. A new
`RejectsInvalidUTF8Properties` case in `storetest.RunValidationTests` and
round-trip seeds in `FuzzPropertyValuesTypeZoo` were written first and run
against all four backends with the fix reverted:

| backend | `"\n\xc80"` on create | what the caller got |
| --- | --- | --- |
| fsstore | error | refused (yaml: cannot marshal invalid UTF-8) — correct |
| pgstore | success | `"\n�0"` — silently substituted |
| sqlitestore | success | `"\n�0"` — silently substituted |
| memstore | success | the invalid bytes verbatim |

Three backends, three different answers, and only one of them an error.
pgstore ran against a local Postgres (`RELA_TEST_DATABASE_URL`); the other
three need nothing.

The bug entity's parked seed (`"\n\xc80"`) is now an `f.Add` seed in the
shared target, so all four backends carry it, rather than a `testdata` file
under one of them.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

Recorded on the bug (`why1`–`why4`, `prevention`). The short form: no shared
rule said what property TEXT a store must refuse. IDs had one
(`storeutil.ValidateID`, the fuzz oracle); text values did not, so each backend
inherited its serializer's opinion — YAML refuses, `encoding/json` substitutes
U+FFFD without an error, memory keeps whatever it is handed. And the fuzz
target asserted only "write did not error", which the substituting backends
satisfy.

`why5` deliberately stops at four. The systemic cause (validity defined by
the serializer instead of by the store contract) is already the one
`prevention` addresses; a fifth level would be a restatement.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

**Approach.** `storeutil.ValidateProperties`, next to `ValidateID`: valid
UTF-8 and no NUL, in every key and string value at any nesting a property
value can take. Called from all four backends' entity and relation
create/update paths. One rule, eight call sites, the same arrangement the ID
grammar uses — not a fix inside the two lax backends, which is how
BUG-B1RA3J's `valueToNode` duplication came about.

**Why NUL joined the rule.** The bug is about invalid UTF-8, and the first
cut checked only that. The fuzz round-trip then found the next divergence in
under a second: Postgres cannot hold U+0000 in text or jsonb, so pgstore
alone refused it — loudly, at least — while the other three stored it. A NUL
is never legitimate property text and the fix is one more line in the same
rule, so it was widened rather than filed. The alternative was a fuzz target
that skips NUL, which hides a real divergence.

**Regression test.** The `RejectsInvalidUTF8Properties` conformance case
(create, update, relation create, relation update, plus a valid non-ASCII
value that must survive untouched), `storeutil` unit tests per nesting shape,
and the fuzz seeds. Mutation-verified: with only the storeutil call sites
reverted, all four backends fail the new cases.

**Related areas.** The round-trip assertion — compare what was read to what
was written, per the `yaml-roundtrip-property-test` measure — turned up four
more pre-existing divergences on its first runs. Each was either fixed or
filed, none skipped silently:

- A property KEY beginning with a newline, and a key yaml.v3 reads as null
  (`~`), were lost by fsstore — the BUG-B1RA3J defect on keys. Fixed on that
  ticket's branch (`markdown.KeyNode`), which this branch is stacked on.
- A clip block scalar as the LAST frontmatter property lost its trailing
  newline on fsstore: `frontmatter.Split` dropped the final line terminator.
  Filed as BUG-NWQA0E and fixed here (one line plus a test), because the
  round-trip assertion cannot ship red and skipping trailing newlines would
  hide data loss.
- Only sqlitestore applies `storeutil.ValidateProperty` (empty name, slash)
  in `PropertyValues`; the others accept. Filed as BUG-CQYD5X; the fuzz
  skips such names until that is decided.
- A negative `valueType` in the fuzz target matched no `switch` case, so the
  entity went in with no property at all. Fixed in the target.

pgstore and sqlitestore also snapshot properties into version tables through
the same `marshalProps`, but only from values that already passed the write
gate, so no second call site is needed there.
