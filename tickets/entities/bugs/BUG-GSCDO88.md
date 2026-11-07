---
id: BUG-GSCDO88
type: bug
title: 'Flow subject or create type given as an alias skips the visible: check'
description: 'flowcfg resolves an entity type alias through GetEntityDef but keeps the alias as the flow subject and as a create output type. FieldRestricted compares grant keys exactly, so subject: pers (an alias of person) lets a guard (when: entity.salary > 3) or a template ({{subject.salary}}) read a property a visible: grant restricts, while subject: person is rejected. GitHub #2106.'
priority: high
effort: s
why1: loader.subject checks the type with GetEntityDef, which resolves aliases, and stores the raw name. FieldRestricted keys on the canonical name, so it finds no grant for the alias and the guard or template is accepted.
why2: The loader validated the type for existence only. It never normalized or rejected the name it went on to use as a key.
why3: GetEntityDef silently resolves aliases, so a successful lookup looks like proof the name is canonical. Tests used only canonical names and the test schema declared no aliases.
why4: Each config loader repeats the alias rule by hand. realtimecfg added it after a review; flowcfg was written without that check carried over.
why5: There is no shared helper for 'a config names an entity type', and no test schema with aliases in the flowcfg harness, so the rule depends on each author remembering it.
prevention: The flowcfg test schema now declares aliases and TestParse_Problems pins that subject and create.type reject them, so a later type-name field in the same harness can be tested against an alias at no cost.
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

## Reproduction

In `internal/flowcfg/testdata`, declare `aliases: [pers]` on `person` and change
the flow to `subject: pers` with a template `{{subject.salary}}`. `Parse`
accepts the file. With `subject: person` it reports the `visible:` grant.

## Root cause

`loader.subject` checks the type with `GetEntityDef`, which resolves aliases,
and stores the raw name. `FieldRestricted` keys on the canonical type name, so
the lookup for the alias finds no grant. `createOutput` has the same gap for
`create.type`.

## Fix

Reject an alias in `subject` and `create.type`, as `internal/realtimecfg` does:
the file must name the type itself.
