---
id: RR-1BW3I7
type: review-response
title: Comment write-prep keyed by a hand-built Ref
finding: 'comments_handler builds entity.Ref{ID: target.ID, Face: target.Face} for writePrepRow instead of using a Ref method.'
severity: nit
reason: target is a comments.Target and not an entity. It has no Ref method. Adding one to the comments package for this call alone is not worth the coupling.
status: wont-fix
---
