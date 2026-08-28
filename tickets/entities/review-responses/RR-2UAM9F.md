---
id: RR-2UAM9F
type: review-response
title: Mobile card render site had no test coverage
finding: Every EntityList test asserted on `tbody`, so the isMobile branch (a full second render site, structurally duplicating the desktop cell) was entirely unexercised. It can drift from desktop silently.
severity: minor
resolution: 'Added a ''mobile card render site'' describe block that stubs window.matchMedia before mount to drive isMobile, covering: widget rendering matching desktop, boolean showing Yes/No, the emptiness predicate still hiding empty columns, and a false boolean staying visible.'
status: addressed
---
