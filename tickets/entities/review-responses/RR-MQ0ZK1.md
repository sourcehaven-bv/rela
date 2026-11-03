---
id: RR-MQ0ZK1
type: review-response
title: RelationVerdicts still skips block-less roles
finding: relationVerdicts skips roles without declaredRelations, the same bug class for relation create/remove/meta writes. affordances.go doc claims access stays monotonic in the role set without scoping it.
severity: significant
reason: Relation create/remove/meta-write verdicts still count only roles that declare a relation grant. That is the same bug class but a separate fold with its own verb mapping and tests; BUG-826XOX tracks it. This change scopes the affordances package doc claim to visible/fields/options and relation meta visibility and names BUG-826XOX for the rest.
status: deferred
---
