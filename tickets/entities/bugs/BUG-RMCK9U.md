---
id: BUG-RMCK9U
type: bug
title: 'DUPLICATE of BUG-NEQRY2: rela validate exits 0 when frontmatter cannot be parsed'
description: An entity whose YAML frontmatter fails to parse is skipped with a WARN by store.ListEntities, but rela validate still reports 'All validations passed' and exits 0. CI therefore cannot catch a malformed entity, and every automation run fails at 'collect existing IDs', silently skipping checklist creation repo-wide.
priority: high
effort: s
status: wont-fix
---


> **Duplicate of BUG-NEQRY2 — closed in its favour.**
>
> The same defect was filed twice, three days apart, by two people who each
> tripped over it while doing something else. That is itself evidence for the
> bug: it is only ever found by accident.
>
> BUG-NEQRY2 is the surviving ticket. It has the fuller write-up — the causal
> chain showing how the silent under-count hid a merged-`done` bug with no
> review record, and the comparison against the earlier `AM-date-property-
> write-roundtrip` occurrence that failed *loudly* and so was fixed at once.
> That asymmetry is the argument for failing closed, and it is worth keeping.
>
> The reproduction below is still accurate and still reproduces; it is left in
> place rather than deleted so anyone landing here from a search sees the
> evidence rather than a redirect.

## Reproduction

1. Introduce an unquoted `: ` inside a plain-scalar frontmatter value on any
entity, e.g. a description containing the words `the where: clause`.
2. Run the exact command CI runs:

```console
$ cd tickets && rela validate --check cardinality --check properties --check validations
...
level=WARN msg="appbuild: failed to index entities" error="... failed to parse frontmatter: yaml: line 4: mapping values are not allowed in this context"
...
level=WARN msg="analysis: store.ListEntities iterator error; results may under-count"
✓ All validation rules passed

All validations passed.
$ echo $?
0
```

**Expected:** non-zero exit — an entity that cannot be read is not an entity
that passed validation.

**Actual:** exit 0. The malformed entity is silently dropped from the result
set, so every check that follows validates a *subset* of the graph and reports
success on it.

## Why this matters

The failure is not confined to reporting. Any subsequent write fails at `collect
existing IDs`, which is the ID-uniqueness scan — so **automations stop firing**:

```console
⚠ Automation error: failed to create automation entity planning-checklist:
  collect existing IDs: failed to parse frontmatter: yaml: line 4: ...
```

A ticket moved to `planning` therefore gets **no planning checklist**, and the
`planning-ticket-needs-checklist` rule then flags it as incomplete — the symptom
appears on an unrelated, valid ticket, far from the malformed file.

Observed live: `AM-feed-field-redaction.md` (merged in #1314) carried an
unquoted `where:` and blocked checklist creation for TKT-7S5735. CI on #1314 was
green.

## Cause

`store.ListEntities` treats a parse failure as a per-item skip and surfaces it
through an iterator error that callers log as a WARN rather than propagate.
`rela validate` aggregates check results but never consults that error, so
"nothing to report" and "could not read the input" are indistinguishable in its
exit code.

## Suggested fix

Make the read error fatal to `validate`: if the entity iterator reports any
parse failure, print the offending path and exit non-zero. This is the
`--strict` distinction applied to *unreadable input* rather than soft warnings —
a file that cannot be parsed is a hard error, not a warning, because every
downstream count silently under-reports.

Consider also failing the write path loudly rather than degrading to a skipped
automation.
