---
id: RR-7KJOAK
type: review-response
title: Data gate verdict is stale after a save's migration
finding: The candidate evaluated the data gate before migrating, so its GC verdict described the pre-migration data.
severity: significant
resolution: appbuild.ReevaluateDataGate refreshes the verdict after the migration.
status: addressed
---
