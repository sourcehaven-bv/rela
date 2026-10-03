---
id: RR-Z23T2T
type: review-response
title: Empty face grant can reach the store as nil FaceIn and fail open
finding: FaceIn nil means every face; the design does not say how a principal with no readable face is represented before the world query
severity: significant
resolution: 'Design section 8.6 (PR 2): permitted faces are all / non-empty list / none; none is a miss before any query and FaceIn is nil only for all. Unit test asserts no ListEntities call.'
status: addressed
---

**Where:** design section 2, gate order step 3, world mode: `ListEntities{IDs:
[id], World: scope, FaceIn: permittedFaces(type)}`.

`store.EntityQuery.FaceIn` treats nil as "every face" (store.go:299). If the
principal may read no face of the type, `permittedFaces` can produce nil or an
empty slice. Passing nil fails open. The design does not say how "no readable
face" is represented or where it stops.
