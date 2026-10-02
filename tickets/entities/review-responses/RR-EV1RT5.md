---
id: RR-EV1RT5
type: review-response
title: Inline editor minimum widths collapse in every row
finding: 'The caps min(240px, 100%) and min(180px, 100%) sit inside a max-content inline edit, so the percentage is cyclic and resolves to zero: editors lost their floor even in full-width rows.'
severity: significant
resolution: The value column is now a named container (rl-detail-field-value) and both floors use min(N, 100cqi). Pinned by the e2e test that opens the full-width note editor (>= 240px) and the span-4 assignee editor (inside its cell).
status: addressed
---
