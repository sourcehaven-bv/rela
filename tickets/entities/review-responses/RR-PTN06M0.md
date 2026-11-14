---
id: RR-PTN06M0
type: review-response
title: The outcome switch must fail closed
finding: Gate is nil so flow.Gated cannot occur, but a default or Gated arm that maps to allowed would open a hole once a Gate is added.
severity: minor
resolution: 'Addressed in the plan: Gated and the default arm map to a precondition failure; a unit test calls the mapping with Gated.'
status: addressed
---
