---
id: RR-9UZVY9
type: review-response
title: Pull cost on large accounts
finding: Every pull did per-to-do lookups and full merges, so large accounts may never finish.
severity: significant
resolution: One listing maps refs to todos; a to-do unchanged on both sides since the last pull (cached updated_at and version token) is skipped without a merge. Keeping pages across a 429 is left for later; the README describes the cost.
status: addressed
---
