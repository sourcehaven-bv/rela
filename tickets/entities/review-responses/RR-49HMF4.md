---
id: RR-49HMF4
type: review-response
title: Unused-key check counts keys mentioned in comments
finding: The substring search covered comments, so keys named in doc comments could never be reported unused.
severity: minor
resolution: guards.test.ts strips block and line comments before searching.
status: addressed
---
