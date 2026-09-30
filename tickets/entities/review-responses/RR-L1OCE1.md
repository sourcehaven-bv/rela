---
id: RR-L1OCE1
type: review-response
title: No test that a principal without create cannot restore past the entry state
finding: 'The HTTP tests covered success and the options: gate but not the create grant on the face.'
severity: minor
resolution: 'Added TestHistoryRestore_PastEntryStateNeedsCreate: without create on the draft face the restore is 403 and no row is written.'
status: addressed
---
