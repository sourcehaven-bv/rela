---
id: RR-4DYGAZ2
type: review-response
title: Focus check is per tab, not per browser
finding: Holder A notifies while the user reads rela in tab B.
severity: minor
resolution: Tabs announce focus on a base-scoped BroadcastChannel; the holder skips notifying while any tab is focused.
status: addressed
---
