---
id: RR-FD1D
type: review-response
title: 'Round 1 #4: serializeRelatedEntityForWire comment is correct as-is'
finding: |
  Plan's framing called this ticket a "revision of the serializeRelatedEntityForWire contract." That's wrong. serializeRelatedEntityForWire operates on V1Entity. V1ViewEntity is a separate, narrower type. The comment at affordances.go:805-811 is still correct for its actual subject. Don't change that comment.
severity: significant
status: addressed
resolution: |
  PLAN's Existing Solutions section updated: the comment at affordances.go:805-811 is NOT revised — it correctly describes V1Entity behaviour and stays in place. The new behaviour applies to V1ViewEntity (a different type). A new doc comment on V1ViewEntity's `_fields`/`_props` fields explains the pointer-to-map idiom and the relationship to V1Entity semantics.
---
