---
id: RR-7BI6XC
type: review-response
title: Enum-value validation lost on pushdown
severity: minor
status: addressed
finding: 'filter.matchEnum errors when a filter value is not a declared enum value, which surfaces an operator typo. Pushed down, prop:status=snet silently matched nothing and the source went permanently quiet with no diagnostic -- and the documented example in docs/data-entry.md is exactly this shape.'
resolution: 'Subsumed by the pre-filter fix (RR-HO25O0): enums are excluded from pushdown eligibility, so matchEnum still runs and still errors on an undeclared value.'
---
