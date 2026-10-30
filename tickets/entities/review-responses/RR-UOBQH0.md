---
id: RR-UOBQH0
type: review-response
title: No reload test for a list busy column
finding: 'The scope says the busy type is checked ''at boot, on reload, in rela validate and in Configure''. AC 4 tests SetRealtime, CheckChannels, the CLI and Configure, but not the data-entry.yaml reload (readConfig at watcher.go:342-355) or the realtime.yaml reload that drops a type a list still names (reloadRealtime at realtime_wiring.go:108-134). The kanban field has both (TestReloadConfig_ChecksBusyField, TestReloadRealtime_ChannelsDrift in realtime_wiring_test.go:175,291). The code is shared, but the list''s at-prefix and the busyFields walk over cfg.Lists are new. Fix: add a list case to TestReloadConfig_ChecksBusyField: refused ''nope'' keeps the last good config, ''run'' is applied.'
severity: minor
resolution: 'ADDRESSED AC 4 adds a list case to TestReloadConfig_ChecksBusyField: busy: nope is refused and keeps the last good config, busy: run is applied'
status: addressed
---
