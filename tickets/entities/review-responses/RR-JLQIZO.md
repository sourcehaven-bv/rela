---
id: RR-JLQIZO
type: review-response
title: Widget resolved per cell instead of per column
finding: The WeakMap cache only avoided re-renders; the first render still resolved once per cell, against the resolve-per-column rule.
severity: significant
resolution: ViewTableCell caches the resolved widget per PropertyDef in a module-level WeakMap, so each property resolves once and a schema reload resolves afresh.
status: addressed
---
