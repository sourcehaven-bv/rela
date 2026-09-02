---
id: RR-J6PWOV
type: review-response
title: 'Budget/truncation logic was correct by accident: tautological guard and two boundary rules for one concept'
finding: The root loop's `budget <= 0 && i < len(rootIDs)` conjunct was a tautology inside the range loop, so the stated 'exact exhaustion is not truncation' property held via the loop exit, by accident; the child path expressed the rule separately. (The reviewer's off-by-one claim for budget=1-with-child was itself incorrect — the child IS a cut visible node — but the duplication critique stood.)
severity: significant
resolution: 'Extracted ganttBudget{remaining, truncated} with a single take() method: truncated becomes true exactly when a visible node is denied emission, stated once and shared by the root loop and the child walk. All truncation tests (including TestGantt_TruncatedIsPostFilter''s exact-budget oracle case) still pass.'
status: addressed
---
