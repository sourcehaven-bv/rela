---
id: RR-ULH7FY
type: review-response
title: Before and after refs on the incoming side are addresses
finding: A sibling of an incoming move is named by its source, which may carry a face (S@face). The handler gate must take the id from the address, and the planner must match the tail when a face is named and the first place otherwise.
severity: minor
resolution: orderRef parses before/after as entity addresses on the incoming side; a bare id matches the source's first place and id@face that tail. The wire carries addresses for faced rows and the SPA maps ids through them (orderMoveArgs). Pinned by TestPlaceOrder_IncomingRefAddresses and TestRelationOrder_IncomingFacedSources.
status: addressed
---
