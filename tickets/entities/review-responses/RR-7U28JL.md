---
id: RR-7U28JL
type: review-response
title: Tailed edges on undeclared source become delete-only
finding: UpdateRelation now refuses a named tail on an undeclared source type; renumber can abort mid-plan.
severity: minor
reason: Refusing on update is intended and is now documented on requireRelationFaceFor and in the data-migration guide. A whole-plan precheck for renumber is separate work outside PR 2.
status: deferred
---
