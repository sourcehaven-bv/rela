---
id: RR-HNIT8N
type: review-response
title: Dead assertion in TestDerivedSchemaPublishesSpecsBeforeReconciling
finding: 'The condition `f.specsPublished == nil && len(f.specsPublished) != 0` is unsatisfiable — a nil slice has len 0, so the conjunction can never be true. Mutation-tested by the reviewer: deleting s.SetUniqueSpecProvider(specs) from the resolver entirely left the test PASSING. A test named ''PublishesSpecsBeforeReconciling'' asserted neither publishing nor ordering. Two deeper issues: even the intended check (specsPublished != nil) would be wrong, because the test passes a nil metamodel so SetUniqueSpecProvider(nil) is indistinguishable from never being called; and nilness cannot express ordering at all.'
severity: significant
resolution: 'Replaced with explicit call-order recording (calls []string + slices.Equal against [SetUniqueSpecProvider, Reconcile]). Verified non-vacuous by swapping the order in the resolver: fails with ''call order = [Reconcile SetUniqueSpecProvider], want [SetUniqueSpecProvider Reconcile]''. I had independently suspected this line and flagged it to the reviewer; the mutation test confirmed it.'
status: addressed
---
