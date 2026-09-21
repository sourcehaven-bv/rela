---
id: migration-reports-what-it-rewrote
type: automated-measure
title: 'Test: a migration step reports every kind of row it rewrote, not just its headline kind'
description: A migrate_face run that destroyed 245 relations and one that carried all 854 produced byte-identical output, both exiting 0. The step counted entities because entities were what it named; relations were rewritten invisibly. Any step whose blast radius exceeds its headline kind must report the other kinds too, and a test should assert the count appears — otherwise the next silent data-loss defect is discovered the same way this one was, by counting rows in SQL afterwards.
kind: test
location: internal/datamigration/migrateface_tx_test.go
status: proposed
---

## Why

A migration step names one kind of thing — `migrate_face` names entities — and
reports its work in those terms: `changed 61 record(s)`. But moving an entity
between faces rewrites every outgoing edge it owns.

That gap is not cosmetic. Measured on a real dataset:

```
develop (pre-fix):  "changed 61 record(s)"  relations 854 -> 609   (245 destroyed)
PR 1627 (fixed):    "changed 61 record(s)"  relations 854 -> 854
```

**Identical output, exit 0 both times.** The destructive run was
indistinguishable from the correct one, and the loss surfaced only because
someone counted rows in SQL before and after.

## What it pins

For a migration step whose blast radius exceeds the kind it names, the run
output reports the other kinds too, and a test asserts the reported count
matches what was actually rewritten. A step that carries N edges must say N.

The sharper form, worth considering at the same time: the step asserts
conservation itself (edges in == edges out) and fails loudly. A tool that can
detect its own data loss should not delegate that to the operator.

## What this is NOT

Not a substitute for the fix. BUG-TOX8U4 stopped the destruction; this stops the
*invisibility*. The two are independent: a future step can be correct and
silent, or buggy and loud, and only the second gets caught early.

## Generalisation

The rule to carry to a new step: **report what you rewrote, not what you were
named after.** A step called `migrate_face` that also rewrites relations,
attachments or versions owes a count for each. The failure mode this guards
against is not a wrong number — it is a number that looks complete and is not.
