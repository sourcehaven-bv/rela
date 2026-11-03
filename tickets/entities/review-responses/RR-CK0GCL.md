---
id: RR-CK0GCL
type: review-response
title: Handler loads state twice and resolves a bare id for the title
finding: viewMention calls app.State() again after the handler captured a snapshot, and resolves the new entity by bare id, so a faced target can get another face's title.
severity: minor
resolution: The handler passes its snapshot Meta and the created entity Ref to the mention lookup, which resolves that face through ResolveHeaders. The unused mentionOf helper was removed.
status: addressed
---
