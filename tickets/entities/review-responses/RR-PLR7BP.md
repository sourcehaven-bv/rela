---
id: RR-PLR7BP
type: review-response
title: View entry relations token can differ from the PATCH check
finding: The view entry is serialized with forWire (no neighbour filter) while PATCH uses faceEdges with the visible map; docs claim the tokens are the same as the GET's.
severity: minor
resolution: 'The view entry no longer carries a relations token (FieldVersions.Relations is omitempty and cleared on the entry). Docs and the frontend type say so. Test: TestV1Views_EntryVersionsMatchGet.'
status: addressed
---
