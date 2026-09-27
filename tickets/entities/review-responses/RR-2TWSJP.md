---
id: RR-2TWSJP
type: review-response
title: Principal-dependent failures that could be load errors
finding: ErrTraversalUnsupported from a conditionally-visible far property turns a configured list into a 422 for some principals only.
severity: minor
resolution: 'View and next-action conditions refuse at load what Policy.ConditionallyVisible decides without a principal; as scopes do. ACL when: traversals filtering such a property log a load warning.'
status: addressed
---
