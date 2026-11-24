---
id: RR-C8B3C1T
type: review-response
title: 'Design: picker paging and unreadable scope'
finding: 'The relations endpoint may page; an unreadable scope entity looks like no candidates.'
severity: minor
resolution: 'The picker reads the scope entity''s owning edges once through the gated relations endpoint, and shows a distinct hint when the load fails. The server check is the authority; the picker filter is UX only, documented in the component.'
status: addressed
---
