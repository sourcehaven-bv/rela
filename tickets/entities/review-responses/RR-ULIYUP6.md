---
id: RR-ULIYUP6
type: review-response
title: renameTriggerReferences misses create, notify and anchors filters loosely
finding: create.properties of the renamed type, notify.to.property and renameFilter replacing the first substring.
severity: minor
resolution: Renames create.properties keys when create.type is the type, notify.to.property on the type's triggers, and anchors the filter rename at the start of the clause. Test in configDraft.test.ts. Triggers without on.entity are left for the save to report.
status: addressed
---
