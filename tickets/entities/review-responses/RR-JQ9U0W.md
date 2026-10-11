---
id: RR-JQ9U0W
type: review-response
title: Faced anchor of an incoming list needs a face in its address
finding: The target side has no face, but the move PATCH reads the anchor through the path. A bare id of a faced type reads the default world's first face and may 404 for a reader in another world. Send the anchor as id@face whenever the anchor has a face.
severity: minor
resolution: newRelationOrdering sets the anchor to id@face on the incoming side whenever the anchor's face is not implicit.
status: addressed
---
