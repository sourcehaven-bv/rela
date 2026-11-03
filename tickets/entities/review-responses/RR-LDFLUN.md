---
id: RR-LDFLUN
type: review-response
title: Comments name the old receivers
finding: validate.go (x2), config.go, manager_order.go, project/context.go and docs/architecture/consumer-side-interfaces.md still cite Metamodel.ResolveWidgetFromType, schema_output.go, Meta.ValidateRelation or metamodel.ValidateRelation.
severity: minor
resolution: All six now name metamodel.ResolveWidgetFromType or Validator.ValidateRelation; the stale single-source-of-truth sentence is gone; the docs example says relation validation.
status: addressed
---
