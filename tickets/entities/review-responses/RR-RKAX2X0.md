---
id: RR-RKAX2X0
type: review-response
title: Tests did not check what other peers receive
finding: Tests inspected only the room's stored state, not the broadcast; owner update/remove and reconnect were untested.
severity: minor
resolution: Added TestStampAwareness_PeersReceiveTheServerName, TestAwarenessOwnership_OwnerUpdatesAndRemoves and the binding test. Hocuspocus framing and token-auth stamping stay untested because rela uses neither.
status: addressed
---
