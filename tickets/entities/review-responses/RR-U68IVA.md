---
id: RR-U68IVA
type: review-response
title: Conflict-marker prohibition must be a control-flow rule, not just an output property
finding: 'AC6 forbids conflict markers reaching the entity and the test plan asserts on the pure function''s OUTPUT. The dangerous path is the retry loop: ''merge → re-PATCH''. If threeWayMerge returns {merged, conflicts[]} with BOTH a merged body and a non-empty conflicts array — the natural diff3 shape where hunk 1 auto-merged and hunk 2 conflicted — nothing in the plan''s control flow says the re-PATCH must be SKIPPED. Step 3 says only ''surface UI when a conflict is found'', which describes the UI, not the write. Failure: node-diff3''s mergeDiff3 returns {conflict:true, result:[...]} with marker strings inline by default; an implementer wires merged straight into patch.content and separately raises the UI → markers land in the entity body, then in entity_versions where they are effectively permanent (append-only history; the audited history-purge path REFUSES while a live row still holds the content). FIX: state the invariant as control flow — ''a merge result with ANY conflict entry MUST NOT be written; re-PATCH only for a fully clean merge'' — and assert in tests that NO PATCH is issued on a conflicting body merge, in addition to the marker-free output assertion.'
severity: minor
status: open
---
