---
id: RR-0SD5RU
type: review-response
title: Unbounded EntityIDs batches were an unwritten store contract
finding: The gantt drill-down passes a whole subtree as EntityIDs; backends in tree accept it but the contract did not say so.
severity: significant
resolution: RelationQuery.EntityIDs godoc states that a backend must accept a batch of any size, pointing at the ListEntityIDsLargeBatch conformance test (40,000 ids).
status: addressed
---
