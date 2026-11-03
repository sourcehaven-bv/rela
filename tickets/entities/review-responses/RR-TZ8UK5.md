---
id: RR-TZ8UK5
type: review-response
title: Docs overstate SSE frame-timing independence
finding: docs/acl-security.md and the runSSELoop doc claimed frame timing depends only on readable types. Any relation write (even between hidden entities) arms the flush window, so a readable frame can arrive up to 200 ms early. Raised by both the code and security review.
severity: significant
resolution: Both texts now state that a hidden entity write produces no frame and does not shift timing, while a relation write of any kind can make a readable frame arrive up to one flush window early. The signal names no entity or type and predates this change.
status: addressed
---
