---
id: RR-A6EFGQL
type: review-response
title: Unclear error when no default dashboard exists
finding: 'DashboardByName returns nil for a missing dashboard: block, and the error read no dashboard named empty string.'
severity: nit
resolution: 'The error says data-entry.yaml has no dashboard: block. TestCardFigure_NoDefaultDashboard.'
status: addressed
---
