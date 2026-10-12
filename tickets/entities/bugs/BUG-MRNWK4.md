---
id: BUG-MRNWK4
type: bug
title: Data-entry migrations leak a metamodel across rela migrate runs
description: 'TestDetectMigrationsWithFS_UnloadableSchemaUsesNoMetamodelPath fails under -shuffle=on: the data-entry cleanup migration reports ''using metamodel'' when the schema cannot load. The runner stored the metamodel on the registered migration singleton and skipped the call when it was nil.'
priority: medium
effort: xs
why1: The data-entry cleanup migration still held a metamodel from an earlier run in the same process.
why2: DetectFromNodeWithMetamodel called SetMetamodel on the registered migration instance and only when meta was non-nil.
why3: Registered migrations are process-wide singletons and the MetamodelAware interface bound context by mutating them.
why4: Per-run state was put on a shared registry object; nothing reset it and concurrent runs raced on it.
why5: Test order hid it until CI enabled -shuffle=on; the interface shape allowed shared mutable state.
prevention: MetamodelAware.WithMetamodel returns a per-run copy so the registry is never mutated; runner_test pins that a metamodel does not leak into a later run.
status: ready
---

Fix: bind the metamodel to a per-run copy via WithMetamodel instead of
SetMetamodel on the shared registry.
