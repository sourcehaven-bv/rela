---
id: RR-Y7MINP
type: review-response
title: Truncation flag and node counts must be computed post-ACL-filter or they leak hidden entities
finding: 'The plan says recursion is bounded by a node cap and that ''a config that would fan out past the cap is truncated with an explicit flag on the response''. If that flag (or any node/child count) is computed against the RAW tree before ACL filtering, it is an existence oracle: a principal who can see 3 children learns a 4th exists because the response says truncated, or because a count says 10. This is exactly the leak class pinned by TestACLList_PaginationLeakSurfaces (internal/dataentry/acl_list_test.go:88-91, TKT-VMD8 AC3, RR-KNGC + RR-VDTW), where ''with 5 visible + 5 hidden tickets ... every pagination surface reflects the post-filter count of 5 — and the hidden total of 10 appears nowhere in the response''. The gantt endpoint must apply the identical rule: cap, count, and truncation flag are all evaluated on the ALREADY-FILTERED tree. Needs an explicit test mirroring the pagination one.'
severity: critical
resolution: 'Plan updated: the gate→fold→cap ordering in the Approach section makes the cap step three, explicitly after row-gating, and states ''A subtree dropped for visibility contributes nothing to any ancestor''s span, flags, or counts, and never sets truncated.'' Security Considerations cites the TestACLList_PaginationLeakSurfaces precedent (acl_list_test.go:88-91, TKT-VMD8 AC3, RR-KNGC + RR-VDTW) as the rule to mirror. Test plan AC7 row now covers the post-filter cap/count test alongside the two-principal fold test.'
status: addressed
---

## Finding

The plan bounds recursion with "a configured max depth, and a node cap; a config
that would fan out past the cap is truncated with an explicit flag on the
response, never silently."

It does not say **when** the cap is applied relative to ACL filtering. That
ordering is the whole security property.

If the cap/count/flag is computed on the raw tree:

- A principal seeing 3 visible children of a node whose 4th child is hidden
gets `truncated: true` (or `childCount: 4`) and learns a hidden sibling exists.
- The *shape* of the hidden graph leaks even though no id or property does.

## Direct precedent in this codebase

`internal/dataentry/acl_list_test.go:88-91` pins the identical rule for
pagination (TKT-VMD8 AC3, findings RR-KNGC + RR-VDTW):

> with 5 visible + 5 hidden tickets and `per_page=3&page=2`, every pagination
> surface reflects the post-filter count of 5 — and the hidden total of 10
> appears nowhere in the response

`visibility.Reader` states the same contract at the entity level
(`internal/visibility/visibility.go:85-95`): denied, missing and type-mismatched
are indistinguishable, and even a store fault is swallowed into the same clean
miss because "the oracle-free contract requires it".

## Resolution

State explicitly in the plan: **filter first, then fold, then cap.** The node
cap, any child/descendant count, and the truncation flag are all computed over
the post-filter tree. A subtree dropped for visibility is not "truncation" and
must not set the flag.

Add a test mirroring `TestACLList_PaginationLeakSurfaces`: a tree with visible
and hidden siblings at the cap boundary, asserting the flag and every count
reflect only what the principal may see.
