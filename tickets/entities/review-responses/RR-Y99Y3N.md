---
id: RR-Y99Y3N
type: review-response
title: Typed-nil test does not exercise the bug
finding: TestLoadMigrationMetamodel only checks the helper. The bug was a nil *Metamodel inside the interface reaching DataEntryCleanupMigration and panicking on a form field with widget/required/default.
severity: minor
resolution: 'Added TestDetectMigrationsWithFS_UnloadableSchemaUsesNoMetamodelPath (schema ''entities: 5'' plus a form field with widget). It panics with the guard removed and passes with it.'
status: addressed
---
