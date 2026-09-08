---
id: RR-NDER2B
type: review-response
title: Enum-typed string properties are silently unpushable
finding: declaredOnEveryType requires PropertyType string exactly, while the predicate compiler maps custom enum-like types to StringType. `entity.status == 'open'` on an enum status compiles and evaluates but is never pushed and derives no index, with no diagnostic — status-shaped enums are the most common next-action conjunct.
severity: nit
reason: The gate is shared with the pre-existing filter-path pushdown (queryplan.PushdownPrefilters), which has the same exclusion; widening it changes what dashboards and scope navigation push too and needs the enum-vs-store string-form question answered once for both. Documented on ConditionPrefilters as deliberately out of scope; to be taken as its own change.
status: deferred
---
