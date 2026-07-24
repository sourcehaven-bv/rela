---
id: RR-MK1XHS
type: review-response
title: 'Combined review: architect + minor cleanups (Annotation.Kind, lifecycle vocab, dead code, pad clamp)'
finding: 'go-architect (verdict: ship it, no critical/significant): (a) Annotation.Box bool is a two-value mode flag in a cross-package DTO that would force a breaking change once a third annotation kind appears; (b) lifecycle{} silently renders a near-duplicate of values{} on a flat enum, blurring the two resolvers'' jobs. cranky minors: (c) dead fieldOf in docscapture (only its own test kept it alive); (d) negative pad inverts the crop rect (no lower bound).'
severity: minor
resolution: '(a) Annotation.Box bool → Kind string (''arrow'' default | ''box''); box=true still accepted for back-compat. (b) lifecycle{} on a flat enum now fails loud pointing at values{}, keeping the vocabulary crisp. (c) fieldOf + its test deleted. (d) padAndClamp clamps negative pad to 0. Architect''s deferred items (the dr.pending ApiError refactor; graph type-vs-id dispatch doc note) noted as future polish, not done now. cranky #3 (cancelled-fetch leaves loadState=pending → backstopped by the timeout) and #6 (acl.yaml load-error swallow, mostly moot since Discover fails first) left as documented follow-ups.'
status: addressed
---
