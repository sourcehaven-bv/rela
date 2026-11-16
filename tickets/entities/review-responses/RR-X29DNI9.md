---
id: RR-X29DNI9
type: review-response
title: Hard cut would break the migration's own metamodel load
finding: loadMigrationMetamodel uses LoadWithoutMigrationCheck; refusing automations there gives per-file migrations a nil metamodel.
severity: significant
resolution: 'Plan: the cut is a Blocking detection (automations-to-flows; project-level so the per-file runner skips its Apply) checked by FSLoader.Load and for includes; LoadWithoutMigrationCheck ignores the key.'
status: addressed
---
