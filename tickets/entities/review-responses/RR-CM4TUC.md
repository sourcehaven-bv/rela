---
id: RR-CM4TUC
type: review-response
title: Metamodel godoc links missing Catalog
finding: types.go Metamodel doc linked [Catalog], which no longer exists; the comment-lint doclink gate fails and the sentence misdescribes where lookups live.
severity: significant
resolution: Doc now names GetEntityDef/GetRelationDef as the lookups, Validator for validation and the New*Projection constructors.
status: addressed
---
