---
id: RR-GPP93S
type: review-response
title: Broken subscriptions.yaml froze schedules.yaml edits
finding: reloadConfig rejected the whole result on ErrSubscriptions although schedules loaded.
severity: significant
resolution: On ErrSubscriptions the schedules apply and the running subscriptions are kept; TestWatchConfig_BrokenSubscriptionsKeepSchedulesLive.
status: addressed
---
