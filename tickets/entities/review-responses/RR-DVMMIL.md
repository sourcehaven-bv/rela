---
id: RR-DVMMIL
type: review-response
title: Count fields had no visible label
finding: Only the number and a shared icon showed, so two relation counts were indistinguishable and show_label was ignored.
severity: significant
resolution: 'Fixed. The label shows beside the number by default (''2 subtasks''); show_label: false shows only the number, with the label kept for screen readers and as a title tooltip. Pinned in KanbanView.cardCounts.test.ts; docs updated.'
status: addressed
---
