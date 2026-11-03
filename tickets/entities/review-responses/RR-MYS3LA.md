---
id: RR-MYS3LA
type: review-response
title: Promote copies hidden ref ids into an entity the actor can read
finding: 'internal/realtime/promote.go:123 promoteProps copies raw stored msg.Props including ref ids into the new entity, while the renderer (render.go:170-175) withholds ids of refs the reader cannot read. A poster who can read hidden entity H posts a ref to H; an actor who cannot read H but passes the action gate promotes it and then reads H''s id from the created entity, or from the promote_invalid/promote_conflict title (realtime_promote.go:149,155 use err.Error()). Fix: before the pending CAS in RunAction resolve every ref-typed mapped property through s.refs.Resolve as the caller; refuse a missing id with 422 promote_invalid naming only the message property. Pin with a test where the poster can read the target and the actor cannot.'
severity: significant
resolution: RunAction resolves every mapped ref prop as the actor in one batch before the pending CAS; a missing or unreadable ref answers the same 422 promote_invalid naming only the message property; an edit landing between check and CAS reverts the action; promote titles no longer quote error text or ids; tested in realtime (fake refs) and appbuild with poster alice and actor carol who cannot read the secret (453779953 6302a4b22)
status: addressed
---
