---
id: RR-QNLAGBX
type: review-response
title: Relation filters lowered before the scope
finding: pushdownPlan's doc says the scope is lowered first; the relation block runs before it. Move it or adjust the doc.
severity: nit
resolution: pushdownPlan lowers the scope, then the relation filters, then the condition.
status: addressed
---
