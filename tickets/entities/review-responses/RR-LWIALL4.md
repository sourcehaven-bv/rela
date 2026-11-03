---
id: RR-LWIALL4
type: review-response
title: A deleted successor id can be reused and inherit old mentions
finding: '[security] After X->Y and a hard delete of Y, nothing reserves Y. A new Y inherits X''s mentions and backlinks, and can rename onto X through the reclaim branch.'
severity: significant
resolution: 'A successor id stays reserved: CreateEntity, rename and HighestID on sqlite and postgres check retired_ids.successor (case-folded, indexed). DeleteKeepsTheReservation asserts create, rename and HighestID.'
status: addressed
---
