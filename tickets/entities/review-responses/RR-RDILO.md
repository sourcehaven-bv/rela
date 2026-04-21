---
id: RR-RDILO
type: review-response
title: classifyRenderings degree snapshot lies when co-targets also collapse
finding: '`classifyRenderings` builds the degree map assuming every pair is plain, then classifies. When two relations share targets and both want to collapse, the snapshot reports each target as degree 2 (connected) even though in the final diagram both edges vanish and the target is actually isolated. Case A: `srcA -> {t1..t4}` and `srcB -> {t1..t4}` both classify as legend → graph body empty. Case B: `srcA -> {t1..t3}` and `srcB -> {t1..t5}`: srcB forces legend (>=5), but with the snapshot t1..t3 still look connected, so srcA also goes legend instead of the correct hub-bundle. Fix: two-pass classification — handle >=5 pairs first (unconditional legend), decrement their targets'' degree, then classify 3-4 pairs against the settled degree.'
severity: critical
resolution: 'Rewrote classifyRenderings with a three-pass approach: (1) unconditional buckets (plain for <3, legend for >=5) assigned first; (2) inDegree computed ONLY over non-legend pairs, reflecting what edges will actually be drawn; (3) 3-4 pairs classified against that honest degree, and when a deferred pair ends up as legend its targets'' inDegree is decremented so later pairs see reality. Added TestSchemaGraphvizFivePairStarvesThreePair which creates a 5-target ''many'' relation (forced legend) plus a 3-target ''three'' relation sharing targets. With the old snapshot, both went legend and the body was empty. With the fix, ''three'' correctly hub-bundles because its co-targets are revealed as isolated after ''many'' collapses.'
status: addressed
---
