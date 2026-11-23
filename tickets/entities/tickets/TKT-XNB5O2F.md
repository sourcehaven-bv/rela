---
id: TKT-XNB5O2F
type: ticket
title: Remove remaining dead code and dedupe helpers
kind: chore
priority: low
effort: s
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

## Description
Remove 14 more unused functions found by deadcode. Replace five dedupe helpers
with slices.Compact(slices.Sorted(...)); no test depended on first-seen order.
