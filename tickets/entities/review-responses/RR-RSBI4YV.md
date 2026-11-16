---
id: RR-RSBI4YV
type: review-response
title: Acceptance 3 is a one-off manual check
finding: A new trigger would go uncovered silently.
severity: minor
resolution: Each case is named after the trigger it covers; a Go test reads tickets/flows.yaml trigger keys and the case names in the Lua file and fails on any uncovered trigger.
status: addressed
---
