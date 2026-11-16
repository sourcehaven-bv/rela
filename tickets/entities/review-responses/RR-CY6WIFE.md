---
id: RR-CY6WIFE
type: review-response
title: PageQuery.Related is trusted, not checked
finding: visibility.PageQuery.Related is ANDed onto the read scope without the reader checking that each entry passed the caller's traversal gate. A future caller that builds an ungated entry would let the row set depend on edges the caller cannot see.
severity: significant
resolution: 'PageQuery no longer carries a store edge test. Related became Traversals []PageTraversal (hop and sign only), and the page reader gates each one itself with its binder''s traversal gate before any read: a denied positive term is an empty page, a denied negated term drops out, a refusal is ErrPropsNotPushable and the caller''s Go path. UnrestrictedReader uses the ungated lowering, a binder without a gate refuses. QueryConditions.Lower no longer gates. Pinned by TestPageQuery_CarriesNoEdgeTest (fails if any PageQuery field can hold a DirectedRelation, RelationPredicate or GraphQuery), the denied and refused cases in TestScriptReader_ListEntityPage, and TestGatedReads_ConditionTraversalsGatedByReader.'
status: addressed
---
