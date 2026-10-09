---
id: RR-C905DM
type: review-response
title: kvpiles as specified cannot work on state.KV
finding: state.KV has no List, so a per-owner key cannot be scanned for rename/delete; owner strings fail key validation; nopKV silently drops writes when there is no cache dir.
severity: critical
resolution: 'Plan: kvpiles stores ONE document (key piles.json) holding all owners, like kvuserstate; owner strings live inside the value. With no cache dir piles fall back to an in-memory store with a startup warning (as newUserState does), never nopKV.'
status: addressed
---
