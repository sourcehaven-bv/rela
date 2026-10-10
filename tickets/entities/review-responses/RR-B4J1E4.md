---
id: RR-B4J1E4
type: review-response
title: Migration starting point undefined
finding: From=stored breaks when no record exists, when the record is already NeedsMigration, or when unapplied files exist (resolve.go:52-60).
severity: significant
resolution: 'Plan changed: preview runs the gate first and refuses unless InSync/Adopted/Bootstrapped with nothing pending; Bootstrapped uses the pre-edit projection; the orchestration in cli/migrate_data.go:230-330 moves into datamigration and is shared.'
status: addressed
---
