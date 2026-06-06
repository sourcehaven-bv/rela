---
id: RR-WX77
type: review-response
title: Search backend integration with ACL is not pinned for future _search ticket
finding: 'The plan''s ACL-then-search ordering is correct for handleV1ListEntities (in-memory intersection after ACL filter). When the deferred /_search wiring lands in a follow-up ticket, the integration order must be the same: ACL filter is authoritative, search-backend ID lists are intersected post-ACL. The plan doesn''t document this as a constraint for the follow-up. Fix: add a note in GUIDE-acl-security ''When /_search is wired up, search-backend ID results MUST be intersected with the ACL-visible set; the search backend never sees the ACL filter. Search backends that return hit-count metadata are themselves leak surfaces.'' Saves the next person from re-litigating it.'
severity: minor
reason: Moved to TKT-VMD8 (touches scopedSortedEntities ordering). AC9 there pins ACL → search intersection ordering; documented as the contract for the future /_search ticket.
status: deferred
---
