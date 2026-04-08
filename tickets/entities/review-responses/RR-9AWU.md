---
id: RR-9AWU
type: review-response
title: FilterBar buildState treats op='=' inconsistently
finding: 'FilterBar.buildState uses `if (op) fv.op = op` which will set fv.op = ''='' literally. Downstream treats ''='' as default and writes the concise form, so it works, but the on-the-wire FilterState briefly contains an explicit ''='' that buildQueryWithFilters then strips. Inconsistent with the convention used elsewhere (omit op when default). Fix: skip op when it''s ''='' or undefined.'
severity: minor
resolution: FilterBar.buildState now omits op when it's absent OR '=' — matches the convention used by buildQueryWithFilters. Keeps the state shape canonical throughout the flow.
status: addressed
---
