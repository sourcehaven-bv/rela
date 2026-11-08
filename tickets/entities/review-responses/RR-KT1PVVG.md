---
id: RR-KT1PVVG
type: review-response
title: 'Security: header-mode callers without identity share one gate key'
finding: In header mode callers without the identity header are all user unknown and share one bucket; a proxy that passes a client-chosen header lets each name get a fresh bucket.
severity: minor
reason: Peer identity and credentials are step 7 of FEAT-CMVZQFI, which keys the gate on the peer credential. Documented in acl-security.md. The data routes behave the same today.
status: deferred
---
