---
id: RR-B6F0E1
type: review-response
title: The test name 'is not stored' is not backed by a remount
finding: 'ChannelsPanel.test.ts:1946 ''the toggle shows every message and is not stored'' never unmounts and remounts to check that the toggle starts off again. Fix: remount the panel (or reopen the channel) and assert aria-pressed=''false'', or drop ''and is not stored'' from the name.'
severity: nit
resolution: Fixed. The test unmounts and remounts the panel and asserts aria-pressed is false and the hidden message is hidden again.
status: addressed
---
