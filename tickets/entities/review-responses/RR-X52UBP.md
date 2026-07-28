---
id: RR-X52UBP
type: review-response
title: Hidden-field churn causes unresolvable permanent 412s — merge has nothing to merge
finding: 'computeEntityETag hashes ALL properties of the RAW entity (api_v1.go:1805-1837); both GET (visibleReader.getVisible returns the unredacted store entity, redaction happens later in serializer.forWire) and PATCH''s If-Match check (h.reader.getEntity, the ungated seam) agree — so no redaction asymmetry, but the ETag covers properties the client can NEVER observe. Failure: a scheduled Lua task or another user updates a hidden field (e.g. salary) periodically. Alice types in a visible field. Every PATCH 412s. Each retry refetches, merges (finding ZERO visible differences — the merge is a no-op), re-PATCHes, 412s again. After the 3-attempt bound the loop surfaces a conflict UI for a conflict with no visible cause and no user-actionable resolution; the entity becomes permanently unwritable through autosave. The plan''s risk section anticipates spurious 412s and mitigates with ''merge+retry absorbs them transparently'' — true for the visible-disjoint case, impossible for the hidden-field case. FIX: the terminal state must distinguish ''merge found no conflict but server keeps rejecting'' from ''genuine conflict''; the former should fall back to PATCHing WITHOUT If-Match (today''s behavior, which the server handles correctly via maps.Copy) rather than surfacing an unresolvable conflict.'
severity: critical
status: open
---
