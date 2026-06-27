---
id: RR-FA1F
type: review-response
title: e2e timing -- fine, but call it out
finding: |
  Reviewer verified: e2e/tests/checkboxes.spec.ts uses expect.poll with 5s timeout for server state and 2s for UI state. 100ms debounce sits comfortably inside both. No timing assertion tighter than 2s. Safe.
severity: minor
status: addressed
resolution: |
  AC 8 amended: add note "checkboxes.spec.ts uses 2-5s poll timeouts; 100ms debounce is within tolerance" so the next reader doesn't re-derive this. Plan row "passes unchanged" is correct.
---
