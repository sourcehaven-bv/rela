---
id: RR-DLML7F
type: review-response
title: 'Coverage gaps: safeDDLName-reject, dry-run drop, non-duplicate create-fail branches'
finding: 'Untested branches in an otherwise well-tested feature: (a) Reconcile''s !safeDDLName branch (the reconciler''s defense-in-depth, claimed tested); (b) the dry-run DROP branch (DB tests cover dry-run create + live drop, not dry-run drop); (c) unenforcedFromCreateErr when create fails for a NON-duplicate reason (count==0) — which echoes raw createErr.Error() into operator output; confirm it can''t carry entity content. FIX: add tests for these branches.'
severity: minor
status: open
---
