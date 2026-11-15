---
id: RR-GQ84MYF
type: review-response
title: The attribute invariant behind propsFilterable is implicit
finding: The handler pushes a filter when every Props property is visible on every row. That is only sound if every entity field the filter reads appears in Props, which nothing checked.
severity: significant
resolution: Lower checks prog.Attributes("entity") against the props and declines otherwise. The invariant is in its doc and the ConditionLowerer doc.
status: addressed
---
