---
id: RR-UKM5ES
type: review-response
title: Discard all kept a stale base version
finding: The review drawer discarded without refetching, so the next edit pinned the old version and every preview returned 409.
severity: significant
resolution: Drawer and stale banner share useDiscardDraft, which discards and refetches.
status: addressed
---
