---
id: RR-5X4XBB
type: review-response
title: Face semantics undecided; stepping breaks on faced entities
finding: Position matches by id only and PositionRef has no face; a pile holding two faces of one id gets stuck. Pinned faces would diverge from the entity page.
severity: significant
resolution: 'Revised after user input: items are explicit entity.Ref{ID, Face} so a user can pile a specific face. Adds resolve bare ids with WriteTarget (ambiguous gets 409 naming faces). _position matches by Ref for pile scope; PositionRef gains address; SPA navigates to /entity/<type>/<ID@face>. EntityFaceDeleted drops that ref.'
status: addressed
---
