---
id: RR-65RROE
type: review-response
title: Read-side behaviour for invalid ownership data is undefined
finding: 'The plan does not say what owner resolution, redirect and delete do when the data breaks the rules: a child with two owning parents, a two-level chain, or a relation newly marked owning over existing data. Schema load does not scan data, so this state is reachable (see the raw-write finding).'
severity: significant
resolution: 'Plan: owner returned only for exactly one incoming owning edge from a parent that is not itself owned and not the child; otherwise no owner and no redirect. Cascade stops at direct children. New analyze check reports irregular data (AC11).'
status: addressed
---
