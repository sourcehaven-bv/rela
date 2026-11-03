---
id: RR-0O7MKT7
type: review-response
title: No case that Previous survives non-moving writes
finding: bumpSession and the file meta rewrite every session column on each append; a forgotten mapping would blank Previous on any non-moving post, which the planned cases would not catch.
severity: significant
resolution: 'SessionPreviousKept conformance case on every backend: a non-moving post, a reminder step, and close then reopen leave Previous unchanged.'
status: addressed
---
