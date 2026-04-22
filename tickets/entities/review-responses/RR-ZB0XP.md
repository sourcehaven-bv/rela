---
id: RR-ZB0XP
type: review-response
title: Distinguish testable vs. inspected acceptance criteria
finding: AC8 (guide present) and AC10 (FEAT-023 updated) are documentation acceptance criteria verified by human review. Mixing them alongside unit-testable ACs in one numbered list is confusing. Split into two categories or mark 'inspection-only' vs 'executable test'.
severity: nit
resolution: AC list split into 'Testable (executable checks)' with IDs AC1-AC11 and 'Inspection-only' with IDs AC-DOC1 through AC-DOC3.
status: addressed
---

From design-review on PLAN-78HJO. Polish; restructure AC list with a label per
item.
