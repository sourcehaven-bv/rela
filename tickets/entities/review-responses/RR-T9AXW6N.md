---
id: RR-T9AXW6N
type: review-response
title: Migration re-run after a crash can duplicate an unnamed trigger
finding: 'projectsetup automations_to_flows names an unnamed automation by its global position. If the run crashes after cutting the root but before an include, the re-run renumbers the include''s entries and appends a duplicate trigger. Fix: cut includes before the root.'
severity: minor
resolution: moveAutomationsToFlows cuts includes first and the root last, so a re-run gives uncut entries the same names and finds them in flows.yaml. TestMigrate_CrashRecovery has a case per crashed write.
status: addressed
---
