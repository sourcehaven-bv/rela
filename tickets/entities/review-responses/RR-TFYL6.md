---
id: RR-TFYL6
type: review-response
title: AC numbering grew to 12; verify Test Plan table is in sync after edits
finding: 'Cosmetic: plan was rewritten v1→v2 and ACs grew from 10 to 12. Test Plan table currently maps AC1–AC12. Confirm coherence after design-review edits land.'
severity: nit
resolution: AC list rebuilt to 20 rows; Test Plan table regenerated to match 1:1. Edge Cases section trimmed to non-AC-overlapping items only.
status: addressed
---

# Finding

Cosmetic. Plan was rewritten v1→v2 and ACs grew from 10 to 12. The Test Plan
table currently maps AC1–AC12. Easy to drift after design-review edits.

# Resolution

After applying design-review changes, do one read-through to confirm:

- Each AC has a row in the Test Scenarios table.
- Each row references concrete code locations or input/output samples.
- Edge Cases don't silently duplicate AC tests.

No change required if it's already consistent.
