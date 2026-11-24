---
id: RR-JA8RW45
type: review-response
title: Per-key lock map grew without bound
finding: The service added one mutex per owner and list and never removed it.
severity: minor
resolution: Fixed array of 64 striped mutexes.
status: addressed
---
