---
id: RR-DYNFSM
type: review-response
title: RenderStandalone returned a struct whose fields it can never populate
finding: RenderStandalone returned *DocumentResult, but two of that struct's three fields (ContentHash, Entities) are permanently zero for a standalone render — they describe an entry entity's dependency footprint and there is no entry entity. The handler worked around it by hardcoding Cached:false and EntityIDs:[] rather than reading the result. A future caller would see DocumentResult.Entities in the signature, wire the SSE live-reload subscription to it, and ship a document that silently never refreshes, with no compile error and no test failure.
severity: significant
resolution: 'Changed the return to (string, error), matching RenderListMarkdown which returns a plain string for the same reason. The godoc now states why the narrower type was chosen, so the struct is not ''restored for consistency'' later. Also documented at the handler that Cached is always false and ?refresh=true is accepted-and-ignored (there is no cache to bust), which was minor finding #8 in the same review.'
status: addressed
---
