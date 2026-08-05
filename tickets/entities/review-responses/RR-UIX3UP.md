---
id: RR-UIX3UP
type: review-response
title: redactRelationMetaStrip took `incoming` as both a struct field and a redundant parameter
finding: 'After the L2 refactor, redactRelationMetaStrip(ctx, s, pathEntity, relType, incoming) branched on the parameter `incoming` while relationMetaStrip already carried s.incoming. No live bug (both call sites passed a value equal to the strip''s field), but two sources of truth for the fail-closed selector: a future caller passing incoming=false with a strip whose incoming=true would route an incoming edge down the outgoing branch and silently under-redact against the wrong-type pathEntity instead of failing closed on the peer.'
severity: minor
resolution: Dropped the parameter; redactRelationMetaStrip now reads s.incoming as the single source of truth, with a comment warning against reintroducing the parameter. Both call sites updated. buildRelationTypeRows and the inline builder already set .incoming authoritatively. Build + lint + tests green.
status: addressed
---
