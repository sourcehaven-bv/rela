---
id: BUG-2XN24C
type: bug
title: Incoming form relation with no declared inverse throws at submit
description: 'A form declaring an incoming relation whose type has no declared inverse throws an unhandled exception at submit: the serializer keys incoming edges under a synthesized relType_inverse name, the SPA getInverseName returns undefined for it, and buildRelationsPatch throws. Config validation never checks that the inverse exists. Pre-existing; found while reviewing BUG-KQSOJ2, which neither introduces nor worsens it.'
priority: medium
effort: s
status: backlog
---

## Description

A form declaring an incoming relation whose type has no `inverse:` in the
metamodel throws an unhandled exception at submit.

Pre-existing; found while reviewing BUG-KQSOJ2, which did not introduce or
worsen it.

## Mechanism

Three places disagree about the inverse name:

- Config validation (`internal/dataentryconfig/validate.go` `validateFormRelation`)
validates direction inference and side, but never that a declared inverse
exists.
- The serializer falls back to `relType + "_inverse"`
(`internal/dataentry/relations_direction.go`), so the GET keys incoming edges
under that synthesized name.
- The SPA's `schemaStore.getInverseName` returns `undefined` for a relation with
no declared inverse, so `buildRelationsPatch` hits its `no inverse declared in
metamodel` throw — whose message says "DynamicForm should have pre-flighted
this", but no such pre-flight exists.

## Options

Either add the missing pre-flight in DynamicForm (refuse the form at load, the
direction CLAUDE.md prefers for a dropped-constraint case), or validate at
config load that an incoming form relation names a type with a declared inverse
— cheaper, and it puts the error in front of the operator who wrote the config.
The serializer's `_inverse` fallback should probably go at the same time, since
nothing else can compute that name.
