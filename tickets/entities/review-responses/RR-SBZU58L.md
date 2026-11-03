---
id: RR-SBZU58L
type: review-response
title: Neighbouring-slot merge is unverified
finding: Show in a unit test that node-diff3 merges edits to slots separated by one unchanged line; otherwise the merged-writes floor measures nothing.
severity: minor
resolution: A unit test in the property file shows node-diff3 merges edits to neighbouring slots cleanly; the merged-writes floor is met.
status: addressed
---
