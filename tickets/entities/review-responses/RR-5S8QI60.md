---
id: RR-5S8QI60
type: review-response
title: PageQuery must not accept ungated relation leaves
finding: A store.Cond relation leaf carries any RelationPredicate including EndpointMatch. If visibility trusts it without the traversal gate it is an existence and property oracle on hidden far-side entities.
severity: significant
resolution: PageQuery.Where is a visibility.PageCond whose relation leaves are relresolve hops. visibility.GateCond gates each hop for the reader; a denied hop becomes false and a refusal wins over a denial.
status: addressed
---
