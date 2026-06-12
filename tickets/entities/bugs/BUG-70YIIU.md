---
id: BUG-70YIIU
type: bug
title: 'Metamodel validation: fractional integers truncated, empty relation from/to allowed'
description: 'Two metamodel validation gaps. (1) An integer property accepted any float64 unconditionally and ParseIntegerValue did int(v), so a hand-edited ''count: 3.5'' validated and silently truncated to 3 instead of erroring. (2) validateRelationReferences only checked that listed from/to types exist, so a relation declaring empty from: or to: loaded cleanly — making any min/max cardinality constraint on it a silent no-op (a typo''d or half-written relation passes).'
priority: medium
why1: The integer validation switch had a bare 'case int, int64, float64' with no integral check, and ParseIntegerValue truncated via int(v); validateRelationReferences never checked len(rel.From)/len(rel.To).
why2: YAML parses bare integers as int but a fractional literal as float64, and the float64 case was added to tolerate that without distinguishing 3.0 from 3.5; the relation check focused on unknown-type references and didn't consider the empty case.
why3: Both validators handled the present/known cases and silently accepted the malformed-but-parseable ones rather than rejecting them.
why4: No test fed a fractional integer or an empty-from/to relation, so both gaps stayed latent.
why5: Schema validation had no convention of 'reject malformed-but-parseable at load' — each rule decided how strict to be ad hoc.
prevention: Integer validation and ParseIntegerValue now reject a float64 with a fractional part (v != math.Trunc(v)); integral floats (3.0) still accepted. validateRelationReferences rejects a relation with len(from)==0 or len(to)==0 at load. Regression tests pin fractional rejection (validation + parse helper) and empty-from/to rejection (with a populated-relation guard against over-rejection); all fail without the fix. Full suite confirms no in-tree metamodel relied on either gap.
status: done
---

## Bug

Found in the 2026-06-09 backend review (Minor / Read-query). Two metamodel
validation gaps:

1. **Fractional integers silently truncate.** The integer property-type case (`validation.go`) accepted `float64` unconditionally, and `ParseIntegerValue` did `int(v)`. A hand-edited `count: 3.5` (YAML parses a fractional literal as `float64`) validated as integer **3** instead of erroring — silent data corruption on a typo.

2. **Empty relation `from:`/`to:` allowed.** `validateRelationReferences` (`loader.go`) only checked that *listed* from/to types exist. A relation with `from: []` (or an omitted field) loaded cleanly, but no entity can ever be a valid endpoint — so any `min_outgoing`/`max_outgoing` cardinality constraint on it is a silent no-op. A typo'd or half-written relation passed validation.

## Fix (PR pending)

- Integer validation + `ParseIntegerValue`: reject a `float64` with a fractional part (`v != math.Trunc(v)`) with a clear message; integral floats (`3.0`) still accepted.
- `validateRelationReferences`: reject a relation with `len(from)==0` or `len(to)==0` at load time.

The full test suite confirms no in-tree metamodel (`tickets/`, `docs-project/`,
fixtures) relied on either gap, so neither change breaks existing projects.

## Tests

`internal/metamodel/validation_gaps_test.go`: fractional rejection at both the
property validator and the parse helper (with `3.0`/`7.0` accepted), and
empty-`from`/empty-`to` rejection (plus a fully-populated-relation guard against
over-rejection). All verified to **fail without the fix**.
