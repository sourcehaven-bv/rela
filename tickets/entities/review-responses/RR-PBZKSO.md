---
id: RR-PBZKSO
type: review-response
title: Tombstone tests check a copy of the catch-up query
finding: changesSince hand-copies the catch-up SQL without its face handling, so it can drift from listener.catchUp.
severity: minor
resolution: The helper comment now says it checks what the writes record, not what the catch-up emits, and names TestCatchUpRecoversMissedDelete as the real-path test.
status: addressed
---
