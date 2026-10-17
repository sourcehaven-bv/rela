---
id: RR-ESHR4I
type: review-response
title: Query ceiling applied to writes
finding: Imports and Lua commands may legitimately run many statements once, and would trip the e2e ceiling check.
severity: minor
resolution: overQueryCeiling applies only to GET and HEAD; a POST test case pins the exemption. Docs and flag help updated.
status: addressed
---
