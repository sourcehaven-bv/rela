---
id: RR-UT8ZDEK
type: review-response
title: Fold count computed in RlChannel
finding: count = messages.length - moves in RlChannel spreads one fact over two components and makes the RlChannelFold count doc wrong.
severity: minor
resolution: 'Plan: foldRow returns count and moves; doc comments updated.'
status: addressed
---
