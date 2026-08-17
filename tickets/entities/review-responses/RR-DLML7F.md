---
id: RR-DLML7F
type: review-response
title: 'Coverage gaps: safeDDLName-reject, dry-run drop, non-duplicate create-fail branches'
finding: 'Untested branches in an otherwise well-tested feature: (a) Reconcile''s !safeDDLName branch (the reconciler''s defense-in-depth, claimed tested); (b) the dry-run DROP branch (DB tests cover dry-run create + live drop, not dry-run drop); (c) unenforcedFromCreateErr when create fails for a NON-duplicate reason (count==0) — which echoes raw createErr.Error() into operator output; confirm it can''t carry entity content. FIX: add tests for these branches.'
severity: minor
resolution: 'Added tests: TestDerivedUnique_UnsafeNameRefused (safeDDLName reject branch → unenforced), TestDerivedUnique_DryRunDrop (dry-run drop branch reports without mutating), TestSafeDDLName (safe/unsafe corpus, pins the pgstore charset half against metamodel''s). The non-duplicate create-fail path keeps createErr.Error() in Reason — that is a driver/DDL error string (e.g. lock timeout), never entity content (the colliding VALUE lives in pgErr.Detail, which is not used here; only the count/sample-values path touches data, and that is opt-in).'
status: addressed
---
