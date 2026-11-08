---
id: RR-KT1PVVG
type: review-response
title: 'Security: header-mode callers without identity share one gate key'
finding: In header mode callers without the identity header are all user unknown and share one bucket; a proxy that passes a client-chosen header lets each name get a fresh bucket.
severity: minor
resolution: 'TKT-H91N91L: a peer key stamps peer:<name> before the gate runs, and the gate is keyed on that principal, so each peer has its own bucket and slots whatever the header mode does. TestPeerKey_ReconcileGateKeyedOnPeer pins it. Callers that do not use a peer key keep the documented header-mode behaviour (acl-security.md Reconcile).'
reason: Peer identity and credentials are step 7 of FEAT-CMVZQFI, which keys the gate on the peer credential. Documented in acl-security.md. The data routes behave the same today.
status: addressed
---
