---
id: RR-RCY86B
type: review-response
title: Reload path for busy fields is unpinned
finding: 'AC 8 says the busy check holds "on reload". internal/dataentry/watcher.go:349 does call checkChannelsPresentation, but no test reloads a data-entry.yaml with `busy: nope` and asserts that the reload is refused and the last good config is kept. TestSetRealtime_ChecksBusyField covers boot only. Fix: add a watcher reload case next to TestReloadRealtime_RereadsDataEntryConfig in internal/dataentry/realtime_wiring_test.go.'
severity: minor
resolution: 'Added setupReloadTestAppWith and TestReloadConfig_ChecksBusyField: a reload with busy: nope is refused with a config-error event and keeps the old config, and busy: run is accepted.'
status: addressed
---
