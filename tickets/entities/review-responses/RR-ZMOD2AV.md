---
id: RR-ZMOD2AV
type: review-response
title: Unscoped handle could replace the scoped update_entity
finding: With both ElevatedManager and ElevatedUpdater set, the unscoped update_entity overwrote the scoped one.
severity: minor
resolution: The scoped binding registers after the unscoped writes, so the narrower one wins.
status: addressed
---
