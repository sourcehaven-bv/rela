---
id: RR-T3OHHD
type: review-response
title: The quiet-signal test covers only collapsed signal types
finding: 'ChannelsPanel.test.ts:1981 ''quiet signal not shown'' checks a collapsed heartbeat only. AC 8 names both collapsed and hidden. Fix: add a signal of a hidden type (for example give the fixture a hidden ephemeral type) and assert that it is also left out.'
severity: nit
resolution: Fixed. The fixture has a hidden ephemeral type ping; the test asserts its signal is left out until Show all is pressed, like the collapsed heartbeat.
status: addressed
---
