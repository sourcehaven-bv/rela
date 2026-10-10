---
id: RR-FQBZ3M
type: review-response
title: FaceMoved and drop path cost at scale
finding: FaceMoved did a Get per comment (O(n^2) I/O on filecomments); drop_entities calls DeleteAllFaces twice per id, each a full thread-directory walk on filecomments.
severity: minor
reason: FaceMoved now lists the destination once and filters with an id set. The filecomments DeleteAllFaces walk is a property of that backend's layout, shared with the entitymanager delete path; the file tier is the single-user tier, and indexing threads by id is a separate change.
status: deferred
---
