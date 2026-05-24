---
id: RR-PFRZ
type: review-response
title: 'flush: ''post'' papers over the contentRef race for mermaid only'
finding: 'Watch with flush: ''post'' fixes THIS watcher, but contentRef is populated by Vue at render-flush time and referenced in script context where the contract isn''t explicit. Next consumer of contentRef who forgets flush: ''post'' re-introduces the same race.'
severity: minor
resolution: 'Watch comment in EntityDetail.vue explicitly documents the `flush: ''post''` requirement and the race it prevents. Future consumers of `contentRef` who add code without reading the comment will trip the same way — but the comment puts the invariant on the file every reader will see. Per the project''s CLAUDE.md ("Default to writing no comments. Only add one when the WHY is non-obvious") this is exactly the case where a comment is justified, and a `withContentEl` wrapper would be premature abstraction for one current consumer.'
status: addressed
---
