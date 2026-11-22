---
id: RR-1VFX0Q2
type: review-response
title: "Brittle test helpers and stale test name"
finding: "The classes() equality assertion breaks on unrelated classes, and TestValidateConfig_PrimaryColumnOnlyOnLists no longer matched its cases."
severity: nit
resolution: "Addressed: the test checks the named classes with toContain, and the Go test is TestValidateConfig_PrimaryColumnPlacement. The td fallback in EntityDetail.table.test.ts stays, since read cells carry the value wrapper."
status: addressed
---
