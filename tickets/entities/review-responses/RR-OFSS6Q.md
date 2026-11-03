---
id: RR-OFSS6Q
type: review-response
title: Budget AC10 omits storedRefs TypeRecords; storetest.Counting does not count channel-store calls
finding: renderer() calls local.storedRefs, which can read TypeRecords on a cache miss. storetest.Counting wraps store.Store, not channels.Store. Add a counting channels.Store fake to pin Head, Messages and TypeRecords.
severity: minor
resolution: A counting channels.Store fake pins Head, Messages and TypeRecords.
status: addressed
---
