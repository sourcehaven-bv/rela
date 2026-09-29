---
id: RR-3WBQ0B
type: review-response
title: Bare-id test passes by accident; the world-resolved path is untested
finding: The bare-id case calls the handler directly (default world) and only asserts < 500. The case that motivates rendering refOf(ent) (a bare id a named world resolves to a face) is never exercised.
severity: significant
resolution: 'TestAnchoredDocument_BareIDRouted routes through NewRouter with default_world set: bare id of a faced type is the uniform 404 (the _documents route is not world-capable), the addressed face renders, a denied face is the uniform 404. The direct-call bare-id subtest now asserts exactly 404.'
status: addressed
---
