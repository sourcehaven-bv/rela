---
id: RR-4THRADO
type: review-response
title: JSON round trip changes number types
finding: Plain json.Unmarshal turns ints into float64 in the deleted state.
severity: minor
resolution: Decode with UseNumber and normalize to int or float64. Tested in TestAutomationJobs_DeletedPayload.
status: addressed
---
