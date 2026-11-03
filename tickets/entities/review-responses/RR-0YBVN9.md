---
id: RR-0YBVN9
type: review-response
title: Refuse roll-ups on every condition and filter surface
finding: A roll-up is never stored, so ACL when, validations, transitions, detail actions, next-action conditions, view where/sort/group_by and list sort silently compare against nothing.
severity: significant
resolution: Roll-up properties are left out of predicatefns.EntityRecordType, the single type adapter every predicate surface compiles against, so any expression naming one fails to load. The data-entry list API refuses filter/sort on a roll-up (400), and the data-entry config validator refuses roll-ups in where/sort/group_by.
status: addressed
---
