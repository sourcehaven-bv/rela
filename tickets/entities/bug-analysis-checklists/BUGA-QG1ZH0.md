---
id: BUGA-QG1ZH0
type: bug-analysis-checklist
title: 'Analysis: Data-entry migrations leak a metamodel across rela migrate runs'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestDetectFromNodeWithMetamodel_DoesNotLeakMetamodelAcrossRuns fails on the pre-fix commit 8d77eb1d5)
- [x] Minimal reproduction steps documented (run data-entry detection with a metamodel, then without one; the second run reports "using metamodel")
- [x] Environment/conditions noted (only under `go test -shuffle=on`, when a test that passes a metamodel runs before TestDetectMigrationsWithFS_UnloadableSchemaUsesNoMetamodelPath in the same process)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (MetamodelAware.WithMetamodel returns a per-run copy; the runner always binds, including nil)
- [x] Regression test planned (internal/migration/runner_test.go)
- [x] Related areas checked for similar issues (RelationDirectionMigration is the only other MetamodelAware implementer and gets the same change; automation.Engine.SetMetamodel is per-engine, not a registry singleton)
