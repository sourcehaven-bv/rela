---
id: RR-MZFM3L
type: review-response
title: Desktop does not wait for the lease watcher before Close
finding: 'Code review: the done channel is discarded and the store closes right after cancel.'
severity: minor
resolution: 'Fixed in f9bddab4 (Lease service: review fixes) with a test; see the commit.'
status: addressed
---
