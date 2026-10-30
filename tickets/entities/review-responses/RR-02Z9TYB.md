---
id: RR-02Z9TYB
type: review-response
title: Snapshot update run fails against itself
finding: With CONTRACT_UPDATE=1 the file was rewritten but compared with the old imported JSON.
severity: minor
resolution: The update run writes the snapshot and returns; the next run compares.
status: addressed
---
