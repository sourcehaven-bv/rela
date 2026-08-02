---
id: RR-01E7DG
type: review-response
title: Animated GIF flatten decision must be made now (in scope, data loss)
finding: image.Decode yields only a GIF's first frame; re-encoding an animated GIF to PNG/JPEG silently destroys animation — potential data loss if it's the only copy. Plan shipped GIF in scope but deferred the decision 'to review'.
severity: significant
resolution: 'Decision made: detect multi-frame GIF via gif.DecodeAll (len(frames) > 1) and REJECT it from a native image: step with a clear error (ErrRejected → 422, ''animated GIF not supported by native image processing; use a cmd: transform''), rather than silently flatten. Single-frame GIFs re-encode normally. Added to plan scope + AC + edge-case test.'
status: addressed
---
