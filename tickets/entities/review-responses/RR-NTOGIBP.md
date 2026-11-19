---
id: RR-NTOGIBP
type: review-response
title: A read that failed on a ctx deadline was kept for the Request
finding: An error caused by the caller ctx ending was memoised so a later caller with a live ctx also saw the failure.
severity: minor
resolution: The memo keeps no error when ctx.Err() is set. TestHoldsPermissionOnAnyEntity_CanceledReadIsNotKept pins it.
status: addressed
---
