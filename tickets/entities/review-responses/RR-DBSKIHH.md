---
id: RR-DBSKIHH
type: review-response
title: 'Design: local rename looks like a delete'
finding: The ledger keys on the local id, so a rename made the row look deleted.
severity: significant
resolution: 'Plan: a missing local id is looked up by the external ref before the row counts as dropped.'
status: addressed
---
