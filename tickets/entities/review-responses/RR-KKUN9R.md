---
id: RR-KKUN9R
type: review-response
title: 'set.face rule makes set: unusable for a mixed attach_to'
finding: 'For a family-anchored type whose attach_to mixes faced and unfaced types, set.face is required (one attach_to type has faces) and also refused (not every attach_to type declares it). So set: can never load. Plan change: allow per-anchor-type resolution (set.face applies when the anchor''s type declares it, zero face otherwise), or state that the combination is refused, with its own message.'
severity: nit
resolution: set.face resolves per anchor type. It is required when any attach_to type is faced, and every faced type must declare it. Unfaced anchor types use the zero face, so a mixed attach_to loads.
status: addressed
---
