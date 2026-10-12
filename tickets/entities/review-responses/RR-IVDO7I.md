---
id: RR-IVDO7I
type: review-response
title: Live subscriptions accept list free-text q
finding: 'Post-commit security review: liveListKeys accepts q, but the list free-text path queries the raw searcher (Limit 1000) and intersects with ACL rows afterwards, so a hidden write can evict a visible row and a live frame turns it into a timed signal about hidden writes matching the term.'
severity: significant
resolution: q removed from liveListKeys so a ?q= subscription is refused, with a test and a docs sentence; done in the phase 1 PR branch.
status: addressed
---
